package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestExternalHandoffSnapshotsAndDoesNotReplay(t *testing.T) {
	t.Setenv("PIPELINE_TEST_TOKEN", "test-token")
	for _, status := range []int{200, 500} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			dir := t.TempDir()
			os.WriteFile(filepath.Join(dir, "prd.md"), []byte("Confirmed PRD"), 0600)
			count := 0
			remote := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				count++
				if r.Header.Get("Authorization") != "Bearer test-token" {
					t.Error("missing credential")
				}
				var body map[string]any
				json.NewDecoder(r.Body).Decode(&body)
				if body["ref"] != "main" {
					t.Error("missing ref")
				}
				w.WriteHeader(status)
				if status == 200 {
					w.Write([]byte(`{"workflow_run_id":123}`))
				}
			}))
			defer remote.Close()
			c := config{Workspace: dir, ResultFile: filepath.Join(dir, "handoff.json"), DispatchURL: remote.URL, TokenEnv: "PIPELINE_TEST_TOKEN", Ref: "main"}
			in := handoff{Summary: "User approved", Artifacts: []string{"prd.md"}}
			first, err := submit(context.Background(), c, in)
			if status == 200 && err != nil {
				t.Fatal(err)
			}
			if status == 500 && err == nil {
				t.Fatal("failure hidden")
			}
			second, err := submit(context.Background(), c, in)
			if (status == 200 && (err != nil || count != 1)) || (status == 500 && (err == nil || count != 2)) || second.SHA256 != first.SHA256 || second.Documents["prd.md"] != "Confirmed PRD" {
				t.Fatalf("retry: %+v %v count %d", second, err, count)
			}
			os.WriteFile(filepath.Join(dir, "prd.md"), []byte("changed"), 0600)
			recovered, err := submit(context.Background(), c, in)
			if status == 200 && (err != nil || recovered.Documents["prd.md"] != "Confirmed PRD" || count != 1) {
				t.Fatal("accepted retry must recover the frozen receipt without dispatching again")
			}
			if status == 500 && err == nil {
				t.Fatal("replaced uncertain evidence")
			}
		})
	}
}

func TestRegistrySelectsOnlyCurrentWorkspaceAndStage(t *testing.T) {
	root := t.TempDir()
	workspace := t.TempDir()
	t.Chdir(workspace)
	real, _ := filepath.EvalSymlinks(workspace)
	sum := sha256.Sum256([]byte(real))
	dir := filepath.Join(root, hex.EncodeToString(sum[:]))
	os.MkdirAll(dir, 0700)
	cfg := config{Workspace: real, Ref: "main", Inputs: map[string]string{"issue": "70"}}
	raw, _ := json.Marshal(cfg)
	os.WriteFile(filepath.Join(dir, "requirements.json"), raw, 0600)
	got, err := loadConfig("", root, "requirements")
	if err != nil || got.Inputs["issue"] != "70" {
		t.Fatalf("route: %+v %v", got, err)
	}
	if _, err := loadConfig("", root, "design"); err == nil {
		t.Fatal("unregistered stage routed")
	}
	t.Chdir(t.TempDir())
	if _, err := loadConfig("", root, "requirements"); err == nil {
		t.Fatal("other workspace routed")
	}
}

func TestQASelectsReturnTargetAndDispatchRetryIsSafe(t *testing.T) {
	t.Setenv("PIPELINE_TEST_TOKEN", "test-token")
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "qa.md"), []byte("Real defect report"), 0600)
	calls := 0
	remote := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		var body struct {
			Inputs map[string]string `json:"inputs"`
		}
		json.NewDecoder(r.Body).Decode(&body)
		if body.Inputs["target_stage"] != "design" {
			t.Error("QA return target was lost")
		}
		if calls == 1 {
			w.WriteHeader(503)
		} else {
			w.Write([]byte(`{"workflow_run_id":123}`))
		}
	}))
	defer remote.Close()
	c := config{Workspace: dir, ResultFile: filepath.Join(dir, "qa-result.json"), DispatchURL: remote.URL, TokenEnv: "PIPELINE_TEST_TOKEN", Ref: "main", Inputs: map[string]string{"after": "qa"}}
	if _, err := submit(context.Background(), c, handoff{Summary: "Needs a destination", Artifacts: []string{"qa.md"}}); err == nil || calls != 0 {
		t.Fatal("QA without a destination must not dispatch or default to delivery")
	}
	var in handoff
	json.Unmarshal([]byte(`{"summary":"Design defect", "artifacts":["qa.md"], "target_stage":"design"}`), &in)
	if _, err := submit(context.Background(), c, in); err == nil {
		t.Fatal("must expose dispatch failure")
	}
	out, err := submit(context.Background(), c, in)
	if err != nil || out.Delivery != "accepted" || calls != 2 {
		t.Fatalf("cannot retry failed dispatch: %+v %v calls=%d", out, err, calls)
	}
	if _, err = submit(context.Background(), c, in); err != nil || calls != 2 {
		t.Fatal("accepted dispatch replayed")
	}
	c.ResultFile = filepath.Join(dir, "dev-result.json")
	c.Inputs["after"] = "development"
	if _, err = submit(context.Background(), c, in); err != nil {
		t.Fatal("development could not return to design:", err)
	}
}

