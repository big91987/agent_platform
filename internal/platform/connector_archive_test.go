package platform

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestConnectorArchiveRetainsMiddleFailureAndRedacts(t *testing.T) {
	dir, workspace := t.TempDir(), t.TempDir()
	t.Setenv("ARCHIVE_SECRET", "private-credential-sentinel")
	payload := strings.Repeat("prefix\n", 4000) + "actual middle failure: private-credential-sentinel\n" + strings.Repeat("success\n", 4000)
	input := filepath.Join(workspace, "input.txt")
	if err := os.WriteFile(input, []byte(payload), 0600); err != nil {
		t.Fatal(err)
	}
	v := Connector{Name: "archive", Kind: "command", Enabled: true, WorkspaceRoot: workspace, Executable: "/bin/cat", Args: []string{input}, EnvRefs: map[string]string{"SECRET": "ARCHIVE_SECRET"}, TimeoutSeconds: 5}
	receipt, err := executeConnectorCommand(context.Background(), v, WorkflowRun{ID: "archive-run", WorkspacePath: workspace}, WorkflowStep{Seq: 1}, connectorRequest{}, dir)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(receipt.Output, "actual middle failure") {
		t.Fatal("fixture did not exercise missing middle")
	}
	if receipt.Log == nil || receipt.Log.Truncated {
		t.Fatalf("missing durable log: %+v", receipt)
	}
	data, err := os.ReadFile(filepath.Join(dir, "workflow-processes", "archive-run-1", "output.log"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "actual middle failure: [redacted]") || strings.Contains(string(data), "private-credential") {
		t.Fatal("archive lost failure or exposed credential")
	}
	if receipt.Log.Bytes != int64(len(data)) {
		t.Fatal("wrong retained byte count")
	}
}

func TestConnectorArchiveBoundAndChunkRedaction(t *testing.T) {
	home := t.TempDir()
	archive, err := newConnectorArchive(home)
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("ARCHIVE_SECRET", "sensitive-cross-write")
	log := newBoundedConnectorLog(Connector{TokenEnv: "ARCHIVE_SECRET"})
	log.window.archive = archive
	for _, part := range []string{"prefix sensitive-", "cross-write middle\n", strings.Repeat("x", connectorArchiveLimit), "terminal"} {
		if _, err := log.Write([]byte(part)); err != nil {
			t.Fatal(err)
		}
	}
	info := log.finishArchive()
	if info == nil || !info.Truncated || info.Bytes != connectorArchiveLimit {
		t.Fatalf("unbounded archive: %+v", info)
	}
	data, err := os.ReadFile(filepath.Join(home, "output.log"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(data), "prefix [redacted] middle\n") || strings.Contains(string(data), "sensitive") {
		t.Fatal("archive redaction failed")
	}
	if !strings.HasSuffix(log.String(), "terminal") {
		t.Fatal("archive cap changed live receipt")
	}
	if _, err := newConnectorArchive(home); err == nil {
		t.Fatal("reopened immutable execution log")
	}
}

func TestConnectorArchiveHTTPIsolationAndLegacy(t *testing.T) {
	s := testStore(t)
	_, _, run := runFixture(t, s)
	h := NewServer(s, nil, nil, "password", "http://localhost")
	cookie := workflowAdmin(t, h)
	path := "/api/workflow-runs/" + run.ID + "/steps/1/output"
	if got := workflowRequest(t, h, cookie, "GET", path, nil); got.Code != 404 {
		t.Fatal("legacy execution claims log", got.Code)
	}
	home := filepath.Join(s.Dir, "workflow-processes", run.ID+"-1")
	if err := os.MkdirAll(home, 0700); err != nil {
		t.Fatal(err)
	}
	data := strings.Repeat("中", 22000) + "middle failure\n"
	if err := os.WriteFile(filepath.Join(home, "output.log"), []byte(data), 0600); err != nil {
		t.Fatal(err)
	}
	receipt, _ := json.Marshal(ConnectorReceipt{Kind: "command", Log: &ConnectorLogInfo{Bytes: int64(len(data))}})
	if _, err := s.DB.Exec(`INSERT INTO workflow_connector_actions(run_id,seq,request,receipt) VALUES(?,?,?,?)`, run.ID, 1, `{}`, string(receipt)); err != nil {
		t.Fatal(err)
	}
	a := testAgent(t, s)
	_, token := testUser(t, s, &a, "unrelated")
	if got := requestJSON(t, h, "GET", path, token, nil); got.Code != 403 {
		t.Fatal("unrelated user read output", got.Code)
	}
	if got := requestJSON(t, h, "GET", path, "", nil); got.Code != 401 {
		t.Fatal("anonymous read output", got.Code)
	}
	offset := int64(0)
	combined := ""
	for {
		got := workflowRequest(t, h, cookie, "GET", fmt.Sprintf("%s?offset=%d", path, offset), nil)
		if got.Code != 200 {
			t.Fatal(got.Code, got.Body.String())
		}
		var page ConnectorOutputPage
		if err := json.Unmarshal(got.Body.Bytes(), &page); err != nil {
			t.Fatal(err)
		}
		combined += page.Output
		if page.EOF {
			break
		}
		if page.NextOffset <= offset {
			t.Fatal("nonprogressing pagination")
		}
		offset = page.NextOffset
	}
	if combined != data {
		t.Fatal("pagination lost multibyte log evidence")
	}
	if got := workflowRequest(t, h, cookie, "GET", path+"?format=text", nil); got.Code != 200 || got.Body.String() != data || got.Header().Get("Content-Type") != "text/plain; charset=utf-8" {
		t.Fatal("plain user log unavailable or not inert")
	}
	if got := workflowRequest(t, h, cookie, "GET", path+"?offset=-1", nil); got.Code != 404 {
		t.Fatal("negative offset accepted")
	}
	// Simulate storage disappearing only after the successful first page.
	interrupted := &interruptLogWriter{ResponseRecorder: httptest.NewRecorder(), path: filepath.Join(home, "output.log")}
	request := httptest.NewRequest("GET", path+"?format=text", nil)
	request.AddCookie(cookie)
	h.ServeHTTP(interrupted, request)
	if !strings.Contains(interrupted.Body.String(), "output is incomplete") {
		t.Fatal("streamed log silently ended after disk failure")
	}

	if err := os.Symlink(filepath.Join(t.TempDir(), "outside"), filepath.Join(home, "output.log")); err != nil {
		t.Fatal(err)
	}
	if got := workflowRequest(t, h, cookie, "GET", path, nil); got.Code == 200 {
		t.Fatal("escaped log root")
	}
}

type interruptLogWriter struct {
	*httptest.ResponseRecorder
	path string
}

func (w *interruptLogWriter) Write(p []byte) (int, error) {
	n, e := w.ResponseRecorder.Write(p)
	if w.path != "" {
		os.Remove(w.path)
		w.path = ""
	}
	return n, e
}
