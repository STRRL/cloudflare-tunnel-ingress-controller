package telemetry

import (
	"errors"
	"testing"
	"time"

	"github.com/go-logr/logr"
)

func TestOpenWithEmptyDirReturnsNoOp(t *testing.T) {
	tel := Open("", 7, logr.Discard())
	if tel == nil {
		t.Fatal("Open returned nil")
	}
	tel.Span("test.span", time.Now(), errors.New("boom"), map[string]any{"key": "value"})
	tel.Log("test.log", "message", nil)
	tel.Close()
}

func TestNilTelemetryIsSafe(t *testing.T) {
	var tel *Telemetry
	tel.Span("test.span", time.Now(), nil, nil)
	tel.Log("test.log", "message", nil)
	tel.Close()
}
