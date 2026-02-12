//go:build !coverage

package coverage

// SetupSignalHandler is a no-op when built without the "coverage" build tag.
func SetupSignalHandler() {}
