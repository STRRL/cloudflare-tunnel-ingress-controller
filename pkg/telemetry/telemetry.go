// Package telemetry records spans and logs in an embedded duckmon store.
// The real store is only compiled with the duckmon build tag; the default
// build uses a no-op backend and never touches cgo.
package telemetry

import (
	"time"

	"github.com/go-logr/logr"
)

// backend is the storage behind Telemetry. A nil backend means telemetry
// is disabled.
type backend interface {
	Span(name string, start time.Time, err error, attrs map[string]any)
	Log(name string, message string, attrs map[string]any)
	Close()
}

// Telemetry records spans and logs. A nil pointer and the zero value are
// safe no-ops, so callers never need a nil check.
type Telemetry struct {
	backend backend
}

// Open returns a Telemetry that writes to the store in dir and keeps
// retentionDays days of data. An empty dir disables telemetry. Open never
// fails; when the store cannot be opened it logs the problem and returns
// a no-op Telemetry.
func Open(dir string, retentionDays int, logger logr.Logger) *Telemetry {
	if dir == "" {
		return &Telemetry{}
	}
	return &Telemetry{backend: openBackend(dir, retentionDays, logger)}
}

// Span records one finished operation with its duration since start. A non
// nil err is stored in the error attribute.
func (t *Telemetry) Span(name string, start time.Time, err error, attrs map[string]any) {
	if t == nil || t.backend == nil {
		return
	}
	t.backend.Span(name, start, err, attrs)
}

// Log records one log event.
func (t *Telemetry) Log(name string, message string, attrs map[string]any) {
	if t == nil || t.backend == nil {
		return
	}
	t.backend.Log(name, message, attrs)
}

// Close flushes and closes the store. It is safe on a no-op Telemetry.
func (t *Telemetry) Close() {
	if t == nil || t.backend == nil {
		return
	}
	t.backend.Close()
}
