package domain

import (
	"errors"
	"math"
	"testing"
)

func TestQuotaReservationBoundaries(t *testing.T) {
	for _, tc := range []struct {
		name  string
		quota Quota
		units int64
		want  error
	}{
		{"zero request", Quota{Limit: 5}, 0, ErrInvalidUsage},
		{"exact remaining", Quota{Limit: 5, Used: 2, Reserved: 1}, 2, nil},
		{"exceeds remaining", Quota{Limit: 5, Used: 2, Reserved: 1}, 3, ErrQuota},
		{"largest valid value", Quota{Limit: math.MaxInt64, Used: math.MaxInt64 - 1}, 1, nil},
		{"overflow-sized request", Quota{Limit: math.MaxInt64, Used: math.MaxInt64 - 1}, math.MaxInt64, ErrQuota},
		{"invalid stored totals", Quota{Limit: math.MaxInt64, Used: math.MaxInt64, Reserved: 1}, 1, ErrQuota},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := tc.quota.CanReserve(tc.units); !errors.Is(err, tc.want) {
				t.Fatalf("CanReserve(%d) = %v, want %v", tc.units, err, tc.want)
			}
		})
	}
}
