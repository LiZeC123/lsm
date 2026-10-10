package service

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/LiZeC123/lsm/internal/config"
	"github.com/LiZeC123/lsm/internal/errs"
	"github.com/LiZeC123/lsm/internal/process"
)

// unusedPID is far above the PID space of every platform lsm supports, so the
// kernel always reports it as free.
const unusedPID = 1 << 30

// idleCommand keeps running for long enough to be observed, on every platform.
func idleCommand() string {
	if runtime.GOOS == "windows" {
		return "ping -n 30 127.0.0.1"
	}
	return "sleep 30"
}

// fixture builds a manager holding a single service "web" that runs command in
// a freshly created work directory.
func fixture(t *testing.T, command string) (mgr *Manager, cfgPath, workDir string) {
	t.Helper()
	dir := t.TempDir()
	workDir = filepath.Join(dir, "work")
	if err := os.MkdirAll(workDir, 0o755); err != nil {
		t.Fatal(err)
	}
	cfgPath = filepath.Join(dir, "config.yaml")
	cfg := &config.Config{Services: map[string]config.Service{
		"web": {Command: command, WorkDir: workDir},
	}}
	return New(cfg, cfgPath), cfgPath, workDir
}

func TestStartUnknownService(t *testing.T) {
	mgr, _, _ := fixture(t, idleCommand())
	_, err := mgr.Start("ghost")
	if got := errs.CodeOf(err); got != errs.NotFound {
		t.Fatalf("CodeOf = %d, want %d (%v)", got, errs.NotFound, err)
	}
}

func TestStartWithAMissingWorkDir(t *testing.T) {
	dir := t.TempDir()
	cfg := &config.Config{Services: map[string]config.Service{
		"web": {Command: idleCommand(), WorkDir: filepath.Join(dir, "gone")},
	}}
	mgr := New(cfg, filepath.Join(dir, "config.yaml"))

	_, err := mgr.Start("web")
	if got := errs.CodeOf(err); got != errs.ProcessOp {
		t.Fatalf("CodeOf = %d, want %d (%v)", got, errs.ProcessOp, err)
	}
}

func TestStartStopLifecycle(t *testing.T) {
	mgr, cfgPath, workDir := fixture(t, idleCommand())

	pid, err := mgr.Start("web")
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	t.Cleanup(func() { _, _ = mgr.Stop("web") })

	info, err := process.ReadPIDFile(config.PIDPath(cfgPath, "web"))
	if err != nil {
		t.Fatalf("ReadPIDFile: %v", err)
	}
	if info == nil || info.PID != pid {
		t.Fatalf("pid file = %+v, want pid %d", info, pid)
	}
	if info.Command != idleCommand() || info.WorkDir != workDir {
		t.Fatalf("pid file does not describe the service: %+v", info)
	}
	if info.StartedAt == 0 {
		t.Fatal("started_at must be recorded for the uptime column")
	}

	result, err := mgr.Stop("web")
	if err != nil {
		t.Fatalf("Stop: %v", err)
	}
	if !result.WasRunning || result.PID != pid {
		t.Fatalf("Stop result = %+v, want the running pid %d", result, pid)
	}
	if _, err := os.Stat(config.PIDPath(cfgPath, "web")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("the pid file must be removed, got %v", err)
	}
}

func TestStartRefusesAnAlreadyRunningService(t *testing.T) {
	mgr, _, _ := fixture(t, idleCommand())

	if _, err := mgr.Start("web"); err != nil {
		t.Fatalf("Start: %v", err)
	}
	t.Cleanup(func() { _, _ = mgr.Stop("web") })

	_, err := mgr.Start("web")
	if got := errs.CodeOf(err); got != errs.AlreadyRunning {
		t.Fatalf("CodeOf = %d, want %d (%v)", got, errs.AlreadyRunning, err)
	}
}

func TestStartFailsWhenTheCommandCannotRun(t *testing.T) {
	mgr, cfgPath, _ := fixture(t, "definitely-not-a-real-binary")

	_, err := mgr.Start("web")
	if got := errs.CodeOf(err); got != errs.ProcessOp {
		t.Fatalf("CodeOf = %d, want %d (%v)", got, errs.ProcessOp, err)
	}
	if want := "service 'web' exited immediately after start"; !strings.HasPrefix(err.Error(), want) {
		t.Fatalf("Error() = %q, want the %q prefix", err, want)
	}
	if _, statErr := os.Stat(config.PIDPath(cfgPath, "web")); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("a failed start must not leave a pid file, got %v", statErr)
	}
}

func TestStopWithoutAPIDFileIsNotRunning(t *testing.T) {
	mgr, _, _ := fixture(t, idleCommand())

	result, err := mgr.Stop("web")
	if err != nil {
		t.Fatalf("Stop: %v", err)
	}
	if result.WasRunning {
		t.Fatalf("Stop result = %+v, want not running", result)
	}
}

func TestStopUnknownService(t *testing.T) {
	mgr, _, _ := fixture(t, idleCommand())
	_, err := mgr.Stop("ghost")
	if got := errs.CodeOf(err); got != errs.NotFound {
		t.Fatalf("CodeOf = %d, want %d (%v)", got, errs.NotFound, err)
	}
}

// A PID file whose process is gone is stale: stop must report "not running"
// and clean the file up (dev.md §7.3).
func TestStopCleansUpAStalePIDFile(t *testing.T) {
	mgr, cfgPath, workDir := fixture(t, idleCommand())

	pidPath := config.PIDPath(cfgPath, "web")
	stale := process.PIDInfo{PID: unusedPID, StartedAt: time.Now().Unix(), WorkDir: workDir}
	if err := process.WritePIDFile(pidPath, stale); err != nil {
		t.Fatalf("WritePIDFile: %v", err)
	}

	result, err := mgr.Stop("web")
	if err != nil {
		t.Fatalf("Stop: %v", err)
	}
	if result.WasRunning {
		t.Fatalf("Stop result = %+v, want not running", result)
	}
	if _, err := os.Stat(pidPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("the stale pid file must be removed, got %v", err)
	}
}

// A stale PID file must not block a new start; it is simply overwritten.
func TestStartOverwritesAStalePIDFile(t *testing.T) {
	mgr, cfgPath, workDir := fixture(t, idleCommand())

	pidPath := config.PIDPath(cfgPath, "web")
	stale := process.PIDInfo{PID: unusedPID, StartedAt: 1, WorkDir: workDir}
	if err := process.WritePIDFile(pidPath, stale); err != nil {
		t.Fatalf("WritePIDFile: %v", err)
	}

	pid, err := mgr.Start("web")
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	t.Cleanup(func() { _, _ = mgr.Stop("web") })

	info, err := process.ReadPIDFile(pidPath)
	if err != nil {
		t.Fatalf("ReadPIDFile: %v", err)
	}
	if info == nil || info.PID != pid {
		t.Fatalf("pid file = %+v, want the fresh pid %d", info, pid)
	}
}
