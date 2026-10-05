package platform

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestWorkflowDeliveryReadsMergedDeploymentWithoutWriting(t *testing.T) {
	s := testStore(t)
	if err := s.initAuth("password"); err != nil {
		t.Fatal(err)
	}
	c, err := s.userCaller("admin")
	if err != nil {
		t.Fatal(err)
	}
	workspace := t.TempDir()
	connector, err := s.SaveConnector(c, Connector{Name: "PR", Kind: "github.pull_request", Repository: "demo/repo", TokenEnv: "DELIVERY_TEST_TOKEN", WorkspaceRoot: workspace, Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	w, err := s.SaveWorkflow(c, Workflow{Name: "delivery", Enabled: true, Entry: "pr", MaxSteps: 2,
		Nodes: []WorkflowNode{{ID: "pr", Name: "PR", Kind: "connector", ConnectorID: connector.ID, ConnectorInput: ConnectorInput{Head: "feature", Base: "main"}}, {ID: "done", Name: "Done", Kind: "end"}},
		Edges: []WorkflowEdge{{Source: "pr", Route: "next", Target: "done"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	run, err := s.StartWorkflow(c, WorkflowStart{WorkflowID: w.ID, Input: "Ship", WorkspacePath: workspace})
	if err != nil {
		t.Fatal(err)
	}
	h := NewServer(s, nil, nil, "password", "http://localhost")
	if _, err = h.workflowDelivery(context.Background(), c, run.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("unpublished run: %v", err)
	}
	receipt, _ := json.Marshal(ConnectorReceipt{Kind: "github.pull_request", URL: "https://github.com/demo/repo/pull/7", Number: 7, HeadSHA: strings.Repeat("a", 40)})
	if _, err = s.DB.Exec(`INSERT INTO workflow_connector_actions(run_id,seq,request,receipt) VALUES(?,?,?,?)`, run.ID, 1, `{}`, string(receipt)); err != nil {
		t.Fatal(err)
	}
	if _, err = h.workflowDelivery(context.Background(), Caller{UserID: "other"}, run.ID); !errors.Is(err, ErrForbidden) {
		t.Fatalf("another user read delivery: %v", err)
	}
	t.Setenv("DELIVERY_TEST_TOKEN", "test-token")
	original := githubClient
	t.Cleanup(func() { githubClient = original })
	requests := []string{}
	merged := false
	githubClient = &http.Client{Transport: connectorTransport(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodGet || req.Header.Get("Authorization") != "Bearer test-token" {
			t.Fatalf("unexpected GitHub request: %s", req.Method)
		}
		requests = append(requests, req.URL.Path)
		body := ""
		switch {
		case strings.HasSuffix(req.URL.Path, "/pulls/7"):
			body = `{"html_url":"https://github.com/demo/repo/pull/7","state":"open","draft":true,"merged":false,"head":{"sha":"` + strings.Repeat("b", 40) + `"}}`
			if merged {
				body = `{"html_url":"https://github.com/demo/repo/pull/7","state":"closed","draft":false,"merged":true,"merge_commit_sha":"` + strings.Repeat("c", 40) + `","head":{"sha":"` + strings.Repeat("b", 40) + `"}}`
			}
		case strings.HasSuffix(req.URL.Path, "/deployments"):
			body = `[{"id":42,"sha":"` + strings.Repeat("c", 40) + `","environment":"local-preview"}]`
		case strings.HasSuffix(req.URL.Path, "/deployments/42/statuses"):
			body = `[{"state":"success","environment_url":"http://127.0.0.1:5545/admin/","log_url":"https://github.com/demo/repo/actions/runs/1"}]`
		default:
			t.Fatalf("unexpected GitHub path: %s", req.URL.Path)
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body)), Header: http.Header{}}, nil
	})}
	before, err := h.workflowDelivery(context.Background(), c, run.ID)
	if err != nil || before.Merged || before.MainSHA != "" || len(before.Deployments) != 0 || before.CurrentHeadSHA == before.RunHeadSHA || len(requests) != 1 {
		t.Fatalf("draft PR was overstated: %+v, requests=%v, err=%v", before, requests, err)
	}
	merged = true
	after, err := h.workflowDelivery(context.Background(), c, run.ID)
	if err != nil || !after.Merged || after.MainSHA != strings.Repeat("c", 40) || len(after.Deployments) != 1 || after.Deployments[0].State != "success" || after.Deployments[0].URL != "http://127.0.0.1:5545/admin/" || len(requests) != 4 {
		t.Fatalf("merged deployment facts: %+v, requests=%v, err=%v", after, requests, err)
	}
}
