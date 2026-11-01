package routing

import (
	"testing"
	"time"
)

func TestLatencySelectionRejectsOldConfigurationObservations(t *testing.T) {
	pool := New([]Target{{ID: "one", Model: "chat", URL: "http://one", Version: 1}, {ID: "two", Model: "chat", URL: "http://two", Version: 1}})
	one, err := pool.Select("chat")
	if err != nil {
		t.Fatal(err)
	}
	pool.Observe(one, 100*time.Millisecond)
	pool.Observe(Target{ID: "two", Version: 1}, 20*time.Millisecond)
	selected, _ := pool.Select("chat")
	if selected.ID != "two" {
		t.Fatalf("selected %s", selected.ID)
	}
	pool.Replace([]Target{{ID: "one", Model: "chat", URL: "http://one", Version: 2}, {ID: "two", Model: "chat", URL: "http://two", Version: 2}})
	pool.Observe(Target{ID: "two", Version: 1}, time.Nanosecond)
	selected, _ = pool.Select("chat")
	if selected.ID != "one" {
		t.Fatal("old configuration contaminated routing")
	}
	if _, err := pool.Select("missing"); err == nil {
		t.Fatal("missing model accepted")
	}
}
