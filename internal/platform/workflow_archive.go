package platform

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"
)

const archiveFailurePrefix = "artifact archive failed: "
const archiveFileLimit = 8 << 20
const archiveStepLimit = 128 << 20

type ArchivedFile struct {
	Path   string `json:"path"`
	Size   int64  `json:"size"`
	SHA256 string `json:"sha256"`
}
type WorkflowArchive struct {
	Seq      int            `json:"seq"`
	Status   string         `json:"status"`
	Captured string         `json:"captured_at,omitempty"`
	Error    string         `json:"error,omitempty"`
	Files    []ArchivedFile `json:"files"`
}

func validArchivePath(p string) bool {
	return p != "" && len(p) <= 4096 && !path.IsAbs(p) && path.Clean(p) == p && p != ".." && !strings.HasPrefix(p, "../") && !strings.ContainsAny(p, "\\\x00") && !hiddenArtifact(p)
}
func archivePaths(step WorkflowStep) ([]string, error) {
	var paths []string
	if step.Result == nil {
		return paths, nil
	}
	for _, p := range step.Result.Artifacts {
		if strings.HasPrefix(p, "https://") || strings.HasPrefix(p, "http://") {
			continue
		}
		if !validArchivePath(p) {
			return nil, fmt.Errorf("invalid artifact path %q; declare individual workspace-relative files", p)
		}
		if !slices.Contains(paths, p) {
			paths = append(paths, p)
		}
	}
	if len(paths) > 256 {
		return nil, errors.New("at most 256 files may be retained per node execution")
	}
	slices.Sort(paths)
	return paths, nil
}
func (s *Store) archiveDir(id string, seq int) string {
	return filepath.Join(s.Dir, "runs", id, "artifacts", strconv.Itoa(seq))
}
func (s *Store) archiveState(r WorkflowRun, step WorkflowStep) (WorkflowArchive, error) {
	a := WorkflowArchive{Seq: step.Seq, Status: "pending", Files: []ArchivedFile{}}
	b, err := os.ReadFile(filepath.Join(s.archiveDir(r.ID, step.Seq), "manifest.json"))
	if err == nil {
		if err = json.Unmarshal(b, &a); err != nil {
			return a, err
		}
		expected, e := archivePaths(step)
		if e != nil {
			return a, e
		}
		actual := []string{}
		for _, f := range a.Files {
			actual = append(actual, f.Path)
		}
		if a.Status != "retained" || a.Seq != step.Seq || !slices.Equal(expected, actual) {
			return a, errors.New("artifact manifest does not match this handoff")
		}
		return a, nil
	}
	if !os.IsNotExist(err) {
		return a, err
	}
	if b, e := os.ReadFile(s.archiveDir(r.ID, step.Seq) + ".error"); e == nil {
		a.Status = "failed"
		a.Error = string(b)
	} else if !os.IsNotExist(e) {
		return a, e
	}
	return a, nil
}
func (s *Store) workflowArchives(r WorkflowRun) ([]WorkflowArchive, error) {
	out := []WorkflowArchive{}
	for _, step := range r.Steps {
		if step.Result == nil || len(step.Result.Artifacts) == 0 {
			continue
		}
		paths, err := archivePaths(step)
		if err == nil && len(paths) == 0 {
			continue
		}
		a, e := s.archiveState(r, step)
		if e != nil {
			return nil, e
		}
		if err != nil {
			a.Status = "failed"
			a.Error = err.Error()
		}
		out = append(out, a)
	}
	return out, nil
}
func syncArchiveFile(name string, b []byte) error {
	f, err := os.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return err
	}
	_, err = f.Write(b)
	if err == nil {
		err = f.Sync()
	}
	return errors.Join(err, f.Close())
}
func (s *Store) captureWorkflowArchive(ctx context.Context, r WorkflowRun, step WorkflowStep) error {
	paths, err := archivePaths(step)
	if err != nil {
		return err
	}
	if len(paths) == 0 {
		return nil
	}
	state, err := s.archiveState(r, step)
	if err != nil {
		return err
	}
	if state.Status == "retained" {
		return s.checkArchiveFiles(r, step, state)
	}
	dir := s.archiveDir(r.ID, step.Seq)
	parent := filepath.Dir(dir)
	if err = os.MkdirAll(parent, 0700); err != nil {
		return err
	}
	temp, err := os.MkdirTemp(parent, ".capture-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(temp)
	a := WorkflowArchive{Seq: step.Seq, Status: "retained", Captured: now(), Files: []ArchivedFile{}}
	total := 0
	for _, p := range paths {
		if err = ctx.Err(); err != nil {
			return err
		}
		var b []byte
		if r.Environment != nil {
			_, b, err = s.sandboxArtifacts(ctx, r, p)
		} else {
			b, err = readLocalArchive(r.WorkspacePath, p)
		}
		if err != nil {
			return fmt.Errorf("%s: %w", p, err)
		}
		total += len(b)
		if total > archiveStepLimit {
			return errors.New("node artifacts exceed 128 MiB")
		}
		dest := filepath.Join(temp, "files", filepath.FromSlash(p))
		if err = os.MkdirAll(filepath.Dir(dest), 0700); err != nil {
			return err
		}
		if err = syncArchiveFile(dest, b); err != nil {
			return err
		}
		sum := sha256.Sum256(b)
		a.Files = append(a.Files, ArchivedFile{Path: p, Size: int64(len(b)), SHA256: hex.EncodeToString(sum[:])})
	}
	b, _ := json.Marshal(a)
	if err = syncArchiveFile(filepath.Join(temp, "manifest.json"), b); err != nil {
		return err
	}
	// Sync directories before publishing the immutable snapshot.
	err = filepath.WalkDir(temp, func(p string, d os.DirEntry, e error) error {
		if e != nil {
			return e
		}
		if d.IsDir() {
			f, e := os.Open(p)
			if e != nil {
				return e
			}
			e = f.Sync()
			return errors.Join(e, f.Close())
		}
		return nil
	})
	if err != nil {
		return err
	}
	if err = os.Rename(temp, dir); err != nil {
		return err
	}
	f, err := os.Open(parent)
	if err != nil {
		return err
	}
	err = f.Sync()
	err = errors.Join(err, f.Close())
	if err == nil {
		_ = os.Remove(dir + ".error")
	}
	return err
}
func readLocalArchive(root, p string) ([]byte, error) {
	f, err := openArtifact(root, p)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	before, err := f.Stat()
	if err != nil {
		return nil, err
	}
	if before.Size() > archiveFileLimit {
		return nil, errors.New("artifact exceeds 8 MiB")
	}
	b, err := io.ReadAll(io.LimitReader(f, archiveFileLimit+1))
	if err != nil {
		return nil, err
	}
	after, err := f.Stat()
	if err != nil {
		return nil, err
	}
	if len(b) > archiveFileLimit || before.Size() != after.Size() || !before.ModTime().Equal(after.ModTime()) || int64(len(b)) != after.Size() {
		return nil, errors.New("artifact changed during capture or exceeds 8 MiB")
	}
	return b, nil
}
func (s *Store) archivedContent(r WorkflowRun, seq int, p string) ([]byte, error) {
	if !validArchivePath(p) {
		return nil, ErrForbidden
	}
	for _, step := range r.Steps {
		if step.Seq == seq {
			a, err := s.archiveState(r, step)
			if err != nil {
				return nil, err
			}
			for _, f := range a.Files {
				if f.Path == p {
					b, err := readLocalArchive(filepath.Join(s.archiveDir(r.ID, seq), "files"), p)
					if err != nil {
						return nil, err
					}
					sum := sha256.Sum256(b)
					if int64(len(b)) != f.Size || hex.EncodeToString(sum[:]) != f.SHA256 {
						return nil, errors.New("retained artifact failed integrity verification")
					}
					return b, nil
				}
			}
		}
	}
	return nil, ErrNotFound
}
func (s *Store) checkArchiveFiles(r WorkflowRun, step WorkflowStep, a WorkflowArchive) error {
	for _, f := range a.Files {
		if _, err := s.archivedContent(r, step.Seq, f.Path); err != nil {
			return err
		}
	}
	return nil
}
func (s *Store) requireWorkflowArchives(r WorkflowRun) error {
	for _, step := range r.Steps {
		paths, err := archivePaths(step)
		if err != nil {
			return err
		}
		if len(paths) == 0 {
			continue
		}
		a, err := s.archiveState(r, step)
		if err != nil {
			return err
		}
		if a.Status != "retained" {
			return fmt.Errorf("step %d has unarchived artifacts; retain them before releasing the environment", step.Seq)
		}
		if err = s.checkArchiveFiles(r, step, a); err != nil {
			return err
		}
	}
	return nil
}

// The engine's existing per-Run job lock also protects capture from release,
// retry and advancement. A capture never executes the node again.
func (e *WorkflowEngine) startArchive(ctx context.Context, r WorkflowRun, step WorkflowStep, retry bool) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	e.connectorJobs[r.ID] = workflowConnectorJob{seq: step.Seq, cancel: cancel}
	e.connectorWG.Add(1)
	go func() {
		defer e.connectorWG.Done()
		defer cancel()
		err := e.store.captureWorkflowArchive(ctx, r, step)
		e.mu.Lock()
		defer e.mu.Unlock()
		delete(e.connectorJobs, r.ID)
		if err != nil {
			dir := e.store.archiveDir(r.ID, step.Seq)
			_ = os.MkdirAll(filepath.Dir(dir), 0700)
			_ = os.WriteFile(dir+".error", []byte(err.Error()), 0600)
			current, loadErr := e.store.WorkflowRun(Caller{Admin: true}, r.ID)
			if loadErr == nil && (current.Status == "running" || current.Status == "waiting") && current.Seq == r.Seq {
				e.fail(r, fmt.Errorf("%s%w", archiveFailurePrefix, err))
			}
			return
		}
		if retry && r.Seq == step.Seq && r.Status == "failed" && strings.HasPrefix(r.Error, archiveFailurePrefix) {
			// This is an explicit archive-only recovery. Accepted handoff and original
			// execution receipt remain untouched; history keeps the prior error.
			tx, dbErr := e.store.DB.Begin()
			if dbErr != nil {
				return
			}
			defer tx.Rollback()
			changed, dbErr := tx.Exec(`UPDATE workflow_runs SET status='running',error='',updated=? WHERE id=? AND seq=? AND status='failed' AND error=?`, now(), r.ID, r.Seq, r.Error)
			if dbErr != nil {
				return
			}
			n, dbErr := changed.RowsAffected()
			if dbErr != nil || n != 1 {
				return
			}
			if _, dbErr = tx.Exec(`UPDATE workflow_steps SET status='running',updated=? WHERE run_id=? AND seq=? AND status='failed'`, now(), r.ID, r.Seq); dbErr == nil {
				_ = tx.Commit()
			}
		}
	}()
}
func (e *WorkflowEngine) RetainArtifacts(ctx context.Context, c Caller, id string, seq int) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	r, err := e.store.WorkflowRun(c, id)
	if err != nil {
		return err
	}
	if !e.connectorQuiet(id) || !e.quiet(r.Steps[len(r.Steps)-1].ConversationID) || !slices.Contains([]string{"failed", "completed", "stopped", "cancelled"}, r.Status) {
		return ErrConflict
	}
	if err = e.store.nativeRunReady(id); err != nil {
		return err
	}
	if err = e.store.executionRunReady(id); err != nil {
		return err
	}
	for _, step := range r.Steps {
		if step.Seq == seq && step.Result != nil {
			paths, err := archivePaths(step)
			if err != nil {
				return err
			}
			if len(paths) == 0 {
				return ErrNotFound
			}
			e.startArchive(ctx, r, step, true)
			return nil
		}
	}
	return ErrNotFound
}
