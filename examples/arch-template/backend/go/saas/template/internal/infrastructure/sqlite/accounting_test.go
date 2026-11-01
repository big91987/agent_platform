package sqlite_test

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"sync"
	"testing"

	accounting "example.com/service/internal/accounting/application"
	"example.com/service/internal/accounting/domain"
	"example.com/service/internal/infrastructure/sqlite"
)

func TestAdmissionIsAtomicAndIdempotent(t *testing.T) {
	store, err := sqlite.Open(filepath.Join(t.TempDir(), "quota.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	ctx := context.Background()
	service := accounting.New(store)
	if err = service.SetLimit(ctx, "tenant", 5); err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	successes := make(chan string, 20)
	failures := make(chan error, 20)
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			id := string(rune('a' + i))
			err := service.Reserve(ctx, "tenant", id, 1)
			if err == nil {
				successes <- id
			} else {
				failures <- err
			}
		}(i)
	}
	wg.Wait()
	close(successes)
	close(failures)
	if len(successes) != 5 {
		t.Fatalf("admitted %d, want 5", len(successes))
	}
	for err := range failures {
		if !errors.Is(err, domain.ErrQuota) {
			t.Fatal(err)
		}
	}
	for id := range successes {
		if err = service.Reserve(ctx, "tenant", id, 1); err != nil {
			t.Fatal("duplicate admission", err)
		}
	}
	quota, err := service.Usage(ctx, "tenant")
	if err != nil {
		t.Fatal(err)
	}
	if quota.Reserved != 5 || quota.Used != 0 {
		t.Fatalf("quota %+v", quota)
	}
}

func TestSettlementRollbackDuplicateAndTenantIsolation(t *testing.T) {
	store, err := sqlite.Open(filepath.Join(t.TempDir(), "quota.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	ctx := context.Background()
	service := accounting.New(store)
	for _, tenant := range []string{"one", "two"} {
		if err := service.SetLimit(ctx, tenant, 10); err != nil {
			t.Fatal(err)
		}
		if err := service.Reserve(ctx, tenant, "same-id", 4); err != nil {
			t.Fatal(err)
		}
	}
	if err := service.Finish(ctx, "one", "same-id", 5); !errors.Is(err, domain.ErrInvalidUsage) {
		t.Fatalf("over reservation: %v", err)
	}
	q, _ := service.Usage(ctx, "one")
	if q.Reserved != 4 || q.Used != 0 {
		t.Fatalf("failed settlement changed usage: %+v", q)
	}
	if err := service.Finish(ctx, "one", "same-id", 3); err != nil {
		t.Fatal(err)
	}
	if err := service.Finish(ctx, "one", "same-id", 3); err != nil {
		t.Fatal(err)
	}
	if err := service.Finish(ctx, "one", "same-id", 2); !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("conflicting completion: %v", err)
	}
	q, _ = service.Usage(ctx, "one")
	if q.Reserved != 0 || q.Used != 3 {
		t.Fatalf("settlement: %+v", q)
	}
	q, _ = service.Usage(ctx, "two")
	if q.Reserved != 4 || q.Used != 0 {
		t.Fatalf("cross-tenant update: %+v", q)
	}
	if err := service.SetLimit(ctx, "two", 3); !errors.Is(err, domain.ErrQuota) {
		t.Fatalf("lower below committed usage: %v", err)
	}
}

// A test-only SQL failure after the reservation write exercises actual transaction rollback.
func TestSettlementRollsBackBothWritesAfterDatabaseFailure(t *testing.T) {
	filename := filepath.Join(t.TempDir(), "quota.db")
	store, err := sqlite.Open(filename)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	ctx := context.Background()
	service := accounting.New(store)
	if err = service.SetLimit(ctx, "tenant", 10); err != nil {
		t.Fatal(err)
	}
	if err = service.Reserve(ctx, "tenant", "request", 4); err != nil {
		t.Fatal(err)
	}
	probe, err := sql.Open("sqlite3", filename)
	if err != nil {
		t.Fatal(err)
	}
	defer probe.Close()
	if _, err = probe.Exec(`CREATE TRIGGER reject_settlement BEFORE UPDATE OF used ON quotas BEGIN SELECT RAISE(ABORT,'test settlement failure'); END`); err != nil {
		t.Fatal(err)
	}
	if err = service.Finish(ctx, "tenant", "request", 3); err == nil {
		t.Fatal("injected database failure was ignored")
	}
	quota, err := service.Usage(ctx, "tenant")
	if err != nil {
		t.Fatal(err)
	}
	if quota.Used != 0 || quota.Reserved != 4 {
		t.Fatalf("partial quota write: %+v", quota)
	}
	if _, err = probe.Exec(`DROP TRIGGER reject_settlement`); err != nil {
		t.Fatal(err)
	}
	if err = service.Finish(ctx, "tenant", "request", 3); err != nil {
		t.Fatal(err)
	}
	quota, err = service.Usage(ctx, "tenant")
	if err != nil {
		t.Fatal(err)
	}
	if quota.Used != 3 || quota.Reserved != 0 {
		t.Fatalf("reservation write was not rolled back: %+v", quota)
	}
}
