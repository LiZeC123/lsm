// Package version carries the build metadata that the release pipeline injects
// at link time.
//
// The variables below are read-only at runtime and are meant to be overridden
// with -ldflags -X:
//
//	go build -ldflags "\
//	  -X github.com/LiZeC123/lsm/internal/version.Version=v0.1.0 \
//	  -X github.com/LiZeC123/lsm/internal/version.Commit=$(git rev-parse --short HEAD) \
//	  -X github.com/LiZeC123/lsm/internal/version.Date=$(date -u +%Y-%m-%dT%H:%M:%SZ)"
//
// A plain `go build` leaves the defaults, which renders as "lsm dev".
package version

import "strings"

var (
	// Version is the release tag, or "dev" when built from source.
	Version = "dev"
	// Commit is the abbreviated git revision the binary was built from.
	Commit = ""
	// Date is the build time in RFC 3339.
	Date = ""
)

// String renders the single line shown by `lsm version` and `lsm --version`.
// Missing metadata is simply left out, so a source build prints "lsm dev".
func String() string {
	banner := "lsm " + Version

	details := make([]string, 0, 2)
	if Commit != "" {
		details = append(details, "commit "+Commit)
	}
	if Date != "" {
		details = append(details, "built "+Date)
	}
	if len(details) == 0 {
		return banner
	}
	return banner + " (" + strings.Join(details, ", ") + ")"
}
