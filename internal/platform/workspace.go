package platform

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func normalizeWorkspace(path string) (string, error) {
	if !filepath.IsAbs(path) || strings.ContainsRune(path, 0) {
		return "", errors.New("workspace_path must be an absolute local directory")
	}
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		return "", fmt.Errorf("workspace unavailable: %w", err)
	}
	st, err := os.Stat(resolved)
	if err != nil || !st.IsDir() {
		return "", errors.New("workspace_path must point to an existing directory")
	}
	return resolved, nil
}

func containsPath(parent, child string) bool {
	rel, err := filepath.Rel(parent, child)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

func (c Conversation) workspace(root string) string {
	if c.WorkspacePath != "" {
		return c.WorkspacePath
	}
	return filepath.Join(root, "conversations", c.ID, "workspace")
}
