//go:build windows

package process

import (
	"errors"

	"github.com/LiZeC123/lsm/internal/config"
)

// errUnsupported marks the Windows implementation gap. Windows support is the
// M5 milestone of dev.md §11; until it lands these stubs keep the windows build
// green so the cross-compilation job in CI stays meaningful.
var errUnsupported = errors.New("windows support is not implemented yet")

// Launch starts svc detached from the console and returns its PID.
func Launch(_ config.Service) (int, error) { return 0, errUnsupported }

// IsAlive reports whether the recorded process still exists.
func IsAlive(_ PIDInfo) bool { return false }

// Stop terminates the recorded process and its children.
func Stop(_ PIDInfo) error { return errUnsupported }
