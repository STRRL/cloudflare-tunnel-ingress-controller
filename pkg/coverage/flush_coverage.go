//go:build coverage

package coverage

import (
	"log"
	"os"
	"os/signal"
	"runtime/coverage"
	"syscall"
)

// SetupSignalHandler registers a SIGUSR1 handler that flushes coverage
// counter data to GOCOVERDIR. If GOCOVERDIR is not set, this is a no-op.
// This allows collecting coverage from a long-running process without
// terminating it.
func SetupSignalHandler() {
	coverDir, exists := os.LookupEnv("GOCOVERDIR")
	if !exists {
		return
	}

	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGUSR1)
	go func() {
		for range c {
			if err := coverage.WriteCountersDir(coverDir); err != nil {
				log.Printf("coverage flush error: %v", err)
				continue
			}
			if err := coverage.ClearCounters(); err != nil {
				log.Printf("coverage clear error: %v", err)
			}
			log.Printf("coverage data flushed to %s", coverDir)
		}
	}()
}
