// Package routing owns target selection and its observation state; it performs no I/O.
package routing

import (
	"errors"
	"sync"
	"time"
)

type Target struct {
	ID, Model, URL string
	Version        int64
}
type Pool struct {
	mu        sync.Mutex
	targets   []Target
	latencies map[string]time.Duration
}

func New(targets []Target) *Pool { p := &Pool{}; p.Replace(targets); return p }

// Replace resets observations: scores from an older configuration cannot survive a change.
func (p *Pool) Replace(targets []Target) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.targets = append([]Target(nil), targets...)
	p.latencies = map[string]time.Duration{}
}
func (p *Pool) Select(model string) (Target, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	var selected Target
	var best time.Duration
	for _, target := range p.targets {
		if target.Model != model {
			continue
		}
		latency := p.latencies[target.ID]
		if selected.ID == "" || (latency > 0 && (best == 0 || latency < best)) {
			selected = target
			best = latency
		}
	}
	if selected.ID == "" {
		return Target{}, errors.New("no target for model")
	}
	return selected, nil
}
func (p *Pool) Observe(target Target, latency time.Duration) {
	if latency <= 0 {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	for _, current := range p.targets {
		if current.ID == target.ID && current.Version == target.Version {
			p.latencies[target.ID] = latency
			return
		}
	}
}
