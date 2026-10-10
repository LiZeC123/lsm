package service

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/LiZeC123/lsm/internal/config"
	"github.com/LiZeC123/lsm/internal/process"
)

func TestStatusWithoutAnyPIDFiles(t *testing.T) {
	mgr, _, _ := fixture(t, idleCommand())

	statuses, err := mgr.Status()
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if len(statuses) != 1 {
		t.Fatalf("got %d statuses, want 1", len(statuses))
	}
	got := statuses[0]
	if got.Name != "web" || got.Running || got.StalePID {
		t.Fatalf("status = %+v, want a configured but stopped service", got)
	}
	if got.PID != 0 {
		t.Errorf("PID = %d, want 0", got.PID)
	}
}

func TestStatusReportsARunningService(t *testing.T) {
	mgr, _, workDir := fixture(t, idleCommand())
	pid, err := mgr.Start("web")
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	t.Cleanup(func() { _, _ = mgr.Stop("web") })

	statuses, err := mgr.Status()
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	got := statuses[0]
	if !got.Running || got.StalePID {
		t.Fatalf("status = %+v, want running", got)
	}
	if got.PID != pid {
		t.Errorf("PID = %d, want %d", got.PID, pid)
	}
	if got.StartedAt.IsZero() {
		t.Error("StartedAt must be derived from the pid file")
	}
	if got.Uptime < 0 {
		t.Errorf("Uptime = %v, want a non-negative duration", got.Uptime)
	}
	if got.WorkDir != workDir {
		t.Errorf("WorkDir = %q, want %q", got.WorkDir, workDir)
	}
}

// ps must report a stale PID file as stopped and, being a read-only command,
// must leave the file alone (dev.md §7.3).
func TestStatusReportsAStalePIDFileAsStopped(t *testing.T) {
	mgr, cfgPath, workDir := fixture(t, idleCommand())
	pidPath := config.PIDPath(cfgPath, "web")

	stale := process.PIDInfo{PID: unusedPID, StartedAt: time.Now().Unix(), WorkDir: workDir}
	if err := process.WritePIDFile(pidPath, stale); err != nil {
		t.Fatalf("WritePIDFile: %v", err)
	}

	statuses, err := mgr.Status()
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	got := statuses[0]
	if got.Running || !got.StalePID {
		t.Fatalf("status = %+v, want stopped with StalePID set", got)
	}
	if _, err := os.Stat(pidPath); err != nil {
		t.Fatalf("ps must not remove the stale pid file: %v", err)
	}
}

func TestStatusToleratesAPIDFileWithoutATimestamp(t *testing.T) {
	mgr, cfgPath, workDir := fixture(t, idleCommand())

	// Reuse a live process so the status is "running" without lsm having
	// recorded a start time.
	live, err := mgr.Start("web")
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	t.Cleanup(func() { _, _ = mgr.Stop("web") })

	info := process.PIDInfo{PID: live, WorkDir: workDir}
	if err := process.WritePIDFile(config.PIDPath(cfgPath, "web"), info); err != nil {
		t.Fatalf("WritePIDFile: %v", err)
	}

	statuses, err := mgr.Status()
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	got := statuses[0]
	if !got.Running {
		t.Fatalf("status = %+v, want running", got)
	}
	if !got.StartedAt.IsZero() || got.Uptime != 0 {
		t.Fatalf("status = %+v, want an unknown start time", got)
	}
}

func TestStatusIsSortedAndCoversEveryService(t *testing.T) {
	dir := t.TempDir()
	workDir := filepath.Join(dir, "work")
	if err := os.MkdirAll(workDir, 0o755); err != nil {
		t.Fatal(err)
	}
	cfg := &config.Config{Services: map[string]config.Service{
		"web":    {Command: idleCommand(), WorkDir: workDir},
		"api":    {Command: idleCommand(), WorkDir: workDir},
		"worker": {Command: idleCommand(), WorkDir: workDir},
	}}
	mgr := New(cfg, filepath.Join(dir, "config.yaml"))

	statuses, err := mgr.Status()
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	got := make([]string, len(statuses))
	for i, s := range statuses {
		got[i] = s.Name
	}
	want := []string{"api", "web", "worker"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("names = %v, want %v", got, want)
		}
	}
}

func TestStatusReportsABrokenPIDFile(t *testing.T) {
	mgr, cfgPath, _ := fixture(t, idleCommand())
	pidPath := config.PIDPath(cfgPath, "web")
	if err := os.MkdirAll(filepath.Dir(pidPath), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(pidPath, []byte("garbage"), 0o600); err != nil {
		t.Fatal(err)
	}

	if _, err := mgr.Status(); err == nil {
		t.Fatal("Status must surface an unreadable pid file")
	}
}
