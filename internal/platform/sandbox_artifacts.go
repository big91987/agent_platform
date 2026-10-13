package platform

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"path"
	"strings"

	opensandbox "github.com/alibaba/OpenSandbox/sdks/sandbox/go"
)

// Read using directory descriptors, not a metadata check followed by a download
// that can follow a replaced parent symlink. This fixed helper lives with the
// platform; projects do not maintain their own artifact readers.
const sandboxArtifactReader = `import os,sys,stat,json,base64,datetime
def hidden(p):
 return p=='AGENTS.md' or any(x.startswith('.') or x=='node_modules' for x in p.split('/'))
flags=os.O_RDONLY|os.O_NOFOLLOW|os.O_NONBLOCK
root=os.open('/workspace',flags|os.O_DIRECTORY)
out=[]; scanned=0
def listing(fd,prefix='',depth=0):
 global scanned
 if depth>64: raise ValueError('limit')
 with os.scandir(fd) as entries:
  for e in entries:
   scanned+=1
   if scanned>5000: raise ValueError('limit')
   name=prefix+e.name
   if hidden(name): continue
   if len(name.encode())>4096: raise ValueError('limit')
   st=e.stat(follow_symlinks=False)
   if stat.S_ISLNK(st.st_mode): continue
   if stat.S_ISDIR(st.st_mode):
    child=os.open(e.name,flags|os.O_DIRECTORY,dir_fd=fd)
    try: listing(child,name+'/',depth+1)
    finally: os.close(child)
   elif stat.S_ISREG(st.st_mode):
    out.append({'path':name,'size':st.st_size,'updated_at':datetime.datetime.fromtimestamp(st.st_mtime,datetime.timezone.utc).strftime('%Y-%m-%dT%H:%M:%SZ')})
try:
 p=json.loads(sys.argv[1])
 if not p:
  listing(root); print(json.dumps({'files':sorted(out,key=lambda x:x['path'])}))
 else:
  parts=p.split('/')
  if p.startswith('/') or hidden(p) or any(x in ('','..','.') for x in parts): raise PermissionError()
  fd=os.dup(root)
  try:
   for part in parts[:-1]:
    child=os.open(part,flags|os.O_DIRECTORY,dir_fd=fd);os.close(fd);fd=child
   file=os.open(parts[-1],flags,dir_fd=fd)
   with os.fdopen(file,'rb') as f:
    before=os.fstat(f.fileno())
    if not stat.S_ISREG(before.st_mode): raise PermissionError()
    if before.st_size>8*1024*1024: raise ValueError('limit')
    data=f.read(8*1024*1024+1);after=os.fstat(f.fileno())
    if len(data)>8*1024*1024: raise ValueError('limit')
    if (before.st_size,before.st_mtime_ns)!=(after.st_size,after.st_mtime_ns) or len(data)!=after.st_size: raise ValueError('changed')
    print(json.dumps({'data':base64.b64encode(data).decode()}))
  finally: os.close(fd)
except FileNotFoundError: print(json.dumps({'error':'not_found'}))
except PermissionError: print(json.dumps({'error':'forbidden'}))
except ValueError: print(json.dumps({'error':'limit_or_changed'}))
except OSError: print(json.dumps({'error':'unavailable'}))
finally: os.close(root)
`

func (s *Store) sandboxArtifacts(ctx context.Context, run WorkflowRun, file string) ([]Artifact, []byte, error) {
	if run.Environment == nil || run.Environment.Status != "ready" {
		return nil, nil, errors.New("workspace files are unavailable until the Run environment is ready; released files must be retained as delivery artifacts")
	}
	if file != "" && (strings.ContainsRune(file, 0) || len(file) > 4096 || path.IsAbs(file) || path.Clean(file) != file || hiddenArtifact(file) || file == ".." || strings.HasPrefix(file, "../")) {
		return nil, nil, ErrForbidden
	}
	service, err := s.sandboxFor(run)
	if err != nil {
		return nil, nil, err
	}
	arg, _ := json.Marshal(file)
	var stdout bytes.Buffer
	result, err := service.command(ctx, run.Environment.ID, run.ID, opensandbox.RunCommandRequest{Argv: []string{"python3", "-c", sandboxArtifactReader, string(arg)}, Cwd: "/workspace", Timeout: 30000}, func(event opensandbox.StreamEvent) error {
		var v struct{ Type, Text string }
		if e := json.Unmarshal([]byte(event.Data), &v); e != nil {
			return e
		}
		if v.Type == "stdout" {
			if stdout.Len()+len(v.Text) > 12<<20 {
				return errors.New("workspace file response exceeded its size limit")
			}
			stdout.WriteString(v.Text)
		}
		return nil
	})
	if err != nil {
		return nil, nil, err
	}
	if !result.Stopped || result.ExitCode == nil || *result.ExitCode != 0 {
		return nil, nil, errors.New("workspace file reader did not complete successfully")
	}
	var v struct {
		Files []Artifact `json:"files"`
		Data  *string    `json:"data"`
		Error string     `json:"error"`
	}
	if err = json.Unmarshal(stdout.Bytes(), &v); err != nil {
		return nil, nil, errors.New("workspace file reader returned an invalid response")
	}
	switch v.Error {
	case "":
	case "not_found":
		return nil, nil, ErrNotFound
	case "forbidden":
		return nil, nil, ErrForbidden
	default:
		return nil, nil, errors.New("workspace file is unavailable, changed during reading, or exceeds the size limit")
	}
	if file == "" {
		if v.Files == nil {
			v.Files = []Artifact{}
		}
		for i := range v.Files {
			v.Files[i].Image = imageArtifact(v.Files[i].Path)
		}
		return v.Files, nil, nil
	}
	if v.Data == nil {
		return nil, nil, errors.New("workspace file reader returned no content")
	}
	content, err := base64.StdEncoding.DecodeString(*v.Data)
	if err != nil || len(content) > 8<<20 {
		return nil, nil, errors.New("workspace file reader returned invalid content")
	}
	return nil, content, nil
}

func (s *Store) conversationArtifacts(ctx context.Context, c Conversation) ([]Artifact, error) {
	run, err := s.sandboxRunForConversation(c.ID)
	if err != nil {
		return nil, err
	}
	if run == nil {
		return listArtifacts(c.workspace(s.Dir))
	}
	files, _, err := s.sandboxArtifacts(ctx, *run, "")
	return files, err
}
