package platform

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/coder/websocket"
)

// This is only a framing adapter. Both transports use executeAppServerTransport
// for identity, native approvals, steering and unchanged execution events.
type codexWebSocket struct {
	connection *websocket.Conn
	ctx        context.Context
	pending    []byte
}

func dialCodexWebSocket(ctx context.Context, address string, headers http.Header) (*codexWebSocket, error) {
	u, err := url.Parse(address)
	if err != nil || u.Hostname() == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || (u.Scheme != "ws" && u.Scheme != "wss") {
		return nil, errors.New("invalid sandbox native endpoint")
	}
	if headers.Get("Authorization") == "" || headers.Get("OPEN-SANDBOX-API-KEY") != "" {
		return nil, errors.New("native capability is required; sandbox control credentials cannot be forwarded")
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = nil
	transport.ResponseHeaderTimeout = 10 * time.Second
	client := &http.Client{Transport: transport, CheckRedirect: func(*http.Request, []*http.Request) error { return errors.New("native redirects are refused") }}
	handshake, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	connection, _, err := websocket.Dial(handshake, address, &websocket.DialOptions{HTTPClient: client, HTTPHeader: headers})
	if err != nil {
		return nil, errors.New("sandbox native handshake failed; no task was sent")
	}
	connection.SetReadLimit(4*1024*1024 - 1)
	return &codexWebSocket{connection: connection, ctx: ctx}, nil
}

func (s *codexWebSocket) Read(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	if len(s.pending) == 0 {
		kind, raw, err := s.connection.Read(s.ctx)
		if err != nil {
			if s.ctx.Err() != nil {
				return 0, s.ctx.Err()
			}
			return 0, errors.New("sandbox native connection ended or frame exceeded 4 MiB")
		}
		if kind != websocket.MessageText {
			return 0, errors.New("sandbox native binary frame is unsupported")
		}
		s.pending = append(bytes.TrimSpace(raw), '\n')
	}
	n := copy(p, s.pending)
	s.pending = s.pending[n:]
	return n, nil
}
func (s *codexWebSocket) Write(p []byte) (int, error) {
	raw := bytes.TrimSpace(p)
	if len(raw) >= 4*1024*1024 || !json.Valid(raw) {
		return 0, errors.New("invalid or oversized native request frame")
	}
	if err := s.connection.Write(s.ctx, websocket.MessageText, raw); err != nil {
		if s.ctx.Err() != nil {
			return 0, s.ctx.Err()
		}
		return 0, errors.New("native request delivery is unconfirmed; replay is refused")
	}
	return len(p), nil
}
func (s *codexWebSocket) Close() error { return s.connection.CloseNow() }

var _ io.ReadWriteCloser = (*codexWebSocket)(nil)
