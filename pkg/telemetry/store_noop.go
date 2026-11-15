//go:build !duckmon

package telemetry

import "github.com/go-logr/logr"

// openBackend is the variant for builds without the duckmon build tag. It
// returns no backend, so Telemetry stays a no-op, and tells the operator
// why nothing is written.
func openBackend(dir string, retentionDays int, logger logr.Logger) backend {
	logger.Info("telemetry directory is set but this binary was built without the duckmon build tag, telemetry is disabled", "dir", dir)
	return nil
}
