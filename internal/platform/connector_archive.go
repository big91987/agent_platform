package platform

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const connectorArchiveLimit = 8 * 1024 * 1024
const connectorOutputPage = 32 * 1024

// ConnectorLogInfo describes the retained redacted bytes, not the total output.
// A capped or failed archive never claims to contain the complete command log.
type ConnectorLogInfo struct {
	Bytes     int64 `json:"bytes"`
	Truncated bool  `json:"truncated"`
}

type connectorArchive struct {
	file   *os.File
	info   ConnectorLogInfo
	failed bool
}

func newConnectorArchive(home string) (*connectorArchive, error) {
	file, err := os.OpenFile(filepath.Join(home, "output.log"), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return nil, err
	}
	return &connectorArchive{file: file}, nil
}
func (a *connectorArchive) append(text string) {
	if a.failed {
		return
	}
	remaining := int64(connectorArchiveLimit) - a.info.Bytes
	if int64(len(text)) > remaining {
		a.info.Truncated = true
		text = text[:remaining]
	}
	if len(text) == 0 {
		return
	}
	n, err := io.WriteString(a.file, text)
	a.info.Bytes += int64(n)
	if err != nil {
		a.failed = true
	}
}
func (b *boundedConnectorLog) finishArchive() *ConnectorLogInfo {
	b.mu.Lock()
	defer b.mu.Unlock()
	a := b.window.archive
	if a == nil {
		return nil
	}
	b.pending = b.redactTo(&b.window, b.pending, 0)
	b.window.archive = nil
	syncErr := a.file.Sync()
	closeErr := a.file.Close()
	if a.failed || syncErr != nil || closeErr != nil {
		return nil
	}
	return &a.info
}

type ConnectorOutputPage struct {
	Output     string `json:"output"`
	NextOffset int64  `json:"next_offset"`
	EOF        bool   `json:"eof"`
	Truncated  bool   `json:"truncated"`
}

// Authorization is done by the caller using the Run before reaching storage.
// Run/sequence are existing business keys; no caller-supplied filesystem path.
func (s *Store) connectorOutput(r WorkflowRun, seq int, offset int64) (ConnectorOutputPage, error) {
	var out ConnectorOutputPage
	if offset < 0 || seq < 1 || seq > len(r.Steps) {
		return out, ErrNotFound
	}
	step := r.Steps[seq-1]
	if step.Seq != seq || step.Receipt == nil || step.Receipt.Kind != "command" || step.Receipt.Log == nil {
		return out, ErrNotFound
	}
	info := step.Receipt.Log
	if offset > info.Bytes || info.Bytes > connectorArchiveLimit {
		return out, ErrNotFound
	}
	root, err := os.OpenRoot(filepath.Join(s.Dir, "workflow-processes"))
	if err != nil {
		return out, ErrNotFound
	}
	defer root.Close()
	file, err := root.Open(fmt.Sprintf("%s-%d/output.log", r.ID, seq))
	if err != nil {
		return out, ErrNotFound
	}
	defer file.Close()
	stat, err := file.Stat()
	if err != nil || !stat.Mode().IsRegular() || stat.Size() != info.Bytes {
		return out, errors.New("command log unavailable or changed")
	}
	size := min(int64(connectorOutputPage), info.Bytes-offset)
	// Include the rest of a UTF-8 rune when the byte page ends inside it.
	data := make([]byte, min(size+3, info.Bytes-offset))
	if _, err = file.ReadAt(data, offset); err != nil && err != io.EOF {
		return out, err
	}
	for size < int64(len(data)) && data[size]&0xc0 == 0x80 {
		size++
	}
	out.Output = strings.ToValidUTF8(string(data[:size]), "")
	out.NextOffset = offset + size
	out.EOF = out.NextOffset == info.Bytes
	out.Truncated = info.Truncated
	return out, nil
}
