//go:build windows

package process

import (
	"errors"
	"os/exec"
	"strconv"
	"testing"
	"time"

	"github.com/LiZeC123/lsm/internal/config"
)

// idleCommand keeps running for long enough to be observed on Windows. ping is
// used rather than sleep because sleep is not part of cmd.exe.
const idleCommand = "ping -n 30 127.0.0.1"

func TestLaunchAndStopLifecycle(t *testing.T) {
	svc := config.Service{Command: idleCommand, WorkDir: t.TempDir()}
	pid, err := Launch(svc)
	if err != nil {
		t.Fatalf("Launch: %v", err)
	}
	t.Cleanup(func() { _ = Stop(PIDInfo{PID: pid}) })

	info := PIDInfo{PID: pid}
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

func TestStopReportsAnAlreadyDeadProcess(t *testing.T) {
	svc := config.Service{Command: idleCommand, WorkDir: t.TempDir()}
	pid, err := Launch(svc)
	if err != nil {
		t.Fatalf("Launch: %v", err)
	}
	info := PIDInfo{PID: pid}

	if out, err := exec.Command("taskkill", "/PID", strconv.Itoa(pid), "/F").CombinedOutput(); err != nil {
		t.Fatalf("taskkill: %v: %s", err, out)
	}
	if !waitUntil(func() bool { return !IsAlive(info) }, 2*time.Second) {
		t.Fatal("the helper process did not disappear")
	}

	if err := Stop(info); !errors.Is(err, ErrNotRunning) {
		t.Fatalf("Stop = %v, want ErrNotRunning", err)
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
