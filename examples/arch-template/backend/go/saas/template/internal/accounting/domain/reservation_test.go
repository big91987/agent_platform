package domain

import (
	"errors"
	"testing"
)

func TestCompletionRulesAreIndependentOfStorage(t *testing.T) {
	reserved := Reservation{Units: 4}
	if err := reserved.CheckCompletion(5); !errors.Is(err, ErrInvalidUsage) {
		t.Fatalf("exceeded reservation: %v", err)
	}
	if err := reserved.CheckCompletion(-1); !errors.Is(err, ErrInvalidUsage) {
		t.Fatalf("negative usage: %v", err)
	}
	if err := reserved.CheckCompletion(0); err != nil {
		t.Fatal(err)
	}
	used := int64(3)
	completed := Reservation{Units: 4, Settled: &used}
	if err := completed.CheckCompletion(3); err != nil {
		t.Fatal(err)
	}
	if err := completed.CheckCompletion(2); !errors.Is(err, ErrConflict) {
		t.Fatalf("conflicting usage: %v", err)
	}
}
