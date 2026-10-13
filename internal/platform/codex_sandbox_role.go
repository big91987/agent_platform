package platform

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"strings"
	"syscall"
)

// Private transport inputs, not another task/configuration domain. The existing
// conversation identifies its role home and the Agent snapshot selects assets.
type sandboxRoleFile struct {
	Path    string
	Content []byte
	Mode    fs.FileMode
}

func (x *Codex) sandboxRoleInputs(c Conversation) ([]sandboxRoleFile, []string, map[string]string, error) {
	a := c.Snapshot
	if !validSandboxRun(c.ID) || a.Executor != "codex" || a.Model == "" {
		return nil, nil, nil, errors.New("sandbox role requires its conversation identity, Codex and an explicit model")
	}
	if a.InheritEnv {
		return nil, nil, nil, errors.New("sandbox roles cannot inherit the whole host environment; select required variables explicitly")
	}
	if a.SeedDir != "" {
		return nil, nil, nil, errors.New("prepare the shared Run workspace before role execution; a role workspace seed cannot replace it")
	}
	for _, tool := range a.ResolvedTools {
		if tool.Connection.Command != "" {
			return nil, nil, nil, errors.New("sandbox roles require HTTP MCP connections; host stdio programs cannot be run inside a different environment")
		}
	}
	cfg, err := nativeConfig(bindToolWorkspace(a, "/workspace"))
	if err != nil {
		return nil, nil, nil, err
	}
	home := "/var/lib/agent-platform/roles/" + c.ID
	files := []sandboxRoleFile{{Path: home + "/config.toml", Content: cfg, Mode: 0600}}
	expected := []string{}
	total := len(cfg)
	seen := map[string]bool{}
	for index, selected := range a.Skills {
		source, err := normalizeSkill(selected)
		if err != nil {
			return nil, nil, nil, err
		}
		if seen[source] {
			return nil, nil, nil, errors.New("duplicate selected Skill directory")
		}
		seen[source] = true
		root, err := os.OpenRoot(source)
		if err != nil {
			return nil, nil, nil, errors.New("cannot open selected Skill source")
		}
		destination := fmt.Sprintf("%s/skills/selected-%d", home, index)
		expected = append(expected, destination+"/SKILL.md")
		err = fs.WalkDir(root.FS(), ".", func(name string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return errors.New("cannot inspect selected Skill assets")
			}
			if entry.IsDir() {
				if name != "." && (entry.Name() == ".git" || entry.Name() == ".data" || entry.Name() == "node_modules" || entry.Name() == "__pycache__") {
					return fs.SkipDir
				}
				return nil
			}
			info, err := entry.Info()
			if err != nil {
				return err
			}
			if !info.Mode().IsRegular() {
				return errors.New("selected Skill contains a link or non-regular asset")
			}
			if info.Size() > 32*1024*1024 || len(files) >= 512 {
				return errors.New("selected role assets exceed file limits")
			}
			file, err := root.OpenFile(name, syscall.O_RDONLY|syscall.O_NOFOLLOW, 0)
			if err != nil {
				return errors.New("cannot safely open selected Skill asset")
			}
			actual, err := file.Stat()
			if err != nil || !os.SameFile(info, actual) {
				file.Close()
				return errors.New("selected Skill asset changed while opening")
			}
			raw, err := io.ReadAll(io.LimitReader(file, 32*1024*1024+1))
			file.Close()
			if err != nil {
				return errors.New("cannot read selected Skill asset")
			}
			total += len(raw)
			if len(raw) > 32*1024*1024 || total > 64*1024*1024 {
				return errors.New("selected role assets exceed byte limits")
			}
			mode := fs.FileMode(0600)
			if info.Mode()&0111 != 0 {
				mode = 0700
			}
			files = append(files, sandboxRoleFile{Path: path.Join(destination, name), Content: raw, Mode: mode})
			return nil
		})
		root.Close()
		if err != nil {
			return nil, nil, nil, err
		}
	}
	for _, selected := range expected {
		files[0].Content = append(files[0].Content, []byte(fmt.Sprintf("\n[[skills.config]]\npath = %s\nenabled = true\n", quote(selected)))...)
	}
	auth, err := x.sandboxRoleAuth()
	if err != nil {
		return nil, nil, nil, err
	}
	files = append(files, sandboxRoleFile{Path: home + "/auth.json", Content: auth, Mode: 0600})
	total = 0
	for _, file := range files {
		total += len(file.Content)
	}
	if len(files) > 512 || total > 64*1024*1024 {
		return nil, nil, nil, errors.New("selected role inputs exceed transfer limits")
	}
	env := map[string]string{}
	for name, value := range a.Env {
		if value == nil {
			return nil, nil, nil, errors.New("sandbox role variable removal is not configured; choose explicit values")
		}
		if name == "HOME" || strings.HasPrefix(name, "CODEX_") || strings.HasPrefix(name, "AGENT_PLATFORM_") || !envName.MatchString(name) || strings.ContainsRune(*value, 0) || len(*value) > 64000 {
			return nil, nil, nil, errors.New("sandbox role contains a reserved or invalid variable")
		}
		env[name] = *value
	}
	for _, tool := range a.ResolvedTools {
		name := tool.Connection.BearerTokenEnvVar
		if name == "" {
			continue
		}
		if _, exists := env[name]; exists {
			continue
		}
		value, exists := os.LookupEnv(name)
		if !exists {
			return nil, nil, nil, errors.New("selected HTTP MCP credential reference is unavailable")
		}
		env[name] = value
	}
	return files, expected, env, nil
}

func (x *Codex) sandboxRoleAuth() ([]byte, error) {
	root, err := os.OpenRoot(x.authHome())
	if err != nil {
		return nil, errors.New("native Codex authentication home is unavailable")
	}
	defer root.Close()
	before, err := root.Lstat("auth.json")
	if err != nil || !before.Mode().IsRegular() || before.Mode().Perm()&0077 != 0 {
		return nil, errors.New("native authentication must be a private regular file")
	}
	stat, ok := before.Sys().(*syscall.Stat_t)
	if !ok || int(stat.Uid) != os.Getuid() {
		return nil, errors.New("native authentication must belong to the platform user")
	}
	file, err := root.OpenFile("auth.json", syscall.O_RDONLY|syscall.O_NOFOLLOW, 0)
	if err != nil {
		return nil, errors.New("native authentication could not be safely opened")
	}
	defer file.Close()
	actual, err := file.Stat()
	if err != nil || !os.SameFile(before, actual) || actual.Mode().Perm()&0077 != 0 {
		return nil, errors.New("native authentication changed while opening")
	}
	raw, err := io.ReadAll(io.LimitReader(file, 256*1024+1))
	if err != nil || len(raw) > 256*1024 {
		return nil, errors.New("native authentication could not be read within its bound")
	}
	var value map[string]json.RawMessage
	if json.Unmarshal(raw, &value) != nil || value == nil {
		return nil, errors.New("native authentication is not a JSON object")
	}
	return raw, nil
}
