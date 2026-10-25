package platform

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"strconv"
	"strings"
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
	err = transferWorkspaceBytes(ctx, r, out.Path, raw, out.SHA256, dispatch)
	return out, err
}

func transferWorkspaceBytes(ctx context.Context, r WorkflowRun, relativePath string, raw []byte, digest string, dispatch logExportCommand) error {
	if r.Definition.Environment != "opensandbox" {
		if err := ctx.Err(); err != nil {
			return err
		}
		return writeWorkspaceExportChunk(r.WorkspacePath, relativePath, 0, raw, len(raw), digest)
	}
	if r.Environment == nil || r.Environment.Status != "ready" {
		return errors.New("Run environment is not ready for log export")
	}
	if dispatch == nil {
		return errors.New("log export requires tracked execution")
	}
	for offset := 0; ; {
		if err := ctx.Err(); err != nil {
			return err
		}
		end := min(offset+64*1024, len(raw))
		meta, _ := json.Marshal(map[string]any{"run": r.ID, "path": relativePath, "offset": offset, "total": len(raw), "sha256": digest})
		var stdout bytes.Buffer
		result, e := dispatch(ctx, opensandbox.RunCommandRequest{Argv: []string{"python3", "-I", "-S", "-c", sandboxWorkspaceExportWriter, string(meta), base64.StdEncoding.EncodeToString(raw[offset:end])}, Cwd: "/workspace", Timeout: 30000}, func(event opensandbox.StreamEvent) error {
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
			return e
		}
		var ack struct {
			Complete bool `json:"complete"`
		}
		if !result.Stopped || result.ExitCode == nil || *result.ExitCode != 0 || json.Unmarshal(stdout.Bytes(), &ack) != nil {
			return errors.New("log export not confirmed; inspect the same export before retrying")
		}
		if ack.Complete {
			return nil
		}
		if end == len(raw) {
			return errors.New("log export incomplete")
		}
		offset = end
	}
}

func validWorkspaceExportPath(relativePath string) bool {
	if !validArchivePath(relativePath) {
		return false
	}
	parts := strings.Split(relativePath, "/")
	positive := func(v string) bool {
		n, err := strconv.Atoi(v)
		return err == nil && n > 0 && strconv.Itoa(n) == v
	}
	if len(parts) == 3 && parts[0] == "workflow-output" && validSandboxRun(parts[1]) {
		name := parts[2]
		return strings.HasPrefix(name, "command-") && strings.HasSuffix(name, ".log") && positive(strings.TrimSuffix(strings.TrimPrefix(name, "command-"), ".log"))
	}
	return len(parts) >= 5 && parts[0] == "harness-output" && parts[1] == "imports" && validSandboxRun(parts[2]) && positive(parts[3]) && validArchivePath(strings.Join(parts[4:], "/"))
}

func workspaceExportDirectory(workspace, relativePath string) (int, error) {
	fd, err := unix.Open(workspace, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
	if err != nil {
		return -1, err
	}
	for _, part := range strings.Split(path.Dir(relativePath), "/") {
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

func verifyWorkspaceExportAt(fd int, name string, total int, digest, partial string) error {
	n, err := unix.Openat(fd, name, unix.O_RDONLY|unix.O_NOFOLLOW|unix.O_NONBLOCK|unix.O_CLOEXEC, 0)
	if err != nil {
		return err
	}
	f := os.NewFile(uintptr(n), name)
	defer f.Close()
	st, err := f.Stat()
	var links unix.Stat_t
	if err != nil || !st.Mode().IsRegular() || st.Size() != int64(total) || unix.Fstat(n, &links) != nil || (links.Nlink != 1 && (partial == "" || links.Nlink != 2)) {
		return errors.New("existing workspace export conflicts")
	}
	if links.Nlink == 2 {
		if err = unix.Flock(n, unix.LOCK_EX|unix.LOCK_NB); err != nil {
			return err
		}
	}
	h := sha256.New()
	if _, err = io.Copy(h, io.LimitReader(f, int64(total)+1)); err != nil {
		return err
	}
	if fmt.Sprintf("%x", h.Sum(nil)) != digest {
		return errors.New("existing workspace export conflicts")
	}
	if links.Nlink == 2 {
		// Link publication may have completed before the process stopped while
		// removing its private staging name. Only that exact pair can recover.
		var staged, current unix.Stat_t
		if unix.Fstat(n, &current) != nil || unix.Fstatat(fd, partial, &staged, unix.AT_SYMLINK_NOFOLLOW) != nil || staged.Mode&unix.S_IFMT != unix.S_IFREG || staged.Dev != current.Dev || staged.Ino != current.Ino || staged.Nlink != 2 || current.Nlink != 2 || staged.Size != int64(total) || current.Size != int64(total) {
			return errors.New("existing workspace export conflicts")
		}
		if err = unix.Unlinkat(fd, partial, 0); err != nil {
			return err
		}
		if err = unix.Fsync(fd); err != nil {
			return err
		}
		return verifyWorkspaceExportAt(fd, name, total, digest, "")
	}
	return nil
}

// Log destinations are platform-owned; callers cannot choose a path.
func writeCommandLogChunk(workspace, run string, seq, offset int, data []byte, total int, digest string) error {
	if !validSandboxRun(run) || seq < 1 {
		return ErrForbidden
	}
	return writeWorkspaceExportChunk(workspace, path.Join("workflow-output", run, fmt.Sprintf("command-%d.log", seq)), offset, data, total, digest)
}

// A verified prefix can resume an interrupted export. File descriptors keep
// symlinks out of parent traversal and writes; a final file is never replaced.
func writeWorkspaceExportChunk(workspace, relativePath string, offset int, data []byte, total int, digest string) error {
	hash, err := hex.DecodeString(digest)
	if !validWorkspaceExportPath(relativePath) || offset < 0 || total < 0 || total > archiveFileLimit || offset > total || len(data) > total-offset || err != nil || len(hash) != sha256.Size || strings.ToLower(digest) != digest {
		return ErrForbidden
	}
	dir, err := workspaceExportDirectory(workspace, relativePath)
	if err != nil {
		return err
	}
	defer unix.Close(dir)
	name := path.Base(relativePath)
	partial := "." + name + ".partial"
	if err = verifyWorkspaceExportAt(dir, name, total, digest, partial); err == nil {
		return nil
	} else if !errors.Is(err, unix.ENOENT) {
		return err
	}
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
		return errors.New("partial workspace export conflicts")
	}
	overlap := min(len(data), int(st.Size)-offset)
	prior := make([]byte, overlap)
	if _, err = f.ReadAt(prior, int64(offset)); err != nil && err != io.EOF {
		return err
	}
	if !bytes.Equal(prior, data[:overlap]) {
		return errors.New("partial workspace export conflicts")
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
	if err = verifyWorkspaceExportAt(dir, partial, total, digest, ""); err != nil {
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

const sandboxWorkspaceExportWriter = `import os,sys,json,base64,stat,hashlib,fcntl,posixpath
m=json.loads(sys.argv[1]);data=base64.b64decode(sys.argv[2],validate=True)
run=m['run'];p=m['path'];offset=m['offset'];total=m['total'];digest=m['sha256']
def valid_run(v):return isinstance(v,str) and len(v)==32 and all(c in '0123456789abcdef' for c in v)
def positive(v):return v.isascii() and v.isdigit() and int(v)>0 and str(int(v))==v and int(v)<=sys.maxsize
def valid_path(v):return isinstance(v,str) and 0<len(v.encode())<=4096 and not v.startswith('/') and posixpath.normpath(v)==v and not any(x.startswith('.') or x=='node_modules' for x in v.split('/')) and not any(c in v for c in ('\\','\x00'))
assert valid_run(run) and valid_path(p)
parts=p.split('/')
log=len(parts)==3 and parts[0]=='workflow-output' and valid_run(parts[1]) and parts[2].startswith('command-') and parts[2].endswith('.log') and positive(parts[2][8:-4])
imp=len(parts)>=5 and parts[:2]==['harness-output','imports'] and valid_run(parts[2]) and positive(parts[3]) and '/'.join(parts[4:])!='AGENTS.md'
assert log or imp
assert type(offset)==int and type(total)==int and 0<=offset<=total<=8*1024*1024 and len(data)<=total-offset
assert isinstance(digest,str) and len(digest)==64 and all(c in '0123456789abcdef' for c in digest)
flags=os.O_RDONLY|os.O_NOFOLLOW|os.O_DIRECTORY
fd=os.open('/workspace',flags)
try:
 for part in parts[:-1]:
  try:os.mkdir(part,0o700,dir_fd=fd)
  except FileExistsError:pass
  child=os.open(part,flags,dir_fd=fd);os.close(fd);fd=child
 name=parts[-1];partial='.'+name+'.partial'
 def verify(name,recover=None):
  x=os.open(name,os.O_RDONLY|os.O_NOFOLLOW|os.O_NONBLOCK,dir_fd=fd)
  with os.fdopen(x,'rb') as f:
   s=os.fstat(f.fileno());assert stat.S_ISREG(s.st_mode) and s.st_size==total and (s.st_nlink==1 or recover is not None and s.st_nlink==2),'workspace export conflict'
   if s.st_nlink==2:fcntl.flock(f,fcntl.LOCK_EX|fcntl.LOCK_NB)
   assert hashlib.sha256(f.read(total+1)).hexdigest()==digest,'workspace export conflict'
   if s.st_nlink==2:
    try:t=os.stat(recover,dir_fd=fd,follow_symlinks=False)
    except FileNotFoundError:raise ValueError('workspace export conflict')
    current=os.fstat(f.fileno())
    assert stat.S_ISREG(t.st_mode) and (t.st_dev,t.st_ino)==(current.st_dev,current.st_ino) and t.st_nlink==current.st_nlink==2 and t.st_size==current.st_size==total,'workspace export conflict'
    os.unlink(recover,dir_fd=fd);os.fsync(fd);verify(name)
 try:verify(name,partial);complete=True
 except FileNotFoundError:
  x=os.open(partial,os.O_RDWR|os.O_CREAT|os.O_NOFOLLOW|os.O_NONBLOCK,0o600,dir_fd=fd)
  with os.fdopen(x,'r+b') as f:
   fcntl.flock(f,fcntl.LOCK_EX|fcntl.LOCK_NB);s=os.fstat(f.fileno())
   assert stat.S_ISREG(s.st_mode) and s.st_nlink==1 and offset<=s.st_size<=total,'partial workspace export conflict'
   overlap=min(len(data),s.st_size-offset);f.seek(offset);assert f.read(overlap)==data[:overlap],'partial workspace export conflict'
   f.seek(offset+overlap);f.write(data[overlap:]);f.flush();os.fsync(f.fileno())
   complete=os.fstat(f.fileno()).st_size==total
   if complete:
    verify(partial);os.link(partial,name,src_dir_fd=fd,dst_dir_fd=fd,follow_symlinks=False);os.unlink(partial,dir_fd=fd);os.fsync(fd)
 print(json.dumps({'complete':complete}))
finally:os.close(fd)
`
