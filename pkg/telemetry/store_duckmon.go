//go:build duckmon

package telemetry

import (
	"crypto/rand"
	"encoding/hex"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	duckmon "github.com/STRRL/duckmon/bindings/go"
	"github.com/go-logr/logr"
)

const serviceName = "cloudflare-tunnel-ingress-controller"

const metricSnapshotInterval = 60 * time.Second

const compactInterval = 6 * time.Hour

// duckmonBackend writes events to an embedded duckmon store. Telemetry must
// never break the controller, so every failure is logged or counted and
// dropped.
type duckmonBackend struct {
	logger        logr.Logger
	store         *duckmon.Store
	dir           string
	retentionDays int
	droppedEvents atomic.Uint64
	stop          chan struct{}
	wg            sync.WaitGroup
}

// openBackend opens the duckmon store in dir. When the store cannot be
// opened it logs the problem and returns no backend, so Telemetry stays a
// no-op.
func openBackend(dir string, retentionDays int, logger logr.Logger) backend {
	store, err := duckmon.Open(dir, duckmon.Options{
		Resource: map[string]any{"service.name": serviceName},
	})
	if err != nil {
		logger.Error(err, "open telemetry store, telemetry is disabled", "dir", dir)
		return nil
	}
	b := &duckmonBackend{
		logger:        logger,
		store:         store,
		dir:           dir,
		retentionDays: retentionDays,
		stop:          make(chan struct{}),
	}
	b.wg.Add(2)
	go b.runMetricSnapshots()
	go b.runCompaction()
	return b
}

func (b *duckmonBackend) Span(name string, start time.Time, err error, attrs map[string]any) {
	duration := time.Since(start)
	eventAttrs := make(map[string]any, len(attrs)+1)
	for key, value := range attrs {
		eventAttrs[key] = value
	}
	if err != nil {
		eventAttrs["error"] = err.Error()
	}
	b.write(duckmon.Event{
		Name:     name,
		Time:     start,
		Duration: &duration,
		TraceID:  randomHex(16),
		SpanID:   randomHex(8),
		Attrs:    eventAttrs,
	})
}

func (b *duckmonBackend) Log(name string, message string, attrs map[string]any) {
	b.write(duckmon.Event{
		Name:    name,
		Message: &message,
		Attrs:   attrs,
	})
}

// write queues one event. A failed write is dropped and counted, never
// surfaced to the caller.
func (b *duckmonBackend) write(event duckmon.Event) {
	if err := b.store.Write(event); err != nil {
		b.droppedEvents.Add(1)
	}
}

// Close stops the background goroutines and closes the store.
func (b *duckmonBackend) Close() {
	close(b.stop)
	b.wg.Wait()
	if err := b.store.Close(); err != nil {
		b.logger.Error(err, "close telemetry store")
	}
}

// runMetricSnapshots writes one process.stats event per minute with a few
// process numbers that the Prometheus metrics do not already cover.
func (b *duckmonBackend) runMetricSnapshots() {
	defer b.wg.Done()
	ticker := time.NewTicker(metricSnapshotInterval)
	defer ticker.Stop()
	for {
		select {
		case <-b.stop:
			return
		case <-ticker.C:
			b.writeMetricSnapshot()
		}
	}
}

func (b *duckmonBackend) writeMetricSnapshot() {
	var memory runtime.MemStats
	runtime.ReadMemStats(&memory)
	b.write(duckmon.Event{
		Name: "process.stats",
		Attrs: map[string]any{
			"heap_bytes":     memory.HeapAlloc,
			"goroutines":     runtime.NumGoroutine(),
			"dropped_events": b.droppedEvents.Load(),
		},
	})
}

// runCompaction merges old telemetry files and deletes days past retention,
// once at start and then on a long interval.
func (b *duckmonBackend) runCompaction() {
	defer b.wg.Done()
	b.compact()
	ticker := time.NewTicker(compactInterval)
	defer ticker.Stop()
	for {
		select {
		case <-b.stop:
			return
		case <-ticker.C:
			b.compact()
		}
	}
}

// compact runs one compaction. It recovers from panics so a library fault
// cannot take down the controller.
func (b *duckmonBackend) compact() {
	defer func() {
		if recovered := recover(); recovered != nil {
			b.logger.Info("telemetry compaction panicked", "panic", recovered)
		}
	}()
	keepDays := b.retentionDays
	_, err := duckmon.Compact(b.dir, duckmon.CompactOptions{KeepDays: &keepDays})
	if err != nil {
		b.logger.Error(err, "compact telemetry store")
	}
}

// randomHex returns byteCount random bytes as lowercase hex, as duckmon
// expects for trace and span IDs.
func randomHex(byteCount int) string {
	buffer := make([]byte, byteCount)
	if _, err := rand.Read(buffer); err != nil {
		return ""
	}
	return hex.EncodeToString(buffer)
}
