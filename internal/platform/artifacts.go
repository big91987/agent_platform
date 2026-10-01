package platform

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

type Artifact struct {
	Path    string `json:"path"`
	Size    int64  `json:"size"`
	Image   bool   `json:"image"`
	Updated string `json:"updated_at"`
}

func hiddenArtifact(path string) bool {
	if filepath.ToSlash(filepath.Clean(path)) == "AGENTS.md" {
		return true
	}
	for _, part := range strings.Split(filepath.ToSlash(path), "/") {
		if strings.HasPrefix(part, ".") || part == "node_modules" {
			return true
		}
	}
	return false
}
func imageArtifact(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".png", ".jpg", ".jpeg", ".gif", ".webp":
		return true
	}
	return false
}
func listArtifacts(workspace string) ([]Artifact, error) {
	out := []Artifact{}
	e := filepath.WalkDir(workspace, func(path string, d fs.DirEntry, e error) error {
		if e != nil {
			return e
		}
		rel, e := filepath.Rel(workspace, path)
		if e != nil {
			return e
		}
		if rel == "." {
			return nil
		}
		if hiddenArtifact(rel) {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if d.Type()&os.ModeSymlink != 0 {
			return nil
		}
		if d.IsDir() {
			return nil
		}
		st, e := d.Info()
		if e != nil {
			return e
		}
		if st.Mode().IsRegular() {
			out = append(out, Artifact{Path: filepath.ToSlash(rel), Size: st.Size(), Image: imageArtifact(rel), Updated: st.ModTime().UTC().Format("2006-01-02T15:04:05Z")})
		}
		return nil
	})
	if os.IsNotExist(e) {
		return out, nil
	}
	return out, e
}
func openArtifact(workspace, path string) (*os.File, error) {
	if path == "" || filepath.IsAbs(path) || strings.ContainsRune(path, 0) || hiddenArtifact(path) {
		return nil, errors.New("invalid artifact path")
	}
	clean := filepath.Clean(path)
	if clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return nil, ErrForbidden
	}
	current := workspace
	for _, part := range strings.Split(clean, string(filepath.Separator)) {
		current = filepath.Join(current, part)
		st, e := os.Lstat(current)
		if e != nil {
			return nil, e
		}
		if st.Mode()&os.ModeSymlink != 0 {
			return nil, ErrForbidden
		}
	}
	confined, e := os.OpenRoot(workspace)
	if e != nil {
		return nil, e
	}
	defer confined.Close()
	file, e := confined.Open(clean)
	if e != nil {
		return nil, e
	}
	st, e := file.Stat()
	if e != nil || !st.Mode().IsRegular() {
		file.Close()
		return nil, errors.New("artifact is not a regular file")
	}
	return file, nil
}
