package platform

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

type connectorRequest struct {
	ReadIssue bool           `json:"read_issue,omitempty"`
	Endpoint  string         `json:"endpoint,omitempty"`
	Lookup    string         `json:"lookup,omitempty"`
	Marker    string         `json:"marker,omitempty"`
	Payload   map[string]any `json:"payload,omitempty"`
	Stdin     string         `json:"stdin,omitempty"`
}

func connectorExpand(text string, r WorkflowRun) string {
	return strings.NewReplacer("{{run_id}}", r.ID, "{{input}}", r.Input).Replace(text)
}
func prepareConnectorRequest(v Connector, r WorkflowRun, step WorkflowStep) (connectorRequest, error) {
	var out connectorRequest
	if err := connectorWorkspace(v, r.WorkspacePath); err != nil {
		return out, err
	}
	node := r.Definition.node(step.NodeID)
	p := node.ConnectorInput
	if v.Kind == "command" {
		previous := []WorkflowStep{}
		for _, s := range r.Steps {
			if s.Seq < step.Seq {
				previous = append(previous, s)
			}
		}
		raw, _ := json.Marshal(map[string]any{"run_id": r.ID, "seq": step.Seq, "input": r.Input, "workspace_path": r.WorkspacePath, "node": node.ID, "previous_results": previous, "parameters": r.Parameters})
		out.Stdin = string(raw) + "\n"
		return out, nil
	}
	if os.Getenv(v.TokenEnv) == "" {
		return out, fmt.Errorf("credential environment %s is not configured", v.TokenEnv)
	}
	body := r.Input
	var err error
	if p.BodyFile != "" {
		body, err = connectorBody(r.WorkspacePath, strings.ReplaceAll(p.BodyFile, "{{run_id}}", r.ID))
		if err != nil {
			return out, err
		}
	}
	title := strings.TrimSpace(connectorExpand(p.Title, r))
	if title == "" {
		title = strings.TrimSpace(r.Input)
	}
	// A task can be multi-paragraph even when used in an explicit template.
	// Keep GitHub titles on one line; the complete task stays in the body.
	title = strings.TrimSpace(strings.SplitN(title, "\n", 2)[0])
	if len([]rune(title)) > 200 {
		title = string([]rune(title)[:200])
	}
	issue := p.IssueNumber
	if v.Kind == "github.issue" && p.IssueParameter != "" {
		if value, ok := r.Parameters[p.IssueParameter]; ok {
			issue, err = strconv.Atoi(value)
			if err != nil || issue < 1 {
				return out, errors.New("issue parameter must be a positive integer")
			}
		}
	}
	if p.IssueNode != "" {
		for i := len(r.Steps) - 1; i >= 0; i-- {
			old := r.Steps[i]
			if old.NodeID != p.IssueNode || old.Status != "completed" || old.Result == nil || old.Receipt == nil {
				continue
			}
			source := r.Connectors[r.Definition.node(old.NodeID).ConnectorID]
			if source.Repository != v.Repository || (old.Receipt.Kind != "github.issue_create" && old.Receipt.Kind != "github.issue") {
				return out, errors.New("issue source must be a completed Issue receipt in this repository")
			}
			issue = old.Receipt.Number
			break
		}
		if issue == 0 {
			return out, errors.New("issue source has no completed receipt in this run")
		}
	}
	out.Marker = fmt.Sprintf("<!-- agent-platform:%s:%d -->", r.ID, step.Seq)
	body += "\n\n" + out.Marker
	repo := "/repos/" + v.Repository
	switch v.Kind {
	case "github.issue", "github.issue_create":
		if v.Kind == "github.issue" && issue > 0 {
			out.ReadIssue = true
			out.Endpoint = repo + "/issues/" + strconv.Itoa(issue)
			return out, nil
		}
		out.Endpoint = repo + "/issues"
		out.Lookup = out.Endpoint + "?state=all&sort=created&direction=desc&per_page=100"
		out.Payload = map[string]any{"title": title, "body": body}
	case "github.issue_comment":
		out.Endpoint = repo + "/issues/" + strconv.Itoa(issue) + "/comments"
		out.Lookup = out.Endpoint + "?per_page=100"
		out.Payload = map[string]any{"body": body}
	case "github.pull_request":
		head, base := connectorExpand(p.Head, r), connectorExpand(p.Base, r)
		if issue > 0 {
			body += "\n\nCloses #" + strconv.Itoa(issue)
		}
		out.Endpoint = repo + "/pulls"
		out.Lookup = out.Endpoint + "?" + url.Values{"state": {"all"}, "head": {strings.Split(v.Repository, "/")[0] + ":" + head}, "base": {base}, "per_page": {"100"}}.Encode()
		out.Payload = map[string]any{"title": title, "body": body, "head": head, "base": base, "draft": true}
	default:
		return out, errors.New("unsupported connector")
	}
	return out, nil
}

