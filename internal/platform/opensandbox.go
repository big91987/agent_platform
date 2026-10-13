package platform

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"net/url"
	"path"
	"regexp"
	"strconv"
	"strings"
	"time"

	opensandbox "github.com/alibaba/OpenSandbox/sdks/sandbox/go"
)

// OpenSandbox uses the fixed official SDK. It owns no orchestration state:
// the Run must persist its create/command intent before calling these methods.
type OpenSandbox struct {
	lifecycle  *opensandbox.LifecycleClient
	httpClient *http.Client
	image      string
	ttl        int
}

type SandboxCommandResult struct {
	ID       string `json:"id"`
	ExitCode *int32 `json:"exit_code,omitempty"`
	Stopped  bool   `json:"stopped"`
}

func newOpenSandbox(endpoint, key, image string, ttl int) (*OpenSandbox, error) {
	u, err := url.Parse(endpoint)
	if err != nil || u.Hostname() == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || (u.Scheme != "http" && u.Scheme != "https") {
		return nil, errors.New("invalid OpenSandbox service URL")
	}
	if key == "" || strings.ContainsAny(key, "\r\n\x00") {
		return nil, errors.New("OpenSandbox authentication is required")
	}
	if strings.TrimSpace(image) == "" || strings.ContainsAny(image, "\x00\r\n") || ttl < 60 || ttl > 86400 {
		return nil, errors.New("choose a sandbox image and TTL between 60 and 86400 seconds")
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = nil
	transport.ResponseHeaderTimeout = 30 * time.Second
	client := &http.Client{Transport: transport, Timeout: 30 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return errors.New("OpenSandbox redirects are refused") }}
	return &OpenSandbox{lifecycle: opensandbox.NewLifecycleClient(endpoint, key, opensandbox.WithHTTPClient(client)), httpClient: client, image: image, ttl: ttl}, nil
}

func sandboxError(operation string, err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, context.Canceled) {
		return fmt.Errorf("OpenSandbox %s: %w", operation, context.Canceled)
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return fmt.Errorf("OpenSandbox %s: %w", operation, context.DeadlineExceeded)
	}
	var api *opensandbox.APIError
	if errors.As(err, &api) {
		return fmt.Errorf("OpenSandbox %s returned HTTP %d", operation, api.StatusCode)
	}
	// SDK errors can contain response bodies, credentials and private URLs.
	return fmt.Errorf("OpenSandbox %s failed; inspect the private service log", operation)
}

var sandboxRunID = regexp.MustCompile(`^[a-f0-9]{32}$`)

func validSandboxRun(run string) bool { return sandboxRunID.MatchString(run) }

func (s *OpenSandbox) create(ctx context.Context, run string) (*opensandbox.SandboxInfo, error) {
	if !validSandboxRun(run) {
		return nil, errors.New("invalid Run for sandbox creation")
	}
	ctx, cancel := context.WithTimeout(ctx, 120*time.Second)
	defer cancel()
	result, err := s.lifecycle.CreateSandbox(ctx, opensandbox.CreateSandboxRequest{Image: &opensandbox.ImageSpec{URI: s.image}, Entrypoint: []string{"tail", "-f", "/dev/null"}, Timeout: &s.ttl, ResourceLimits: opensandbox.ResourceLimits{"cpu": "2", "memory": "2Gi"}, Metadata: map[string]string{"agent-platform-run": run}})
	if err != nil {
		return nil, sandboxError("creation result is unconfirmed; reconcile before continuing", err)
	}
	if result.ID == "" || result.Metadata["agent-platform-run"] != run {
		return nil, errors.New("OpenSandbox creation returned an unconfirmed Run identity")
	}
	return result, nil
}

