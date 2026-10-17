//go:build windows

package process

import (
	"errors"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/LiZeC123/lsm/internal/config"
)

const (
	// shellPath interprets a service command. /S is used next to /C so that
	// cmd.exe applies its more predictable quote stripping rules, and the
	// command string travels as a single argument (dev.md §6.2).
	shellPath = "cmd"

	// detachedProcess is DETACHED_PROCESS, which Go's syscall package does not
	// export. It stops the child from inheriting the parent console.
	detachedProcess = 0x00000008

	// processQueryLimitedInformation is PROCESS_QUERY_LIMITED_INFORMATION, also
	// unexported. It is the least privileged right that still allows a process
	// to be queried (dev.md §6.6).
	processQueryLimitedInformation = 0x1000

	// creationTolerance absorbs the difference between the one-second
	// granularity of started_at and the 100ns granularity of the recorded
	// process creation time.
	creationTolerance = 5 * time.Second

	// taskkillVerifyTimeout is how long Stop waits for the target to actually
	// disappear once taskkill returns. waitUntil returns the moment it is gone,
	// so this bound only matters when the kill genuinely did not take effect.
	taskkillVerifyTimeout = 5 * time.Second
)

// taskkillCommand runs `taskkill /PID <pid> /T /F` and returns its combined
// output. It is a variable so tests can reproduce the exit-code races of the
// real command.
var taskkillCommand = func(pid int) ([]byte, error) {
	return exec.Command("taskkill", "/PID", strconv.Itoa(pid), "/T", "/F").CombinedOutput()
}

// shellCommand builds the shell invocation that interprets a service command.
// Go assembles the arguments into a command line, so a command containing
// complex quoting has to be written the way cmd.exe expects it (dev.md §6.2).
func shellCommand(svc config.Service) *exec.Cmd {
	cmd := exec.Command(shellPath, "/S", "/C", svc.Command)
	cmd.Dir = svc.WorkDir
	return cmd
}

// Launch starts svc detached from the console and returns its PID.
//
// The standard streams are left nil, which sends them to the null device.
func Launch(svc config.Service) (int, error) {
	cmd := shellCommand(svc)
	cmd.Stdin = nil
	cmd.Stdout = nil
	cmd.Stderr = nil
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP | detachedProcess,
	}

	return awaitStartup(cmd)
}

// exitStatus reports the exit code cmd.exe recorded. Windows has no signal
// death status, but ExitCode may still be -1 for a process that was terminated
// by an external tool, so an unusable status becomes the generic failure code.
func exitStatus(err *exec.ExitError) int {
	if code := err.ExitCode(); code >= 0 {
		return code
	}
	return 1
}

// IsAlive reports whether the recorded process still exists (dev.md §6.6).
func IsAlive(info PIDInfo) bool {
	if info.PID <= 0 {
		return false
	}
	handle, err := syscall.OpenProcess(processQueryLimitedInformation, false, uint32(info.PID))
	if err != nil {
		return false
	}
	_ = syscall.CloseHandle(handle)
	return true
}

// Stop terminates the recorded process and its children (dev.md §6.5).
//
// A detached console process owns no window that could receive WM_CLOSE, so
// there is no graceful step: taskkill is asked to force the whole tree.
// ErrNotRunning is returned when the process is gone, or when the recorded PID
// was recycled by an unrelated process.
func Stop(info PIDInfo) error {
	if info.PID <= 0 {
		return errors.New("invalid pid")
	}
	if !IsAlive(info) {
		return ErrNotRunning
	}
	// A force-killed process tree must never belong to somebody else.
	if startedBefore(info) {
		return ErrNotRunning
	}

	// /T terminates the whole tree, /F forces it.
	//
	// taskkill's exit status is NOT a reliable success signal. While it walks
	// the tree a member can exit on its own, and taskkill then reports "There is
	// no running instance of the task" for that PID and exits non-zero even
	// though it did terminate the service. The outcome is therefore judged by
	// the process itself: gone means the stop succeeded, regardless of what
	// taskkill returned.
	out, err := taskkillCommand(info.PID)
	if waitUntil(func() bool { return !IsAlive(info) }, taskkillVerifyTimeout) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("taskkill /PID %d /T /F failed: %w: %s",
			info.PID, err, strings.TrimSpace(string(out)))
	}
	return fmt.Errorf("pid %d is still running after taskkill /T /F", info.PID)
}

// startedBefore reports whether the process was already running well before the
// recorded start time, which means the PID was recycled by something else
// (dev.md §7.2).
//
// It only reports true on positive evidence, so a failing API call never blocks
// a legitimate stop.
func startedBefore(info PIDInfo) bool {
	if info.StartedAt <= 0 {
		return false
	}
	handle, err := syscall.OpenProcess(processQueryLimitedInformation, false, uint32(info.PID))
	if err != nil {
		return false
	}
	defer func() { _ = syscall.CloseHandle(handle) }()

	var creation, exit, kernel, user syscall.Filetime
	if err := syscall.GetProcessTimes(handle, &creation, &exit, &kernel, &user); err != nil {
		return false
	}
	created := time.Unix(0, creation.Nanoseconds())
	return created.Before(time.Unix(info.StartedAt, 0).Add(-creationTolerance))
}