// Fixed GitHub origin prevents an imported configuration from sending the token
// to an arbitrary URL. Redirects are rejected, including repository transfers.
var githubClient = &http.Client{Timeout: 30 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}

func githubRequest(ctx context.Context, v Connector, method, path string, payload any, out any) error {
	token := os.Getenv(v.TokenEnv)
	if token == "" {
		return fmt.Errorf("credential environment %s is not configured", v.TokenEnv)
	}
	var b []byte
	var err error
	if payload != nil {
		b, err = json.Marshal(payload)
		if err != nil {
			return err
		}
	}
	req, err := http.NewRequestWithContext(ctx, method, "https://api.github.com"+path, bytes.NewReader(b))
	if err != nil {
		return errors.New("invalid GitHub request")
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2026-03-10")
	req.Header.Set("Content-Type", "application/json")
	if method == "GET" {
		req.Header.Set("Cache-Control", "no-cache")
	}
	resp, err := githubClient.Do(req)
	if err != nil {
		return fmt.Errorf("GitHub request did not return a confirmed result: %s", connectorRedact(v, err.Error()))
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("GitHub returned HTTP %d; check repository access, branch names and token permissions", resp.StatusCode)
	}
	b, err = io.ReadAll(io.LimitReader(resp.Body, 4*1024*1024+1))
	if err != nil {
		return err
	}
	if len(b) > 4*1024*1024 {
		return errors.New("GitHub response too large")
	}
	if out != nil {
		return json.Unmarshal(b, out)
	}
	return nil
}

type githubObject struct {
	PullRequest json.RawMessage `json:"pull_request,omitempty"`
	Number      int             `json:"number"`
	URL         string          `json:"html_url"`
	Body        string          `json:"body"`
	Head        struct {
		SHA string `json:"sha"`
	} `json:"head"`
}

func githubReceipt(v Connector, obj githubObject, recovered bool) ConnectorReceipt {
	return ConnectorReceipt{Kind: v.Kind, URL: obj.URL, Number: obj.Number, HeadSHA: obj.Head.SHA, Recovered: recovered}
}

// githubNotSentError proves this attempt failed before making a write. A saved
// request from an interrupted attempt cannot make this guarantee.
type githubNotSentError struct{ err error }

func (e *githubNotSentError) Error() string { return e.err.Error() }
func (e *githubNotSentError) Unwrap() error { return e.err }

func executeGitHub(ctx context.Context, v Connector, request connectorRequest, lookupOnly bool) (ConnectorReceipt, error) {
	if request.ReadIssue {
		var obj githubObject
		if err := githubRequest(ctx, v, "GET", request.Endpoint, nil, &obj); err != nil {
			return ConnectorReceipt{}, &githubNotSentError{err}
		}
		if obj.Number < 1 || len(obj.PullRequest) > 0 || obj.URL != "https://github.com/"+v.Repository+"/issues/"+strconv.Itoa(obj.Number) || request.Endpoint != "/repos/"+v.Repository+"/issues/"+strconv.Itoa(obj.Number) {
			return ConnectorReceipt{}, &githubNotSentError{errors.New("GitHub resource is not the requested Issue in this repository")}
		}
		return githubReceipt(v, obj, lookupOnly), nil
	}
	// Query the marker before a write. Recovery is deliberately read-only: an
	// absent marker never proves that an interrupted POST failed to take effect.
	attempts := 1
	if lookupOnly {
		attempts = 4
	}
	for attempt := 0; attempt < attempts; attempt++ {
		if attempt > 0 {
			timer := time.NewTimer(time.Duration(1<<uint(attempt-1)) * time.Second)
			select {
			case <-ctx.Done():
				timer.Stop()
				return ConnectorReceipt{}, ctx.Err()
			case <-timer.C:
			}
		}
		lookup := request.Lookup
		if lookupOnly {
			lookup += "&_platform_reconcile=" + newID()
		}
		for page := 1; page <= 100; page++ {
			var objects []githubObject
			if err := githubRequest(ctx, v, "GET", lookup+"&page="+strconv.Itoa(page), nil, &objects); err != nil {
				if !lookupOnly {
					return ConnectorReceipt{}, &githubNotSentError{err}
				}
				return ConnectorReceipt{}, err
			}
			for _, obj := range objects {
				if strings.Contains(obj.Body, request.Marker) {
					return githubReceipt(v, obj, true), nil
				}
			}
			if len(objects) < 100 {
				break
			}
			if page == 100 {
				err := errors.New("GitHub receipt search exceeded its bound; inspect the repository before retrying")
				if !lookupOnly {
					return ConnectorReceipt{}, &githubNotSentError{err}
				}
				return ConnectorReceipt{}, err
			}
		}
	}
	if lookupOnly {
		return ConnectorReceipt{}, errors.New("no matching GitHub receipt found; outcome remains unknown, no write was replayed; inspect the repository, then stop and return explicitly if a new operation is needed")
	}
	var obj githubObject
	if err := githubRequest(ctx, v, "POST", request.Endpoint, request.Payload, &obj); err != nil {
		return ConnectorReceipt{}, err
	}
	if obj.URL == "" {
		return ConnectorReceipt{}, errors.New("GitHub response lacked a resource URL; reconcile before retrying")
	}
	return githubReceipt(v, obj, false), nil
}
func checkConnector(ctx context.Context, v Connector) error {
	if err := validateConnector(v); err != nil {
		return err
	}
	if _, err := normalizeWorkspace(v.WorkspaceRoot); err != nil {
		return err
	}
	if v.Kind == "command" {
		st, err := os.Stat(v.Executable)
		if err != nil {
			return err
		}
		if !st.Mode().IsRegular() || st.Mode()&0111 == 0 {
			return errors.New("executable is not an executable file")
		}
		for _, ref := range v.EnvRefs {
			if _, ok := os.LookupEnv(ref); !ok {
				return fmt.Errorf("environment %s not configured", ref)
			}
		}
		return nil
	}
	var result map[string]any
	return githubRequest(ctx, v, "GET", "/repos/"+v.Repository, nil, &result)
}
func connectorRedact(v Connector, text string) string {
	refs := []string{v.TokenEnv}
	for _, ref := range v.EnvRefs {
		refs = append(refs, ref)
	}
	for _, ref := range refs {
		if value := os.Getenv(ref); value != "" {
			text = strings.ReplaceAll(text, value, "[redacted]")
		}
	}
	return text
}

// Redact before retaining the bounded head/tail, so truncation cannot expose
// pieces of a configured credential. Only a possible cross-write match remains
// pending; memory does not grow with command output.
type boundedConnectorLog struct {
	mu      sync.Mutex
	window  connectorLogWindow
	secrets []string
	pending string
	keep    int
}

type connectorLogWindow struct {
	head, tail string
	truncated  bool
}

func newBoundedConnectorLog(v Connector) *boundedConnectorLog {
	b := &boundedConnectorLog{}
	refs := []string{v.TokenEnv}
	for _, ref := range v.EnvRefs {
		refs = append(refs, ref)
	}
	seen := map[string]bool{}
	for _, ref := range refs {
		if value := os.Getenv(ref); value != "" && !seen[value] {
			b.secrets = append(b.secrets, value)
			b.keep = max(b.keep, len(value)-1)
			seen[value] = true
		}
	}
	return b
}

func (w *connectorLogWindow) append(text string) {
	const half = 12000
	n := min(half-len(w.head), len(text))
	w.head += text[:n]
	w.tail += text[n:]
	if len(w.tail) > half {
		w.tail = strings.Clone(w.tail[len(w.tail)-half:])
		w.truncated = true
	}
}

func (b *boundedConnectorLog) redactTo(w *connectorLogWindow, text string, keep int) string {
	for len(text) > keep {
		limit := len(text) - keep
		at, length := limit, 0
		for _, secret := range b.secrets {
			if i := strings.Index(text, secret); i >= 0 && i < limit && (i < at || i == at && len(secret) > length) {
				at, length = i, len(secret)
			}
		}
		w.append(text[:at])
		if length == 0 {
			return strings.Clone(text[at:])
		}
		w.append("[redacted]")
		text = text[at+length:]
	}
	return strings.Clone(text)
}

func (b *boundedConnectorLog) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	n := len(p)
	for len(p) > 0 {
		size := min(len(p), 24000)
		b.pending = b.redactTo(&b.window, b.pending+string(p[:size]), b.keep)
		p = p[size:]
	}
	return n, nil
}
func (b *boundedConnectorLog) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	w := b.window
	b.redactTo(&w, b.pending, 0)
	if !w.truncated {
		return strings.ToValidUTF8(w.head+w.tail, "")
	}
	// A retained boundary can split a UTF-8 rune; omit that partial rune.
	return strings.ToValidUTF8(w.head, "") + "\n[... output omitted; showing beginning and end ...]\n" + strings.ToValidUTF8(w.tail, "")
}
func executeConnectorCommand(ctx context.Context, v Connector, r WorkflowRun, step WorkflowStep, request connectorRequest, dataDir string) (ConnectorReceipt, error) {
	out := ConnectorReceipt{Kind: v.Kind}
	if err := checkConnector(ctx, v); err != nil {
		return out, err
	}
	home := filepath.Join(dataDir, "workflow-processes", fmt.Sprintf("%s-%d", r.ID, step.Seq))
	if err := os.MkdirAll(home, 0700); err != nil {
		return out, err
	}
	nonce := "agent-platform-run-" + newID()
	// Same process registration handshake as native Agents. No configured text is
	// interpolated into a shell program; executable and arguments remain argv.
	supervisor := `IFS= read -r start || exit 1; [ "$start" = "$0" ] || exit 1; "$@" <&0 & child=$!; wait "$child"`
	args := append([]string{"-c", supervisor, nonce, v.Executable}, v.Args...)
	cmd := exec.CommandContext(ctx, "/bin/sh", args...)
	cmd.Dir = r.WorkspacePath
	cmd.Env = []string{}
	for _, name := range []string{"PATH", "HOME", "TMPDIR", "LANG"} {
		if val, ok := os.LookupEnv(name); ok {
			cmd.Env = append(cmd.Env, name+"="+val)
		}
	}
	for key, ref := range v.EnvRefs {
		cmd.Env = append(cmd.Env, key+"="+os.Getenv(ref))
	}
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.WaitDelay = 3 * time.Second
	cmd.Cancel = func() error { return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL) }
	input, err := cmd.StdinPipe()
	if err != nil {
		return out, err
	}
	log := newBoundedConnectorLog(v)
	cmd.Stdout = log
	cmd.Stderr = log
	if err = cmd.Start(); err != nil {
		input.Close()
		return out, err
	}
	registered := false
	defer func() {
		input.Close()
		syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		if registered {
			os.Remove(filepath.Join(home, "process.json"))
		}
	}()
	if err = registerProcess(home, cmd.Process.Pid, nonce); err != nil {
		syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		cmd.Wait()
		return out, err
	}
	registered = true
	if _, err = io.WriteString(input, nonce+"\n"+request.Stdin); err != nil {
		input.Close()
		syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		cmd.Wait()
		return out, err
	}
	input.Close()
	err = cmd.Wait()
	exit := cmd.ProcessState.ExitCode()
	out.ExitCode = &exit
	out.Output = log.String()
	if ctx.Err() != nil {
		return out, errors.New("command interrupted; effects may be partial, inspect before explicitly returning to retry")
	}
	var exitErr *exec.ExitError
	if err != nil && !errors.As(err, &exitErr) {
		return out, err
	}
	return out, nil
}
