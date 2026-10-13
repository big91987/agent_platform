package platform

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path"
	"syscall"
	"time"

	opensandbox "github.com/alibaba/OpenSandbox/sdks/sandbox/go"
)

// This private journal extends the existing conversation's native preparation.
// It is not a public task identity or a second workspace. Interrupted uploads
// may only be reconciled by the final remote marker; they are never replayed.
type sandboxRolePreparation struct {
	Run      string   `json:"run"`
	Sandbox  string   `json:"sandbox"`
	Snapshot string   `json:"snapshot"`
	Files    string   `json:"files"`
	Config   []byte   `json:"config"`
	Skills   []string `json:"skills"`
	Prepared bool     `json:"prepared"`
}

func (x *Codex) prepareSandboxRole(ctx context.Context, service *OpenSandbox, run WorkflowRun, c Conversation) (string, []byte, []string, map[string]string, error) {
	fail := func(err error) (string, []byte, []string, map[string]string, error) { return "", nil, nil, nil, err }
	if service == nil || run.Environment == nil || run.Environment.Status != "ready" || run.WorkspacePath != "/workspace" || service.connectionHash != run.Environment.ConnectionHash || !validSandboxRun(run.ID) || !validSandboxRun(c.ID) {
		return fail(errors.New("role preparation requires the owned ready Run environment"))
	}
	a := c.Snapshot
	if _, err := nativeConfig(a); err != nil {
		return fail(err)
	}
	// These three fields are the existing formal, between-turn permission API.
	// Thread and turn requests apply them; they do not replace frozen role assets.
	a.NetworkAccess, a.AllowElevation, a.ApprovalsReviewer = false, false, ""
	snapshot, err := json.Marshal(a)
	if err != nil {
		return fail(err)
	}
	identity := hashText(string(snapshot))
	env, err := sandboxRoleEnv(c.Snapshot)
	if err != nil {
		return fail(err)
	}
	root, err := os.OpenRoot(x.Root)
	if err != nil {
		return fail(err)
	}
	defer root.Close()
	directory := "conversations/" + c.ID + "/native"
	journal := directory + "/sandbox-preparation.json"
	home := "/var/lib/agent-platform/roles/" + c.ID
	prepared, err := readSandboxRolePreparation(root, journal)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return fail(err)
	}
	if prepared != nil {
		if prepared.Run != run.ID || prepared.Sandbox != run.Environment.ID || prepared.Snapshot != identity || prepared.Files == "" || len(prepared.Config) == 0 || prepared.Skills == nil {
			return fail(errors.New("saved sandbox role preparation differs from this conversation; no files were replaced"))
		}
		if err = service.confirmRoleMarker(ctx, run, c.ID, home, prepared.Files); err != nil {
			return fail(err)
		}
		if !prepared.Prepared {
			prepared.Prepared = true
			if err = writeSandboxRolePreparation(root, journal, *prepared, false); err != nil {
				return fail(err)
			}
		}
		return home, prepared.Config, prepared.Skills, env, nil
	}
	if c.ThreadID != "" {
		return fail(errors.New("original sandbox role preparation is missing; no fresh native home was created"))
	}
	files, selected, env, err := x.sandboxRoleInputs(c)
	if err != nil {
		return fail(err)
	}
	// Retain a digest of the initial assets, including auth, only in private
	// preparation metadata. Never expose their bytes/digest as a public setting.
	hash := sha256.New()
	for _, file := range files {
		metadata, _ := json.Marshal([]any{file.Path, file.Mode, len(file.Content)})
		hash.Write(metadata)
		hash.Write(file.Content)
	}
	prepared = &sandboxRolePreparation{Run: run.ID, Sandbox: run.Environment.ID, Snapshot: identity, Files: hex.EncodeToString(hash.Sum(nil)), Config: files[0].Content, Skills: selected}
	execd, err := service.execd(ctx, run.Environment.ID, run.ID)
	if err != nil {
		return fail(err)
	}
	if _, err = execd.GetFileInfo(ctx, home); err == nil {
		return fail(errors.New("remote native home already exists without its preparation journal; no files were replaced"))
	}
	var api *opensandbox.APIError
	if !errors.As(err, &api) || api.StatusCode != 404 {
		return fail(sandboxError("native home absence check", err))
	}
	if err = root.MkdirAll(directory, 0700); err != nil {
		return fail(err)
	}
	if err = writeSandboxRolePreparation(root, journal, *prepared, true); err != nil {
		return fail(err)
	}
	bounded, cancel := context.WithTimeout(ctx, 120*time.Second)
	defer cancel()
	directories := map[string]bool{}
	for _, file := range files {
		parent := path.Dir(file.Path)
		if !directories[parent] {
			if err = service.nativeHome(bounded, run.Environment.ID, run.ID, parent); err != nil {
				return fail(err)
			}
			directories[parent] = true
		}
		if err = service.upload(bounded, run.Environment.ID, run.ID, file.Path, file.Content, file.Mode); err != nil {
			return fail(err)
		}
	}
	marker := sandboxRoleMarker(run, c.ID, prepared.Files)
	if err = service.upload(bounded, run.Environment.ID, run.ID, home+"/prepared.json", marker, 0600); err != nil {
		return fail(err)
	}
	if err = service.confirmRoleMarker(bounded, run, c.ID, home, prepared.Files); err != nil {
		return fail(err)
	}
	prepared.Prepared = true
	if err = writeSandboxRolePreparation(root, journal, *prepared, false); err != nil {
		return fail(err)
	}
	return home, prepared.Config, prepared.Skills, env, nil
}