func (s *OpenSandbox) find(ctx context.Context, run string) (*opensandbox.SandboxInfo, error) {
	if !validSandboxRun(run) {
		return nil, errors.New("invalid Run for sandbox reconciliation")
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	var found *opensandbox.SandboxInfo
	for page := 1; page <= 20; page++ {
		list, err := s.lifecycle.ListSandboxes(ctx, opensandbox.ListOptions{Metadata: map[string]string{"agent-platform-run": run}, Page: page, PageSize: 100})
		if err != nil {
			return nil, sandboxError("reconciliation", err)
		}
		for _, item := range list.Items {
			if item.Metadata["agent-platform-run"] != run {
				return nil, errors.New("OpenSandbox returned an environment outside the requested Run")
			}
			if found != nil {
				return nil, errors.New("multiple environments belong to this Run; reconcile explicitly")
			}
			copy := item
			found = &copy
		}
		if !list.Pagination.HasNextPage {
			return found, nil
		}
	}
	return nil, errors.New("OpenSandbox reconciliation exceeded its bounded page limit")
}

func (s *OpenSandbox) owned(ctx context.Context, id, run string) (*opensandbox.SandboxInfo, error) {
	if !validSandboxRun(run) || id == "" {
		return nil, errors.New("invalid sandbox identity")
	}
	info, err := s.lifecycle.GetSandbox(ctx, id)
	if err != nil {
		return nil, sandboxError("environment lookup", err)
	}
	if info.ID != id || info.Metadata["agent-platform-run"] != run {
		return nil, errors.New("sandbox does not belong to this Run")
	}
	return info, nil
}

func (s *OpenSandbox) execd(ctx context.Context, id, run string) (*opensandbox.ExecdClient, error) {
	info, err := s.owned(ctx, id, run)
	if err != nil {
		return nil, err
	}
	if info.Status.State != opensandbox.StateRunning {
		return nil, fmt.Errorf("sandbox is %s; preserve the environment and inspect before continuing", info.Status.State)
	}
	proxy := false
	endpoint, err := s.lifecycle.GetEndpoint(ctx, id, 44772, &proxy)
	if err != nil {
		return nil, sandboxError("execution endpoint", err)
	}
	address := endpoint.Endpoint
	if !strings.Contains(address, "://") {
		address = "http://" + address
	}
	u, err := url.Parse(address)
	if err != nil || u.Hostname() == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || (u.Scheme != "http" && u.Scheme != "https") {
		return nil, errors.New("invalid sandbox execution endpoint")
	}
	// Only endpoint-scoped headers cross into the sandbox. The lifecycle API key
	// remains in the control-plane SDK client.
	return opensandbox.NewExecdClient(address, "", opensandbox.WithHTTPClient(s.httpClient), opensandbox.WithHeaders(endpoint.Headers)), nil
}

func (s *OpenSandbox) command(ctx context.Context, id, run string, in opensandbox.RunCommandRequest, onEvent opensandbox.EventHandler) (SandboxCommandResult, error) {
	var result SandboxCommandResult
	if in.Background {
		return result, errors.New("workflow commands must report their actual completion")
	}
	// The fixed execd v1.1.0 wire contract requires Command. Quote literal
	// program arguments once before dispatch; never retry a rejected/unknown call
	// using a different representation.
	if len(in.Argv) > 0 {
		if in.Command != "" || in.Argv[0] == "" {
			return result, errors.New("choose one sandbox command or program argument list")
		}
		quoted := make([]string, len(in.Argv))
		for i, arg := range in.Argv {
			if strings.ContainsRune(arg, 0) {
				return result, errors.New("sandbox program arguments cannot contain NUL")
			}
			quoted[i] = "'" + strings.ReplaceAll(arg, "'", "'\"'\"'") + "'"
		}
		in.Command = strings.Join(quoted, " ")
		in.Argv = nil
	}
	if strings.TrimSpace(in.Command) == "" || strings.ContainsRune(in.Command, 0) {
		return result, errors.New("sandbox command is required")
	}

	if in.Timeout < 0 || in.Timeout > 86400000 {
		return result, errors.New("invalid sandbox command timeout")
	}
	if in.Timeout == 0 {
		in.Timeout = 60000
	}
	ctx, cancel := context.WithTimeout(ctx, time.Duration(in.Timeout)*time.Millisecond)
	defer cancel()
	execd, err := s.execd(ctx, id, run)
	if err != nil {
		return result, err
	}
	err = execd.RunCommand(ctx, in, func(event opensandbox.StreamEvent) error {
		// Preserve the received execution event even if its identity is invalid.
		if onEvent != nil {
			if eventErr := onEvent(event); eventErr != nil {
				return eventErr
			}
		}
		var meta struct {
			Type string `json:"type"`
			Text string `json:"text"`
		}
		if json.Unmarshal([]byte(event.Data), &meta) == nil && meta.Type == "init" {
			if meta.Text == "" || result.ID != "" {
				return errors.New("sandbox command identity is unconfirmed")
			}
			result.ID = meta.Text
		}
		return nil
	})
	if result.ID == "" {
		return result, errors.Join(errors.New("sandbox command result is unconfirmed; no execution ID was received; replay is refused"), sandboxError("command start", err))
	}
	// Closing the HTTP stream alone does not stop the process. Cancel its exact
	// server execution, then query the independent status under a bounded context.
	cleanup, stop := context.WithTimeout(context.Background(), 5*time.Second)
	defer stop()
	if err != nil {
		if interruptErr := execd.InterruptCommand(cleanup, result.ID); interruptErr != nil {
			return result, sandboxError("command interruption unconfirmed", interruptErr)
		}
	}
	for {
		status, statusErr := execd.GetCommandStatus(cleanup, result.ID)
		if statusErr != nil {
			return result, sandboxError("command completion unconfirmed", statusErr)
		}
		if status.ID != result.ID {
			return result, errors.New("sandbox returned a different command status")
		}
		if !status.Running {
			result.Stopped = true
			result.ExitCode = status.ExitCode
			if err != nil {
				return result, sandboxError("command", err)
			}
			if result.ExitCode == nil {
				return result, errors.New("sandbox command ended without an exit status")
			}
			return result, nil
		}
		if err == nil {
			return result, errors.New("sandbox command stream ended while its process is still running")
		}
		select {
		case <-cleanup.Done():
			return result, errors.New("sandbox command stop was not confirmed before the cleanup deadline")
		case <-time.After(100 * time.Millisecond):
		}
	}
}

func (s *OpenSandbox) upload(ctx context.Context, id, run, target string, content []byte, mode fs.FileMode) error {
	if !path.IsAbs(target) || path.Clean(target) != target || strings.ContainsRune(target, 0) || mode & ^fs.FileMode(0777) != 0 || mode == 0 {
		return errors.New("invalid sandbox upload path or permissions")
	}
	execd, err := s.execd(ctx, id, run)
	if err != nil {
		return err
	}
	// Execd v1.1.0 parses the JSON integer as octal digits (600), whereas Go's
	// permission bits serialize as decimal 384. Keep this wire detail here.
	wireMode, err := strconv.Atoi(strconv.FormatUint(uint64(mode), 8))
	if err != nil {
		return errors.New("invalid sandbox file permissions")
	}
	return sandboxError("file upload result unconfirmed", execd.UploadFile(ctx, bytes.NewReader(content), opensandbox.UploadFileOptions{FileName: path.Base(target), Metadata: opensandbox.FileMetadata{Path: target, Mode: wireMode}}))
}
