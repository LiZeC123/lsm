// Package process handles the platform specific side of service management:
// launching a detached process, stopping it, checking whether it is alive, and
// the PID files that tie those operations together.
//
// Everything in this file is platform independent; Launch, Stop and IsAlive are
// implemented separately for Unix (process_unix.go) and Windows
// (process_windows.go).
package process

import "errors"

// ErrNotRunning reports that the recorded process no longer exists, either
// because it exited on its own or because the recorded PID was recycled by an
// unrelated process.
var ErrNotRunning = errors.New("process is not running")

// ErrExitedImmediately reports that the service died inside the startup grace
// window. Because the command always runs through a shell, a missing binary or
// a broken command line can only be observed this way (dev.md §6.3).
var ErrExitedImmediately = errors.New("exited immediately after start")
