// Package application exposes quota use cases and their atomic storage contract.
package application

import (
	"context"
	"errors"

	"example.com/service/internal/accounting/domain"
)

var ErrNotFound = errors.New("tenant or reservation not found")

// Repository keeps each business operation atomic, including duplicate checks.
// The implementation must check Domain rules against the values read in its transaction.
type Repository interface {
	SetLimit(context.Context, string, int64) error
	Reserve(context.Context, string, string, int64) error
	Finish(context.Context, string, string, int64) error
	Usage(context.Context, string) (domain.Quota, error)
}

type Service struct{ repository Repository }

func New(repository Repository) *Service { return &Service{repository: repository} }

func (s *Service) SetLimit(ctx context.Context, tenant string, limit int64) error {
	if tenant == "" || limit < 0 {
		return domain.ErrInvalidUsage
	}
	return s.repository.SetLimit(ctx, tenant, limit)
}

func (s *Service) Reserve(ctx context.Context, tenant, id string, units int64) error {
	if tenant == "" || id == "" || units <= 0 {
		return domain.ErrInvalidUsage
	}
	return s.repository.Reserve(ctx, tenant, id, units)
}

func (s *Service) Finish(ctx context.Context, tenant, id string, used int64) error {
	if tenant == "" || id == "" || used < 0 {
		return domain.ErrInvalidUsage
	}
	return s.repository.Finish(ctx, tenant, id, used)
}

// Usage is a read result; callers receive no mutable domain object or domain behavior.
type Usage struct {
	Tenant   string `json:"tenant"`
	Limit    int64  `json:"limit"`
	Reserved int64  `json:"reserved"`
	Used     int64  `json:"used"`
}

func (s *Service) Usage(ctx context.Context, tenant string) (Usage, error) {
	quota, err := s.repository.Usage(ctx, tenant)
	return Usage{Tenant: quota.Tenant, Limit: quota.Limit, Reserved: quota.Reserved, Used: quota.Used}, err
}
