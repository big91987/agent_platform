// Package domain owns quota and reservation rules independently of storage.
package domain

import (
	"errors"
)

var (
	ErrQuota        = errors.New("quota exhausted")
	ErrInvalidUsage = errors.New("usage must be nonnegative and within reservation")
	ErrConflict     = errors.New("request already exists with different usage")
)

type Quota struct {
	Tenant   string
	Limit    int64
	Reserved int64
	Used     int64
}

type Reservation struct {
	Units   int64
	Settled *int64
}

func (r Reservation) CheckCompletion(used int64) error {
	if used < 0 || used > r.Units {
		return ErrInvalidUsage
	}
	if r.Settled != nil && *r.Settled != used {
		return ErrConflict
	}
	return nil
}

func (q Quota) CanReserve(units int64) error {
	if units <= 0 {
		return ErrInvalidUsage
	}
	// Subtraction avoids overflowing the aggregate when comparing untrusted counts.
	if q.Limit < q.Used || q.Limit-q.Used < q.Reserved || units > q.Limit-q.Used-q.Reserved {
		return ErrQuota
	}
	return nil
}
