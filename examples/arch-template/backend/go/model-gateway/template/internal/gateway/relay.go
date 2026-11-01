// Package gateway coordinates a single dispatch and streaming lifecycle.
package gateway

import (
	"context"
	"io"
	"time"

	"example.com/service/internal/routing"
)

type Upstream interface {
	Open(context.Context, routing.Target, []byte) (io.ReadCloser, int, string, error)
}
type Router interface {
	Select(string) (routing.Target, error)
	Observe(routing.Target, time.Duration)
}
type Service struct {
	router   Router
	upstream Upstream
}

func New(router Router, upstream Upstream) *Service {
	return &Service{router: router, upstream: upstream}
}

// Forward never retries a request: a caller cannot safely replay after output commitment.
// The commit callback is the protocol adapter's responsibility, called only after Open succeeds.
func (s *Service) Forward(ctx context.Context, model string, payload []byte, commit func(int, string) io.Writer) error {
	target, err := s.router.Select(model)
	if err != nil {
		return err
	}
	started := time.Now()
	body, status, contentType, err := s.upstream.Open(ctx, target, payload)
	if err != nil {
		return err
	}
	defer body.Close()
	if status >= 200 && status < 300 {
		s.router.Observe(target, time.Since(started))
	}
	_, err = io.Copy(commit(status, contentType), body)
	return err
}
