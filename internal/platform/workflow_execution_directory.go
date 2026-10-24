package platform

import (
	"errors"
	"path"
	"strings"

	opensandbox "github.com/alibaba/OpenSandbox/sdks/sandbox/go"
)

func validateExecutionDirectory(dir string) error {
	if len(dir) > 1024 || strings.ContainsRune(dir, 0) || path.IsAbs(dir) || dir == ".." || strings.HasPrefix(dir, "../") || dir != "" && path.Clean(dir) != dir {
		return errors.New("execution cwd must be a workspace-relative directory")
	}
	return nil
}

func validateExecutionVariables(values, refs map[string]string) error {
	if err := validateExecutionEnvRefs(refs); err != nil {
		return err
	}
	if len(values)+len(refs) > 32 {
		return errors.New("execution environment supports up to 32 variables and references")
	}
	total := 0
	for name, value := range values {
		if !executionEnvName(name) || strings.ContainsRune(value, 0) || len(value) > 64000 {
			return errors.New("execution variables require non-reserved names and string values up to 64000 bytes without NUL")
		}
		if _, ok := refs[name]; ok {
			return errors.New("an execution variable cannot have both a literal value and a reference")
		}
		total += len(name) + len(value)
	}
	if total > 64000 {
		return errors.New("execution variable values exceed 64000 bytes")
	}
	return nil
}

// Open each component relative to a held directory, refusing symlinks. The SDK
// starts in the Run root; only the actual child process changes its directory.
const executionCwdScript = `import os,sys
fd=None
try:
 flags=os.O_RDONLY|os.O_DIRECTORY|os.O_NOFOLLOW
 fd=os.open(sys.argv[1],flags)
 for component in sys.argv[2].split('/'):
  if component=='.':continue
  child=os.open(component,flags,dir_fd=fd)
  os.close(fd)
  fd=child
 os.fchdir(fd)
except OSError:
 sys.stderr.write('Execution working directory is missing, inaccessible, or contains a symbolic link.\n')
 sys.exit(1)
finally:
 if fd is not None:os.close(fd)
os.execv('/bin/sh',['/bin/sh','-c',sys.argv[3]])
`

func configureExecutionDirectory(request *opensandbox.RunCommandRequest, relative string) {
	if relative == "" || relative == "." {
		return
	}
	request.Argv = []string{"python3", "-c", executionCwdScript, request.Cwd, relative, request.Command}
	request.Command = ""
}