func sandboxRoleMarker(run WorkflowRun, conversation, digest string) []byte {
	raw, _ := json.Marshal(map[string]string{"run": run.ID, "sandbox": run.Environment.ID, "conversation": conversation, "files": digest})
	return raw
}

func (s *OpenSandbox) confirmRoleMarker(ctx context.Context, run WorkflowRun, conversation, home, digest string) error {
	execd, err := s.execd(ctx, run.Environment.ID, run.ID)
	if err != nil {
		return err
	}
	reader, err := execd.DownloadFile(ctx, home+"/prepared.json", "")
	if err != nil {
		return errors.New("sandbox role preparation is incomplete or its final marker is unavailable; no upload was replayed")
	}
	defer reader.Close()
	raw, err := io.ReadAll(io.LimitReader(reader, 4097))
	if err != nil || len(raw) > 4096 || !bytes.Equal(raw, sandboxRoleMarker(run, conversation, digest)) {
		return errors.New("sandbox role preparation marker does not match the original assets; no files were replaced")
	}
	return nil
}

func readSandboxRolePreparation(root *os.Root, name string) (*sandboxRolePreparation, error) {
	before, err := root.Lstat(name)
	if err != nil {
		return nil, err
	}
	owner, ok := before.Sys().(*syscall.Stat_t)
	if !before.Mode().IsRegular() || before.Mode().Perm() != 0600 || !ok || int(owner.Uid) != os.Getuid() {
		return nil, errors.New("sandbox preparation journal is not a private owned regular file")
	}
	file, err := root.OpenFile(name, os.O_RDONLY|syscall.O_NOFOLLOW, 0)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	actual, err := file.Stat()
	if err != nil || !os.SameFile(before, actual) || actual.Mode().Perm() != 0600 {
		return nil, errors.New("sandbox preparation journal changed while opening")
	}
	raw, err := io.ReadAll(io.LimitReader(file, 1024*1024+1))
	if err != nil || len(raw) > 1024*1024 {
		return nil, errors.New("sandbox preparation journal exceeds its bound")
	}
	var prepared sandboxRolePreparation
	if json.Unmarshal(raw, &prepared) != nil {
		return nil, errors.New("sandbox preparation journal is invalid")
	}
	return &prepared, nil
}

func writeSandboxRolePreparation(root *os.Root, name string, prepared sandboxRolePreparation, first bool) error {
	target := name
	if !first {
		target = name + ".ready-" + newID()
		defer root.Remove(target)
	}
	file, err := root.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_EXCL|syscall.O_NOFOLLOW, 0600)
	if err != nil {
		return err
	}
	err = json.NewEncoder(file).Encode(prepared)
	if err == nil {
		err = file.Sync()
	}
	closed := file.Close()
	if err != nil {
		return err
	}
	if closed != nil {
		return closed
	}
	if !first {
		if err = root.Rename(target, name); err != nil {
			return err
		}
	}
	directory, err := root.Open(path.Dir(name))
	if err != nil {
		return err
	}
	defer directory.Close()
	return directory.Sync()
}
