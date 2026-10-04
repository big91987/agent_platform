// A standalone external MCP example. The platform has no dependency on this package.
package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"time"
	"unicode/utf8"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type config struct {
	SourceStage    string            `json:"source_stage,omitempty"`
	TaskFile       string            `json:"task_file,omitempty"`
	ControlCommand []string          `json:"control_command,omitempty"`
	Workspace      string            `json:"workspace"`
	ResultFile     string            `json:"result_file"`
	DispatchURL    string            `json:"dispatch_url"`
	TokenEnv       string            `json:"token_env"`
	Ref            string            `json:"ref"`
	Inputs         map[string]string `json:"inputs"`
}
type handoff struct {
	Summary     string   `json:"summary" jsonschema:"Deliverables, verification evidence, decisions and their authorization basis"`
	Artifacts   []string `json:"artifacts" jsonschema:"Workspace-relative UTF-8 documents to snapshot; requirements/design may use [] when summary records no new stage work and existing constraints"`
	TargetStage string   `json:"target_stage,omitempty" jsonschema:"Target stage: the next stage or any earlier stage. Omit for the normal next stage except QA must specify it. Return summary must include the reason, required changes and authorization basis under the current stage approval policy."`
}
type result struct {
	SourceStage string            `json:"source_stage,omitempty"`
	RunID       int64             `json:"run_id,omitempty"`
	RunURL      string            `json:"run_url,omitempty"`
	Revoked     bool              `json:"revoked,omitempty"`
	Handoff     handoff           `json:"handoff"`
	Documents   map[string]string `json:"documents"`
	SHA256      string            `json:"sha256"`
	Delivery    string            `json:"delivery"`
}

func main() {
	configPath := flag.String("config", "", "Integration-owned configuration file")
	registry := flag.String("registry", "", "Runner-owned directory of workspace registrations")
	stage := flag.String("stage", "", "Registered pipeline stage")
	hashResult := flag.String("hash-result", "", "Hash immutable handoff content")
	verify := flag.String("verify-result", "", "Verify a saved handoff without dispatching")
	submitJSON := flag.Bool("submit-json", false, "Submit a registered handoff from stdin")
	expected := flag.String("expected-sha256", "", "Expected handoff digest")
	flag.Parse()
	if *hashResult != "" {
		raw, err := os.ReadFile(*hashResult)
		var v result
		if err == nil {
			err = json.Unmarshal(raw, &v)
		}
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		fmt.Println(digest(v.Handoff, v.Documents))
		return
	}
	if *verify != "" {
		raw, err := os.ReadFile(*verify)
		var saved result
		if err == nil {
			err = json.Unmarshal(raw, &saved)
		}
		if err != nil || *expected == "" || saved.SHA256 != *expected || digest(saved.Handoff, saved.Documents) != *expected {
			fmt.Fprintln(os.Stderr, "handoff digest mismatch or unreadable result")
			os.Exit(1)
		}
		return
	}
	if *submitJSON {
		c, err := loadConfig(*configPath, *registry, *stage)
		var in handoff
		if err == nil {
			err = json.NewDecoder(os.Stdin).Decode(&in)
		}
		var out result
		if err == nil {
			out, err = submit(context.Background(), c, in)
		}
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		json.NewEncoder(os.Stdout).Encode(out)
		return
	}
	server := mcp.NewServer(&mcp.Implementation{Name: "pipeline-handoff-example", Version: "1"}, nil)
	mcp.AddTool(server, &mcp.Tool{Name: "submit_handoff", Description: "Submit stage documents to the configured external pipeline according to the Runner-provided stage approval policy; default to strict mode requiring explicit user approval for forward delivery and agent-proposed returns, except QA may return proven product defects. In autonomous mode, use evidence-backed recommendations and reversible defaults for ordinary clarification, then hand off or return ordinary rework after required checks pass without formal approval. A user may request an earlier stage in natural language; execute an explicit request without asking again. Ready-PR integration covers only mainline synchronization, conflict repair and QA reruns within accepted scope; never guess unresolved upstream product choices. Wait for the user on an indeterminate goal, incompatible goals or P0 risks of major irreversible data loss, security/privacy harm or major external financial commitments. Never waive checks, claim unverified success or fabricate user approval. Final PR merging and high-risk deployment require human authorization. Record reasons, requested changes and actual user approval or autonomous decision evidence in summary. Retrying the same failed dispatch is safe; accepted dispatches are not resent. This performs an external write."}, func(ctx context.Context, r *mcp.CallToolRequest, in handoff) (*mcp.CallToolResult, any, error) {
		cfg, err := loadConfig(*configPath, *registry, *stage)
		if err != nil {
			return nil, nil, err
		}
		output, err := submit(ctx, cfg, in)
		if err != nil {
			return nil, nil, err
		}
		raw, _ := json.Marshal(output)
		return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: string(raw)}}}, nil, nil
	})
	for _, operation := range []string{"supplement_handoff", "replace_handoff"} {
		description := "Add user-authorized information to a previously dispatched run through the external pipeline. Use the returned run_id. Do not edit shared files after handing off. Identical retries are deduplicated."
		if operation == "replace_handoff" {
			description = "Withdraw a previous handoff and dispatch to the user-requested requirements, design, development or qa stage. Stops the old execution first, preserves files/history, rejects stale handles. Pass run_id, content explaining the user's correction, and target_stage. Do not edit shared files before this operation. Identical retries return the same replacement."
		}
		mcp.AddTool(server, &mcp.Tool{Name: operation, Description: description}, func(ctx context.Context, r *mcp.CallToolRequest, in controlInput) (*mcp.CallToolResult, any, error) {
			cfg, err := loadConfig(*configPath, *registry, *stage)
			if err != nil {
				return nil, nil, err
			}
			if len(cfg.ControlCommand) == 0 {
				return nil, nil, errors.New("handoff management is not configured")
			}
			body, _ := json.Marshal(map[string]any{"operation": operation, "input": in, "workspace": cfg.Workspace, "source_stage": cfg.Inputs["after"]})
			command := exec.CommandContext(ctx, cfg.ControlCommand[0], cfg.ControlCommand[1:]...)
			command.Stdin = bytes.NewReader(body)
			output, err := command.Output()
			if err != nil {
				if e, ok := err.(*exec.ExitError); ok {
					return nil, nil, fmt.Errorf("handoff management: %s", e.Stderr)
				}
				return nil, nil, err
			}
			return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: string(output)}}}, nil, nil
		})
	}
	if err := server.Run(context.Background(), &mcp.StdioTransport{}); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

