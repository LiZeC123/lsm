//go:build windows

package process

import (
	"errors"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/LiZeC123/lsm/internal/config"
)

// idleCommand keeps running for long enough to be observed on Windows. ping is
// used rather than sleep because sleep is not part of cmd.exe.
const idleCommand = "ping -n 30 127.0.0.1"

// killTree force-kills a process together with its children, mirroring what the
// product does in Stop.
//
// /T is not optional here. Windows removes a process tree only when it is asked
// to: killing the shell alone orphans its children, and an orphan keeps a handle
// on the shell's working directory.
//
// It calls taskkill directly rather than through taskkillCommand, so that a test
// which stubs that seam still gets its process tree cleaned up.
func killTree(info PIDInfo) error {
	out, err := exec.Command("taskkill", "/PID", strconv.Itoa(info.PID), "/T", "/F").CombinedOutput()
	if err != nil {
		// Already gone is not a failure.
		if waitUntil(func() bool { return !IsAlive(info) }, time.Second) {
			return nil
		}
		return fmt.Errorf("taskkill: %w: %s", err, strings.TrimSpace(string(out)))
	}
	if !waitUntil(func() bool { return !IsAlive(info) }, 5*time.Second) {
		return fmt.Errorf("pid %d is still reported alive after taskkill", info.PID)
	}
	return nil
}

// launchIdle starts the idle service in a fresh work directory and guarantees
// that its whole tree is killed afterwards.
//
// Windows keeps a handle on a process's working directory for as long as the
// process lives, so an orphaned child makes t.TempDir's RemoveAll fail with
// "the process cannot access the file because it is being used by another
// process" — after the assertions themselves have already passed. The cleanup
// is registered after t.TempDir and t.Cleanup runs LIFO, so the tree is always
// killed before the directory is removed.
func launchIdle(t *testing.T) PIDInfo {
	t.Helper()
	svc := config.Service{Command: idleCommand, WorkDir: t.TempDir()}
	pid, err := Launch(svc)
	if err != nil {
		t.Fatalf("Launch: %v", err)
	}
	info := PIDInfo{PID: pid}
	t.Cleanup(func() { _ = killTree(info) })
	return info
}

func TestLaunchAndStopLifecycle(t *testing.T) {
	info := launchIdle(t)

	if !IsAlive(info) {
		t.Fatal("the launched process should be alive")
	}
	if err := Stop(info); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	if IsAlive(info) {
		t.Fatal("the process should be gone after Stop")
	}
}

func TestLaunchReportsACommandThatCannotRun(t *testing.T) {
	svc := config.Service{Command: "definitely-not-a-real-binary --flag", WorkDir: t.TempDir()}
	pid, err := Launch(svc)
	if err == nil {
		_ = Stop(PIDInfo{PID: pid})
		t.Fatalf("Launch must fail, got pid %d", pid)
	}
	if !errors.Is(err, ErrExitedImmediately) {
		t.Fatalf("error %v does not match ErrExitedImmediately", err)
	}
	if err.Error() == "" {
		t.Fatal("the startup failure must carry the shell's reason")
	}
}

// Stop must classify a process that is already gone as not running. The PID
// comes from a helper that has finished on its own, so this does not depend on
// taskkill behaviour at all.
func TestStopReportsAnAlreadyDeadProcess(t *testing.T) {
	helper := exec.Command("cmd", "/S", "/C", "exit 0")
	if err := helper.Run(); err != nil {
		t.Fatalf("helper: %v", err)
	}

	info := PIDInfo{PID: helper.Process.Pid}
	if IsAlive(info) {
		t.Skipf("pid %d was recycled before the assertion", info.PID)
	}
	if err := Stop(info); !errors.Is(err, ErrNotRunning) {
		t.Fatalf("Stop = %v, want ErrNotRunning", err)
	}
}

// TestTaskkillRemovesTheProcess pins down the helper primitive itself, so that a
// failure here is distinguishable from a failure in Stop.
func TestTaskkillRemovesTheProcess(t *testing.T) {
	info := launchIdle(t)

	if err := killTree(info); err != nil {
		t.Fatalf("killTree: %v", err)
	}
	if IsAlive(info) {
		t.Fatalf("pid %d is still reported alive after taskkill", info.PID)
	}
}

// PID reuse is only refused on positive evidence, so a PID file without a start
// time still stops normally.
func TestStartedBeforeNeedsATimestamp(t *testing.T) {
	if startedBefore(PIDInfo{PID: 1, StartedAt: 0}) {
		t.Fatal("without a started_at there is no evidence of PID reuse")
	}
}

func TestIsAliveRejectsNonPositivePIDs(t *testing.T) {
	for _, pid := range []int{0, -1} {
		if IsAlive(PIDInfo{PID: pid}) {
			t.Errorf("IsAlive(%d) = true, want false", pid)
		}
	}
}

func TestStopRejectsAnInvalidPID(t *testing.T) {
	if err := Stop(PIDInfo{PID: 0}); err == nil {
		t.Fatal("Stop must reject pid 0")
	}
}

// The regression behind the v0.1.0 failure: while taskkill walks a process tree,
// a member can exit on its own. taskkill then reports "There is no running
// instance of the task" for it and exits non-zero even though it did terminate
// the service. Success must be judged by the process state, not by that code.
func TestStopSucceedsWhenTaskkillExitsNonZeroButTheProcessIsGone(t *testing.T) {
	info := launchIdle(t)

	restore := taskkillCommand
	taskkillCommand = func(pid int) ([]byte, error) {
		// Do the real work, then report the failure Windows reports when a tree
		// member was already gone.
		_ = exec.Command("taskkill", "/PID", strconv.Itoa(pid), "/T", "/F").Run()
		return []byte("ERROR: The process with PID 1234 (child process of PID " +
				strconv.Itoa(pid) + ") could not be terminated.\n" +
				"Reason: There is no running instance of the task.\n"),
			errors.New("exit status 255")
	}
	t.Cleanup(func() { taskkillCommand = restore })

	if err := Stop(info); err != nil {
		t.Fatalf("Stop = %v, want nil: the process is gone, so the stop succeeded", err)
	}
	if IsAlive(info) {
		t.Fatalf("pid %d should be gone", info.PID)
	}
}

// The complement: when the process really is still there, the failure must be
// reported together with taskkill's own explanation.
func TestStopFailsWhenTaskkillCannotKillTheProcess(t *testing.T) {
	info := launchIdle(t)

	restore := taskkillCommand
	taskkillCommand = func(int) ([]byte, error) {
		return []byte("ERROR: The process could not be terminated.\nReason: Access is denied.\n"),
			errors.New("exit status 1")
	}
	t.Cleanup(func() { taskkillCommand = restore })

	err := Stop(info)
	if err == nil {
		t.Fatal("Stop must fail while the process is still running")
	}
	if !strings.Contains(err.Error(), "Access is denied") {
		t.Fatalf("taskkill's explanation must be reported: %v", err)
	}
	if !IsAlive(info) {
		t.Fatal("the process must still be running for this scenario to be meaningful")
	}
}
