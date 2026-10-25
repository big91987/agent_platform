package platform

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"path"
	"strconv"
	"strings"
	"time"
	"unicode"
)

type artifactImport struct {
	Seq       int      `json:"seq"`
	SourceRun string   `json:"source_run"`
	SourceSeq int      `json:"source_seq"`
	Paths     []string `json:"paths"`
}
type importedArtifact struct {
	SourcePath string `json:"source_path"`
	Path       string `json:"path"`
	Bytes      int64  `json:"bytes"`
	SHA256     string `json:"sha256"`
}
type artifactImportReceipt struct {
	SourceRun string             `json:"source_run"`
	SourceSeq int                `json:"source_seq"`
	Files     []importedArtifact `json:"files"`
}

// Copy retained originals, never a mutable source workspace. Each opaque file
// keeps its original bytes and source path without making imported code runnable
// by normal project file discovery. This operation does not resume a workflow.
func (e *WorkflowEngine) importArtifacts(ctx context.Context, caller Caller, id string, in artifactImport) (artifactImportReceipt, error) {
	out := artifactImportReceipt{SourceRun: in.SourceRun, SourceSeq: in.SourceSeq, Files: []importedArtifact{}}
	target, err := e.store.WorkflowRun(caller, id)
	if err != nil {
		return out, err
	}
	source, err := e.store.WorkflowRun(caller, in.SourceRun)
	if err != nil {
		return out, err
	}
	if source.Owner != target.Owner {
		return out, ErrForbidden
	}
	if !validSandboxRun(source.ID) || in.Seq < 1 || in.SourceSeq < 1 || len(in.Paths) == 0 || len(in.Paths) > 256 {
		return out, errors.New("select 1 to 256 retained files and their source step")
	}
	raws := make([][]byte, 0, len(in.Paths))
	seen := map[string]bool{}
	total := 0
	for _, p := range in.Paths {
		if !validArchivePath(p) || strings.ContainsFunc(p, unicode.IsControl) || seen[p] {
			return out, errors.New("invalid or duplicate retained artifact path")
		}
		seen[p] = true
		b, err := e.store.archivedContent(source, in.SourceSeq, p)
		if err != nil {
			return out, err
		}
		total += len(b)
		if len(b) > archiveFileLimit || total > archiveStepLimit {
			return out, errors.New("retained artifact import exceeds the size limit")
		}
		destination := path.Join("harness-output", "imports", source.ID, strconv.Itoa(in.SourceSeq), p) + ".artifact"
		if !validWorkspaceExportPath(destination) {
			return out, errors.New("retained artifact path cannot be imported")
		}
		raws = append(raws, b)
		out.Files = append(out.Files, importedArtifact{SourcePath: p, Path: destination, Bytes: int64(len(b)), SHA256: fmt.Sprintf("%x", sha256.Sum256(b))})
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()
	err = e.withWorkspaceExport(ctx, id, in.Seq, func(ctx context.Context, r WorkflowRun, dispatch logExportCommand) error {
		for i, f := range out.Files {
			if err := transferWorkspaceBytes(ctx, r, f.Path, raws[i], f.SHA256, dispatch); err != nil {
				return err
			}
		}
		return nil
	})
	return out, err
}
