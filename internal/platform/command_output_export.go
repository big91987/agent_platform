package platform

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"time"

	opensandbox "github.com/alibaba/OpenSandbox/sdks/sandbox/go"
	"golang.org/x/sys/unix"
)

type commandOutputExport struct {
	Seq       int    `json:"seq"`
	Path      string `json:"path"`
	Bytes     int64  `json:"bytes"`
	SHA256    string `json:"sha256"`
	Truncated bool   `json:"truncated"`
}

// Only retained, already-redacted bytes are exported. The Run and step are the
// existing identity; callers cannot choose a destination or another Run.
type logExportCommand func(context.Context, opensandbox.RunCommandRequest, opensandbox.EventHandler) (SandboxCommandResult, error)

func (s *Store) exportCommandOutput(ctx context.Context, r WorkflowRun, seq int, dispatch logExportCommand) (commandOutputExport, error) {
	out := commandOutputExport{Seq: seq}
	if !validSandboxRun(r.ID) {
		return out, ErrForbidden
	}
	page, err := s.connectorOutput(r, seq, 0)
	if err != nil {
		return out, err
	}
	info := r.Steps[seq-1].actionReceipt().Log
	root, err := os.OpenRoot(path.Join(s.Dir, "workflow-processes"))
	if err != nil {
		return out, err
	}
	defer root.Close()
	f, err := root.OpenFile(fmt.Sprintf("%s-%d/output.log", r.ID, seq), os.O_RDONLY|unix.O_NOFOLLOW|unix.O_NONBLOCK, 0)
	if err != nil {
		return out, err
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil || !st.Mode().IsRegular() || st.Size() != info.Bytes {
		return out, errors.New("retained command log unavailable or changed")
	}
	raw, err := io.ReadAll(io.LimitReader(f, connectorArchiveLimit+1))
	if err != nil || int64(len(raw)) != info.Bytes {
		return out, errors.New("retained command log unavailable or changed")
	}
	after, err := f.Stat()
	if err != nil || after.Size() != st.Size() || !after.ModTime().Equal(st.ModTime()) {
		return out, errors.New("retained command log changed during export")
	}
	out.Bytes = int64(len(raw))
	out.SHA256 = fmt.Sprintf("%x", sha256.Sum256(raw))
	out.Truncated = page.Truncated
	out.Path = path.Join("workflow-output", r.ID, fmt.Sprintf("command-%d.log", seq))
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	if r.Definition.Environment != "opensandbox" {
		if err = ctx.Err(); err == nil {
			err = writeCommandLogChunk(r.WorkspacePath, r.ID, seq, 0, raw, len(raw), out.SHA256)
		}
		return out, err
	}
	if r.Environment == nil || r.Environment.Status != "ready" {
		return out, errors.New("Run environment is not ready for log export")
	}
	if dispatch == nil {
		return out, errors.New("log export requires tracked execution")
	}
	for offset := 0; ; {
		if err = ctx.Err(); err != nil {
			return out, err
		}
		end := min(offset+64*1024, len(raw))
		meta, _ := json.Marshal(map[string]any{"run": r.ID, "seq": seq, "offset": offset, "total": len(raw), "sha256": out.SHA256})
		var stdout bytes.Buffer
		result, e := dispatch(ctx, opensandbox.RunCommandRequest{Argv: []string{"python3", "-I", "-S", "-c", sandboxCommandLogWriter, string(meta), base64.StdEncoding.EncodeToString(raw[offset:end])}, Cwd: "/workspace", Timeout: 30000}, func(event opensandbox.StreamEvent) error {
			var v struct{ Type, Text string }
			if e := json.Unmarshal([]byte(event.Data), &v); e != nil {
				return e
			}
			if v.Type == "stdout" {
				if stdout.Len()+len(v.Text) > 1024 {
					return errors.New("log export response too large")
				}
				stdout.WriteString(v.Text)
			}
			return nil
		})
		if e != nil {
			return out, e
		}
		var ack struct {
			Complete bool `json:"complete"`
		}
		if !result.Stopped || result.ExitCode == nil || *result.ExitCode != 0 || json.Unmarshal(stdout.Bytes(), &ack) != nil {
			return out, errors.New("log export not confirmed; inspect the same export before retrying")
		}
		if ack.Complete {
			return out, nil
		}
		if end == len(raw) {
			return out, errors.New("log export incomplete")
		}
		offset = end
	}
}

func commandLogDirectory(workspace, run string) (int, error) {
	fd, err := unix.Open(workspace, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
	if err != nil {
		return -1, err
	}
	for _, part := range []string{"workflow-output", run} {
		if err = unix.Mkdirat(fd, part, 0700); err != nil && !errors.Is(err, unix.EEXIST) {
			unix.Close(fd)
			return -1, err
		}
		next, e := unix.Openat(fd, part, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
		unix.Close(fd)
		if e != nil {
			return -1, e
		}
		fd = next
	}
	return fd, nil
}

func readLogAt(fd int, name string, total int, digest string) error {
	n, err := unix.Openat(fd, name, unix.O_RDONLY|unix.O_NOFOLLOW|unix.O_NONBLOCK|unix.O_CLOEXEC, 0)
	if err != nil {
		return err
	}
	f := os.NewFile(uintptr(n), name)
	defer f.Close()
	st, err := f.Stat()
	if err != nil || !st.Mode().IsRegular() || st.Size() != int64(total) {
		return errors.New("existing log export conflicts")
	}
	h := sha256.New()
	if _, err = io.Copy(h, io.LimitReader(f, int64(total)+1)); err != nil {
		return err
	}
	if fmt.Sprintf("%x", h.Sum(nil)) != digest {
		return errors.New("existing log export conflicts")
	}
	return nil
}

// A verified prefix can resume an interrupted export. File descriptors keep
// symlinks out of both parent traversal and writes; a final file is never replaced.
func writeCommandLogChunk(workspace, run string, seq, offset int, data []byte, total int, digest string) error {
	if !validSandboxRun(run) || seq < 1 || offset < 0 || total > connectorArchiveLimit || offset+len(data) > total {
		return ErrForbidden
	}
	dir, err := commandLogDirectory(workspace, run)
	if err != nil {
		return err
	}
	defer unix.Close(dir)
	name := fmt.Sprintf("command-%d.log", seq)
	if err = readLogAt(dir, name, total, digest); err == nil {
		return nil
	} else if !errors.Is(err, unix.ENOENT) {
		return err
	}
	partial := name + ".partial"
	fd, err := unix.Openat(dir, partial, unix.O_RDWR|unix.O_CREAT|unix.O_NOFOLLOW|unix.O_NONBLOCK|unix.O_CLOEXEC, 0600)
	if err != nil {
		return err
	}
	f := os.NewFile(uintptr(fd), partial)
	defer f.Close()
	if err = unix.Flock(fd, unix.LOCK_EX|unix.LOCK_NB); err != nil {
		return err
	}
	var st unix.Stat_t
	if err = unix.Fstat(fd, &st); err != nil {
		return err
	}
	if st.Mode&unix.S_IFMT != unix.S_IFREG || st.Nlink != 1 || st.Size < int64(offset) || st.Size > int64(total) {
		return errors.New("partial log export conflicts")
	}
	overlap := min(len(data), int(st.Size)-offset)
	prior := make([]byte, overlap)
	if _, err = f.ReadAt(prior, int64(offset)); err != nil && err != io.EOF {
		return err
	}
	if !bytes.Equal(prior, data[:overlap]) {
		return errors.New("partial log export conflicts")
	}
	if _, err = f.WriteAt(data[overlap:], int64(offset+overlap)); err != nil {
		return err
	}
	if err = f.Sync(); err != nil {
		return err
	}
	final, err := f.Stat()
	if err != nil {
		return err
	}
	if final.Size() != int64(total) {
		return nil
	}
	if err = readLogAt(dir, partial, total, digest); err != nil {
		return err
	}
	if err = unix.Linkat(dir, partial, dir, name, 0); err != nil {
		return err
	}
	if err = unix.Unlinkat(dir, partial, 0); err != nil {
		return err
	}
	return unix.Fsync(dir)
}

const sandboxCommandLogWriter = `import os,sys,json,base64,stat,hashlib,fcntl
m=json.loads(sys.argv[1]);data=base64.b64decode(sys.argv[2],validate=True)
run=m['run'];seq=m['seq'];offset=m['offset'];total=m['total'];digest=m['sha256']
assert len(run)==32 and all(c in '0123456789abcdef' for c in run)
assert seq>0 and 0<=offset<=total<=8*1024*1024 and offset+len(data)<=total
flags=os.O_RDONLY|os.O_NOFOLLOW|os.O_DIRECTORY
fd=os.open('/workspace',flags)
try:
 for part in ('workflow-output',run):
  try:os.mkdir(part,0o700,dir_fd=fd)
  except FileExistsError:pass
  child=os.open(part,flags,dir_fd=fd);os.close(fd);fd=child
 name='command-%d.log'%seq;partial=name+'.partial'
 def verify(name):
  x=os.open(name,os.O_RDONLY|os.O_NOFOLLOW|os.O_NONBLOCK,dir_fd=fd)
  with os.fdopen(x,'rb') as f:
   s=os.fstat(f.fileno());assert stat.S_ISREG(s.st_mode) and s.st_size==total,'log export conflict'
   assert hashlib.sha256(f.read(total+1)).hexdigest()==digest,'log export conflict'
 try:verify(name);complete=True
 except FileNotFoundError:
  x=os.open(partial,os.O_RDWR|os.O_CREAT|os.O_NOFOLLOW|os.O_NONBLOCK,0o600,dir_fd=fd)
  with os.fdopen(x,'r+b') as f:
   fcntl.flock(f,fcntl.LOCK_EX|fcntl.LOCK_NB);s=os.fstat(f.fileno())
   assert stat.S_ISREG(s.st_mode) and s.st_nlink==1 and offset<=s.st_size<=total,'partial log conflict'
   overlap=min(len(data),s.st_size-offset);f.seek(offset);assert f.read(overlap)==data[:overlap],'partial log conflict'
   f.seek(offset+overlap);f.write(data[overlap:]);f.flush();os.fsync(f.fileno())
   complete=os.fstat(f.fileno()).st_size==total
   if complete:
    verify(partial);os.link(partial,name,src_dir_fd=fd,dst_dir_fd=fd,follow_symlinks=False);os.unlink(partial,dir_fd=fd);os.fsync(fd)
 print(json.dumps({'complete':complete}))
finally:os.close(fd)
`
