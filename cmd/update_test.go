package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LiZeC123/lsm/internal/config"
	"github.com/LiZeC123/lsm/internal/errs"
)

// updateFixture writes a config holding one service and returns its path, the
// service's work directory and the stored config.
func updateFixture(t *testing.T) (cfgPath, workDir string, cfg *config.Config) {
	t.Helper()
	cfgPath, workDir = servicePaths(t)
	cfg = &config.Config{Services: map[string]config.Service{
		"web": {Command: idleCommand(), WorkDir: filepath.ToSlash(workDir)},
	}}
	if err := cfg.Save(cfgPath); err != nil {
		t.Fatalf("Save: %v", err)
	}
	return cfgPath, workDir, cfg
}

// reload reads the config back from disk.
func reload(t *testing.T, cfgPath string) *config.Config {
	t.Helper()
	cfg, err := config.Load(cfgPath)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	return cfg
}

func TestUpdateChangesOnlyTheCommand(t *testing.T) {
	cfgPath, workDir, _ := updateFixture(t)

	stdout, stderr, code := run(t, "sleep 42\n\n", "update", "web", "--config", cfgPath)
	if code != int(errs.OK) {
		t.Fatalf("exit = %d, want 0 (stderr: %s)", code, stderr)
	}

	// Both questions print the value they are about to replace.
	for _, want := range []string{
		"Current command: " + idleCommand(),
		"Current working directory: " + filepath.ToSlash(workDir),
		"New command (empty to keep): ",
		"New working directory (empty to keep): ",
		"Service 'web' updated.\n",
	} {
		if !strings.Contains(stdout, want) {
			t.Errorf("stdout is missing %q:\n%s", want, stdout)
		}
	}

	svc, ok := reload(t, cfgPath).Get("web")
	if !ok {
		t.Fatal("the service disappeared")
	}
	if svc.Command != "sleep 42" {
		t.Errorf("command = %q, want the new one", svc.Command)
	}
	if svc.WorkDir != filepath.ToSlash(workDir) {
		t.Errorf("work_dir = %q, want it untouched", svc.WorkDir)
	}
}

func TestUpdateChangesOnlyTheWorkDir(t *testing.T) {
	cfgPath, _, _ := updateFixture(t)
	other := t.TempDir()

	stdout, stderr, code := run(t, "\n"+other+"\n", "update", "web", "--config", cfgPath)
	if code != int(errs.OK) {
		t.Fatalf("exit = %d, want 0 (stderr: %s)", code, stderr)
	}
	if !strings.Contains(stdout, "Service 'web' updated.\n") {
		t.Fatalf("stdout = %q", stdout)
	}

	svc, _ := reload(t, cfgPath).Get("web")
	if svc.WorkDir != filepath.ToSlash(other) {
		t.Errorf("work_dir = %q, want %q", svc.WorkDir, other)
	}
	if svc.Command != idleCommand() {
		t.Errorf("command = %q, want it untouched", svc.Command)
	}
}

func TestUpdateChangesBothFields(t *testing.T) {
	cfgPath, _, _ := updateFixture(t)
	other := t.TempDir()

	if _, stderr, code := run(t, "sleep 7\n"+other+"\n", "update", "web", "--config", cfgPath); code != int(errs.OK) {
		t.Fatalf("exit = %d, want 0 (stderr: %s)", code, stderr)
	}

	svc, _ := reload(t, cfgPath).Get("web")
	if svc.Command != "sleep 7" || svc.WorkDir != filepath.ToSlash(other) {
		t.Fatalf("service = %+v, want both fields updated", svc)
	}
}

// Two empty answers mean "change nothing": the config must not even be
// rewritten, so its bytes stay identical.
func TestUpdateWithNoAnswersKeepsEverything(t *testing.T) {
	cfgPath, _, _ := updateFixture(t)
	before, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatal(err)
	}

	stdout, stderr, code := run(t, "\n\n", "update", "web", "--config", cfgPath)
	if code != int(errs.OK) {
		t.Fatalf("exit = %d, want 0 (stderr: %s)", code, stderr)
	}
	if !strings.Contains(stdout, "Service 'web' unchanged.\n") {
		t.Fatalf("stdout = %q", stdout)
	}
	if strings.Contains(stdout, "updated") {
		t.Fatalf("nothing changed, so nothing may be reported as updated: %q", stdout)
	}

	after, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) {
		t.Fatalf("the config was rewritten:\n%s", after)
	}
}

func TestUpdateRejectsAMissingWorkDirAndAsksAgain(t *testing.T) {
	cfgPath, _, _ := updateFixture(t)
	other := t.TempDir()

	stdin := "\n" + filepath.Join(t.TempDir(), "gone") + "\n" + other + "\n"
	stdout, stderr, code := run(t, stdin, "update", "web", "--config", cfgPath)
	if code != int(errs.OK) {
		t.Fatalf("exit = %d, want 0 (stderr: %s)", code, stderr)
	}
	// One prompt for the rejected answer, one for the accepted one.
	if n := strings.Count(stdout, "New working directory (empty to keep): "); n != 2 {
		t.Fatalf("the directory question should have been asked twice, got %d:\n%s", n, stdout)
	}
	if !strings.Contains(stdout, "is not accessible") {
		t.Fatalf("the rejection reason must be shown:\n%s", stdout)
	}

	svc, _ := reload(t, cfgPath).Get("web")
	if svc.WorkDir != filepath.ToSlash(other) {
		t.Fatalf("work_dir = %q, want %q", svc.WorkDir, other)
	}
}

