package platform

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"

	opensandbox "github.com/alibaba/OpenSandbox/sdks/sandbox/go"
)

// Native app-server is the sole background process type supported here. The
// caller durably owns its launch before dispatch; workflow commands stay finite.
func (s *OpenSandbox) startNative(ctx context.Context, id, run string, in opensandbox.RunCommandRequest, onEvent opensandbox.EventHandler) (SandboxCommandResult, error) {
	var result SandboxCommandResult
	if strings.TrimSpace(in.Command) == "" || strings.ContainsRune(in.Command, 0) || len(in.Argv) > 0 || in.Timeout < 1 || in.Timeout > 86400000 {
		return result, errors.New("invalid native process launch")
	}
	in.Background = true
	execd, err := s.execd(ctx, id, run)
	if err != nil {
		return result, err
	}
	err = execd.RunCommand(ctx, in, func(event opensandbox.StreamEvent) error {
		// The service has already launched this process. Retain its real identity
		// before a presentation/persistence callback can fail, so cleanup still
		// targets this one process and never needs a guessed ID or another launch.
		if err := acceptSandboxCommandID(event.Data, &result); err != nil {
			return err
		}
		if onEvent != nil {
			if err := onEvent(event); err != nil {
				return err
			}
		}
		return nil
	})
	if result.ID == "" {
		return result, errors.Join(errors.New("native process launch is unconfirmed; reconcile before continuing; replay is refused"), sandboxError("native launch", err))
	}
	if err != nil {
		cleanup, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		stopped, stopErr := s.stopNative(cleanup, id, run, result.ID)
		return stopped, errors.Join(sandboxError("native launch", err), stopErr)
	}
	status, statusErr := execd.GetCommandStatus(ctx, result.ID)
	if statusErr != nil {
		return result, sandboxError("native launch status is unconfirmed", statusErr)
	}
	if status.ID != result.ID {
		return result, errors.New("native process identity changed in launch status")
	}
	result.Stopped = !status.Running
	result.ExitCode = status.ExitCode
	if result.Stopped {
		return result, errors.New("native process exited before a connection was established")
	}
	return result, nil
}

// An independent status check prevents another interrupt when the exact owned
// execution has already ended. An unknown DELETE result is never repeated here.
func (s *OpenSandbox) stopNative(ctx context.Context, id, run, commandID string) (SandboxCommandResult, error) {
	result := SandboxCommandResult{ID: commandID}
	if !validSandboxCommandID(commandID) {
		return result, errors.New("native process identity is invalid; no process was interrupted")
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	execd, err := s.execd(ctx, id, run)
	if err != nil {
		return result, err
	}
	interrupted := false
	for {
		status, err := execd.GetCommandStatus(ctx, commandID)
		if err != nil {
			return result, sandboxError("native stop status is unconfirmed", err)
		}
		if status.ID != commandID {
			return result, errors.New("native stop returned a different execution identity")
		}
		if !status.Running {
			result.Stopped = true
			result.ExitCode = status.ExitCode
			return result, nil
		}
		if !interrupted {
			result.interruptSent = true
			if err = execd.InterruptCommand(ctx, commandID); err != nil {
				return result, sandboxError("native interruption is unconfirmed", err)
			}
			interrupted = true
		}
		select {
		case <-ctx.Done():
			return result, sandboxError("native stop is unconfirmed", ctx.Err())
		case <-time.After(100 * time.Millisecond):
		}
	}
}

// Docker has already opened its mapped port before the application listens.
// Wait only on transient transports while the same owned process is running;
// an actual HTTP refusal is final. No JSON-RPC input is sent during this wait.
func (s *OpenSandbox) connectNative(ctx context.Context, id, run, commandID, token string, attempt func(int, error) error) (*codexWebSocket, error) {
	if !validSandboxCommandID(commandID) || len(token) < 32 || len(token) > 256 || strings.ContainsAny(token, "\r\n\x00 ") {
		return nil, errors.New("native connection needs a valid execution identity and private capability")
	}
	endpoint, err := s.executionEndpoint(ctx, id, run, 8080)
	if err != nil {
		return nil, err
	}
	address, err := url.Parse(endpoint.Endpoint)
	if err != nil {
		return nil, errors.New("invalid native endpoint")
	}
	if address.Scheme == "https" {
		address.Scheme = "wss"
	} else {
		address.Scheme = "ws"
	}
	headers := http.Header{}
	for key, value := range endpoint.Headers {
		headers.Set(key, value)
	}
	if headers.Get("Authorization") != "" {
		return nil, errors.New("native capability conflicts with endpoint authorization")
	}
	headers.Set("Authorization", "Bearer "+token)
	ready, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	for count := 1; ; count++ {
		stream, connectErr := dialCodexWebSocket(ready, address.String(), headers)
		if attempt != nil {
			if err := attempt(count, connectErr); err != nil {
				if stream != nil {
					stream.Close()
				}
				return nil, err
			}
		}
		if connectErr == nil {
			stream.ctx = ctx
			return stream, nil
		}
		var failure *nativeHandshakeError
		if !errors.As(connectErr, &failure) || !failure.transient {
			return nil, connectErr
		}
		execd, err := s.execd(ready, id, run)
		if err != nil {
			return nil, err
		}
		status, err := execd.GetCommandStatus(ready, commandID)
		if err != nil {
			return nil, sandboxError("native readiness status", err)
		}
		if status.ID != commandID || !status.Running {
			return nil, errors.New("owned native process ended before initialization")
		}
		select {
		case <-ready.Done():
			return nil, errors.New("native service did not become ready before the deadline")
		case <-time.After(100 * time.Millisecond):
		}
	}
}

// Native state lives outside the shared project. Only the platform chooses this
// private directory; a native launch must not infer that upload made its HOME.
func (s *OpenSandbox) nativeHome(ctx context.Context, id, run, home string) error {
	if !path.IsAbs(home) || path.Clean(home) != home || strings.ContainsRune(home, 0) || home == "/" || home == "/workspace" {
		return errors.New("invalid private native home")
	}
	execd, err := s.execd(ctx, id, run)
	if err != nil {
		return err
	}
	return sandboxError("native home preparation unconfirmed", execd.CreateDirectory(ctx, home, 700))
}

func (s *OpenSandbox) nativeLogs(ctx context.Context, id, run, commandID string) (*opensandbox.CommandLogsResponse, error) {
	if !validSandboxCommandID(commandID) {
		return nil, errors.New("invalid native execution identity")
	}
	execd, err := s.execd(ctx, id, run)
	if err != nil {
		return nil, err
	}
	logs, err := execd.GetCommandLogs(ctx, commandID, nil)
	return logs, sandboxError("native logs", err)
}
