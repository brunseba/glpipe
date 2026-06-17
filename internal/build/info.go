// Package build holds version metadata injected at build time via -ldflags.
package build

import (
	"fmt"
	"runtime"
)

// These variables are set by -ldflags at build time.
// Defaults make `go run .` still print something useful.
var (
	Version   = "dev"
	Commit    = "none"
	BuildDate = "unknown"
)

// Info returns a human-readable version string.
func Info() string {
	return fmt.Sprintf("%s (commit %s, built %s, %s/%s, %s)",
		Version, Commit, BuildDate,
		runtime.GOOS, runtime.GOARCH,
		runtime.Version(),
	)
}