func TestDispatchReturnsDurableRunHandle(t *testing.T) {
	t.Setenv("PIPELINE_TEST_TOKEN", "test-token")
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "prd.md"), []byte("Confirmed"), 0600)
	calls := 0
	remote := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		if body["return_run_details"] != true {
			t.Error("dispatch did not request run identity")
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"workflow_run_id":12345,"html_url":"https://github.com/example/repo/actions/runs/12345"}`))
	}))
	defer remote.Close()
	c := config{Workspace: dir, ResultFile: filepath.Join(dir, "handoff.json"), DispatchURL: remote.URL, TokenEnv: "PIPELINE_TEST_TOKEN", Ref: "main"}
	in := handoff{Summary: "Confirmed", Artifacts: []string{"prd.md"}}
	for i := 0; i < 2; i++ {
		out, err := submit(context.Background(), c, in)
		if err != nil {
			t.Fatal(err)
		}
		raw, _ := json.Marshal(out)
		var got map[string]any
		json.Unmarshal(raw, &got)
		if got["run_id"] != float64(12345) || got["run_url"] != "https://github.com/example/repo/actions/runs/12345" {
			t.Fatalf("missing handle: %s", raw)
		}
	}
	if calls != 1 {
		t.Fatal("retry created a second run")
	}
}

func TestDispatchMissingHandleRemainsRetryable(t *testing.T) {
	t.Setenv("TOKEN", "test")
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, "doc.md"), []byte("approved"), 0600)
	count := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count++
		if count == 1 {
			w.Write([]byte(`{"workflow_run_id":`))
		} else {
			w.Write([]byte(`{"workflow_run_id":321,"html_url":"https://example.invalid/run/321"}`))
		}
	}))
	defer server.Close()
	c := config{Workspace: root, ResultFile: filepath.Join(root, "result.json"), DispatchURL: server.URL, TokenEnv: "TOKEN", Ref: "main"}
	in := handoff{Summary: "approved", Artifacts: []string{"doc.md"}}
	if _, err := submit(context.Background(), c, in); err == nil {
		t.Fatal("accepted dispatch without usable handle")
	}
	raw, _ := os.ReadFile(c.ResultFile)
	var receipt result
	json.Unmarshal(raw, &receipt)
	if receipt.Delivery == "accepted" {
		t.Fatal("uncertain response became final")
	}
	out, err := submit(context.Background(), c, in)
	if err != nil || out.RunID != 321 || count != 2 {
		t.Fatal(out, err, count)
	}
}

func TestPlanningStagesCanHandoffSummaryWithoutDocuments(t *testing.T) {
	t.Setenv("PIPELINE_TEST_TOKEN", "test-token")
	for _, pair := range [][2]string{{"requirements", "design"}, {"design", "development"}} {
		t.Run(pair[0], func(t *testing.T) {
			dir := t.TempDir()
			count := 0
			remote := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				count++
				var body struct {
					Inputs map[string]string `json:"inputs"`
				}
				json.NewDecoder(r.Body).Decode(&body)
				if body.Inputs["target_stage"] != pair[1] {
					t.Error("wrong handoff target")
				}
				w.Write([]byte(`{"workflow_run_id":123}`))
			}))
			defer remote.Close()
			c := config{Workspace: dir, ResultFile: filepath.Join(dir, "handoff.json"), DispatchURL: remote.URL, TokenEnv: "PIPELINE_TEST_TOKEN", Ref: "main", SourceStage: pair[0], Inputs: map[string]string{"after": pair[0]}}
			in := handoff{Summary: "现有 Issue 已明确范围与验收，沿用既有设计；本阶段无新增决策，交下游修复验证。", Artifacts: []string{}, TargetStage: pair[1]}
			first, err := submit(context.Background(), c, in)
			if err != nil {
				t.Fatal(err)
			}
			second, err := submit(context.Background(), c, in)
			if err != nil || count != 1 || len(second.Documents) != 0 || first.SHA256 != second.SHA256 || second.Handoff.Summary != in.Summary {
				t.Fatalf("summary lost or replayed: %+v %v", second, err)
			}
			in.TargetStage = "qa"
			if _, err := submit(context.Background(), c, in); err == nil {
				t.Fatal("must not skip forward stages")
			}
		})
	}
}

func TestDeliveryStagesStillRequireEvidence(t *testing.T) {
	for _, stage := range []string{"development", "qa", "unknown", ""} {
		t.Run(stage, func(t *testing.T) {
			dir := t.TempDir()
			c := config{Workspace: dir, ResultFile: filepath.Join(dir, "receipt.json"), DispatchURL: "https://example.invalid", Ref: "main", Inputs: map[string]string{"after": stage}}
			if _, err := submit(context.Background(), c, handoff{Summary: "no work", Artifacts: []string{}}); err == nil {
				t.Fatal("missing evidence accepted")
			}
		})
	}
}