type controlInput struct {
	RunID       int64  `json:"run_id" jsonschema:"GitHub run_id returned by submit_handoff or replace_handoff"`
	Content     string `json:"content" jsonschema:"User-authorized addition or reason and requirements for replacement"`
	TargetStage string `json:"target_stage,omitempty" jsonschema:"Required for replacement: requirements, design, development or qa"`
}

func submit(ctx context.Context, c config, in handoff) (result, error) {
	var out result
	if !filepath.IsAbs(c.Workspace) || !filepath.IsAbs(c.ResultFile) || c.DispatchURL == "" || c.Ref == "" || in.Summary == "" {
		return out, errors.New("workspace, result_file, dispatch target, ref and handoff documents are required")
	}
	if len(in.Artifacts) == 0 && c.Inputs["after"] != "requirements" && c.Inputs["after"] != "design" {
		return out, errors.New("handoff documents are required outside requirements/design")
	}
	if c.Inputs["after"] == "qa" && in.TargetStage == "" {
		return out, errors.New("QA must explicitly set target_stage: requirements, design, development or report")
	}
	if in.TargetStage != "" && c.Inputs["after"] != "redirect" {
		stages := []string{"requirements", "design", "development", "qa", "report"}
		source, target := -1, -1
		for i, stage := range stages {
			if stage == c.Inputs["after"] {
				source = i
			}
			if stage == in.TargetStage {
				target = i
			}
		}
		if source < 0 || source >= 4 || target < 0 || target == source || target > source+1 {
			return out, errors.New("target_stage must be the next stage or an earlier stage")
		}
	}

	if c.TaskFile != "" {
		lock, err := os.OpenFile(c.TaskFile[:len(c.TaskFile)-len(filepath.Ext(c.TaskFile))]+".lock", os.O_CREATE|os.O_RDWR, 0600)
		if err != nil {
			return out, err
		}
		defer lock.Close()
		if err = syscall.Flock(int(lock.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
			return out, errors.New("pipeline state is busy; retry this operation")
		}
		defer syscall.Flock(int(lock.Fd()), syscall.LOCK_UN)
		var task struct {
			ActiveStage string          `json:"active_stage"`
			Replacement json.RawMessage `json:"replacement"`
		}
		raw, err := os.ReadFile(c.TaskFile)
		if err != nil {
			return out, err
		}
		if err = json.Unmarshal(raw, &task); err != nil {
			return out, err
		}
		// Accepted retries are safe, even after the receiving stage has started.
		var previous result
		old, _ := os.ReadFile(c.ResultFile)
		json.Unmarshal(old, &previous)
		if previous.Revoked {
			return out, errors.New("this handoff was withdrawn; use its replacement")
		}
		if previous.Delivery != "accepted" && (task.ActiveStage != c.Inputs["after"] || len(task.Replacement) > 0) {
			return out, errors.New("this stage is no longer active or its handoff is being replaced")
		}
	}
	// The integration chooses paths and destination; the model supplies only handoff content.
	token := os.Getenv(c.TokenEnv)
	if token == "" {
		return out, errors.New("dispatch credential is unavailable")
	}
	lock, err := os.OpenFile(c.ResultFile+".lock", os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return out, err
	}
	defer lock.Close()
	if err = syscall.Flock(int(lock.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		return out, errors.New("another handoff is running")
	}
	defer syscall.Flock(int(lock.Fd()), syscall.LOCK_UN)
	// Recover an accepted immutable receipt even after its recipient edited the
	// shared workspace. Changed handoff arguments still require a new operation.
	if raw, readErr := os.ReadFile(c.ResultFile); readErr == nil {
		var previous result
		if err := json.Unmarshal(raw, &previous); err != nil {
			return out, err
		}
		if previous.Revoked {
			return out, errors.New("this handoff was withdrawn")
		}
		if previous.Delivery == "accepted" && previous.RunID > 0 {
			if digest(in, previous.Documents) != previous.SHA256 {
				return out, errors.New("this handoff already contains different content; use supplement_handoff or replace_handoff")
			}
			return previous, nil
		}
	}
	root, err := os.OpenRoot(c.Workspace)
	if err != nil {
		return out, err
	}
	defer root.Close()
	out = result{SourceStage: c.SourceStage, Handoff: in, Documents: map[string]string{}, Delivery: "uncertain"}
	total := 0
	for _, name := range in.Artifacts {
		f, err := root.Open(name)
		if err != nil {
			return out, err
		}
		data, err := io.ReadAll(io.LimitReader(f, 5*1024*1024+1))
		f.Close()
		if err != nil {
			return out, err
		}
		total += len(data)
		if total > 5*1024*1024 || !utf8.Valid(data) {
			return out, errors.New("handoff supports at most 5 MiB of UTF-8 documents")
		}
		out.Documents[name] = string(data)
	}
	out.SHA256 = digest(in, out.Documents)
	if old, err := os.ReadFile(c.ResultFile); err == nil {
		var previous result
		if err = json.Unmarshal(old, &previous); err != nil {
			return out, err
		}
		if previous.Revoked {
			return out, errors.New("this handoff was withdrawn")
		}
		if previous.SHA256 != out.SHA256 {
			return out, errors.New("this handoff already contains different content; integration must allocate a new result file")
		}
		if previous.Delivery == "accepted" {
			return previous, nil
		}
		// The receiver deduplicates by the immutable receipt path and digest.
		// Explicit retries may resend an uncertain or rejected HTTP dispatch.
	} else if !os.IsNotExist(err) {
		return out, err
	}
	if err = save(c.ResultFile, out); err != nil {
		return out, err
	}
	inputs := map[string]string{}
	for k, v := range c.Inputs {
		inputs[k] = v
	}
	if in.TargetStage != "" {
		inputs["target_stage"] = in.TargetStage
	}
	inputs["result_file"] = c.ResultFile
	inputs["result_sha256"] = out.SHA256
	body, _ := json.Marshal(map[string]any{"ref": c.Ref, "inputs": inputs, "return_run_details": true})
	req, err := http.NewRequestWithContext(ctx, "POST", c.DispatchURL, bytes.NewReader(body))
	if err != nil {
		return out, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	client := http.Client{Timeout: 20 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	response, err := client.Do(req)
	if err != nil {
		return out, errors.New("dispatch outcome uncertain; retry submit_handoff with identical content; the receiver deduplicates it")
	}
	defer response.Body.Close()
	responseBody, readErr := io.ReadAll(io.LimitReader(response.Body, 1024*1024))
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return out, fmt.Errorf("dispatch HTTP %d; saved handoff retained; retry identical content", response.StatusCode)
	}
	if readErr == nil && response.StatusCode == http.StatusOK {
		var details struct {
			RunID   int64  `json:"workflow_run_id"`
			HTMLURL string `json:"html_url"`
		}
		if json.Unmarshal(responseBody, &details) == nil {
			out.RunID = details.RunID
			out.RunURL = details.HTMLURL
		}
	}
	if out.RunID <= 0 {
		return out, errors.New("dispatch response has no usable run_id; outcome uncertain, retry identical content")
	}
	out.Delivery = "accepted"
	if err = save(c.ResultFile, out); err != nil {
		return out, err
	}
	return out, nil
}
func save(path string, v result) error {
	raw, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".handoff-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if _, err = f.Write(raw); err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	return os.Rename(f.Name(), path)
}

// Configuration lives outside model-writable workspaces and contains the fixed destination.
// Discovery needs no task configuration; only an actual call resolves its workspace.
func loadConfig(path, registry, stage string) (config, error) {
	var c config
	var workspace string
	if registry != "" {
		if stage != "requirements" && stage != "design" && stage != "development" && stage != "qa" && stage != "review" {
			return c, errors.New("unknown registered stage")
		}
		cwd, err := os.Getwd()
		if err != nil {
			return c, err
		}
		workspace, err = filepath.EvalSymlinks(cwd)
		if err != nil {
			return c, err
		}
		sum := sha256.Sum256([]byte(workspace))
		path = filepath.Join(registry, hex.EncodeToString(sum[:]), stage+".json")
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return c, errors.New("no handoff target registered for this workspace and stage")
	}
	if err = json.Unmarshal(raw, &c); err != nil {
		return c, err
	}
	if workspace != "" && c.Workspace != workspace {
		return c, errors.New("registered workspace does not match MCP working directory")
	}
	return c, nil
}
func digest(in handoff, docs map[string]string) string {
	content, _ := json.Marshal(struct {
		Handoff   handoff
		Documents map[string]string
	}{in, docs})
	sum := sha256.Sum256(content)
	return hex.EncodeToString(sum[:])
}
