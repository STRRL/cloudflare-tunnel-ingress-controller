//go:build duckmon

package telemetry

import (
	"errors"
	"testing"
	"time"

	duckmon "github.com/STRRL/duckmon/bindings/go"
	"github.com/go-logr/logr"
)

func TestDuckmonBackendWritesAndReadsBack(t *testing.T) {
	dir := t.TempDir()

	tel := Open(dir, 7, logr.Discard())
	if tel.backend == nil {
		t.Fatal("expected a real duckmon backend")
	}
	tel.Span("test.span", time.Now().Add(-time.Second), errors.New("boom"), map[string]any{"key": "value"})
	tel.Log("test.log", "hello", nil)
	tel.Close()

	store, err := duckmon.Open(dir, duckmon.Options{})
	if err != nil {
		t.Fatalf("reopen telemetry store: %v", err)
	}
	defer func() {
		if err := store.Close(); err != nil {
			t.Errorf("close telemetry store: %v", err)
		}
	}()

	result, err := store.Query("select count(*) from events where name in ('test.span', 'test.log')")
	if err != nil {
		t.Fatalf("query events: %v", err)
	}
	if len(result.Rows) != 1 {
		t.Fatalf("expected one result row, got %d", len(result.Rows))
	}
	count, ok := result.Rows[0][0].(int64)
	if !ok {
		t.Fatalf("expected int64 count, got %T", result.Rows[0][0])
	}
	if count != 2 {
		t.Fatalf("expected 2 events, got %d", count)
	}
}