// A relative answer goes through the same resolution as `add`.
func TestUpdateResolvesARelativeWorkDir(t *testing.T) {
	cfgPath, _, _ := updateFixture(t)
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}

	if _, stderr, code := run(t, "\n.\n", "update", "web", "--config", cfgPath); code != int(errs.OK) {
		t.Fatalf("exit = %d, want 0 (stderr: %s)", code, stderr)
	}

	svc, _ := reload(t, cfgPath).Get("web")
	if svc.WorkDir != filepath.ToSlash(cwd) {
		t.Fatalf("work_dir = %q, want the resolved %q", svc.WorkDir, cwd)
	}
}

func TestUpdateNotesThatARunningServiceNeedsARestart(t *testing.T) {
	cfgPath, _, _ := updateFixture(t)
	t.Cleanup(func() { _, _, _ = cli(t, "stop", "web", "--config", cfgPath) })

	if _, stderr, code := cli(t, "start", "web", "--config", cfgPath); code != int(errs.OK) {
		t.Fatalf("start exit = %d (stderr: %s)", code, stderr)
	}

	stdout, stderr, code := run(t, "sleep 43\n\n", "update", "web", "--config", cfgPath)
	if code != int(errs.OK) {
		t.Fatalf("exit = %d, want 0 (stderr: %s)", code, stderr)
	}
	if !strings.Contains(stdout, "is running (pid ") || !strings.Contains(stdout, "next restart") {
		t.Fatalf("the restart note is missing:\n%s", stdout)
	}
	if svc, _ := reload(t, cfgPath).Get("web"); svc.Command != "sleep 43" {
		t.Fatalf("the config must be updated even while running: %+v", svc)
	}
}

// A stale PID file means the service is not really up, so no restart note.
func TestUpdateDoesNotNoteAStalePIDFile(t *testing.T) {
	cfgPath, workDir, _ := updateFixture(t)
	pidPath := config.PIDPath(cfgPath, "web")
	if err := os.MkdirAll(filepath.Dir(pidPath), 0o700); err != nil {
		t.Fatal(err)
	}
	stale := `{"pid": 999999, "started_at": 1, "work_dir": "` + filepath.ToSlash(workDir) + `"}`
	if err := os.WriteFile(pidPath, []byte(stale), 0o600); err != nil {
		t.Fatal(err)
	}

	stdout, stderr, code := run(t, "sleep 44\n\n", "update", "web", "--config", cfgPath)
	if code != int(errs.OK) {
		t.Fatalf("exit = %d, want 0 (stderr: %s)", code, stderr)
	}
	if strings.Contains(stdout, "next restart") {
		t.Fatalf("a stale PID file is not a running service:\n%s", stdout)
	}
}

func TestUpdateUnknownServiceExitsWithThree(t *testing.T) {
	cfgPath, _, _ := updateFixture(t)

	stdout, stderr, code := run(t, "\n\n", "update", "ghost", "--config", cfgPath)
	if code != int(errs.NotFound) {
		t.Fatalf("exit = %d, want %d", code, errs.NotFound)
	}
	if strings.Contains(stdout, "Current command:") {
		t.Fatalf("no question may be asked for an unknown service: %q", stdout)
	}
	if !strings.Contains(stderr, "service 'ghost' not found") {
		t.Fatalf("stderr = %q", stderr)
	}
}

func TestUpdateWithClosedInputFails(t *testing.T) {
	cfgPath, workDir, _ := updateFixture(t)

	_, stderr, code := run(t, "", "update", "web", "--config", cfgPath)
	if code != int(errs.General) {
		t.Fatalf("exit = %d, want %d", code, errs.General)
	}
	if !strings.Contains(stderr, "input ended") {
		t.Fatalf("stderr = %q", stderr)
	}

	svc, _ := reload(t, cfgPath).Get("web")
	if svc.Command != idleCommand() || svc.WorkDir != filepath.ToSlash(workDir) {
		t.Fatalf("an aborted update must change nothing, got %+v", svc)
	}
}

func TestUpdateRequiresExactlyOneServiceName(t *testing.T) {
	cfgPath, _, _ := updateFixture(t)

	for _, args := range [][]string{
		{"update", "--config", cfgPath},
		{"update", "a", "b", "--config", cfgPath},
	} {
		if _, stderr, code := run(t, "\n\n", args...); code != int(errs.Usage) {
			t.Errorf("%v exit = %d, want %d (stderr: %s)", args, code, errs.Usage, stderr)
		}
	}
}
