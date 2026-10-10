package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LiZeC123/lsm/internal/config"
	"github.com/LiZeC123/lsm/internal/errs"
)

func TestDelConfirmRemovesTheService(t *testing.T) {
	cfgPath := serviceFixture(t, "web", idleCommand())

	stdout, stderr, code := run(t, "y\n", "del", "web", "--config", cfgPath)
	if code != int(errs.OK) {
		t.Fatalf("exit = %d, want 0 (stderr: %s)", code, stderr)
	}
	if !strings.Contains(stdout, "Delete service 'web'? [y/N]: ") {
		t.Errorf("the confirmation prompt is missing:\n%s", stdout)
	}
	if !strings.HasSuffix(stdout, "Service 'web' deleted.\n") {
		t.Errorf("stdout = %q", stdout)
	}

	cfg, err := config.Load(cfgPath)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if _, ok := cfg.Get("web"); ok {
		t.Fatal("the service must be gone from the config")
	}
}

func TestDelDeclinedKeepsTheService(t *testing.T) {
	for _, answer := range []string{"n\n", "\n", ""} {
		t.Run(strings.TrimSpace(answer), func(t *testing.T) {
			cfgPath := serviceFixture(t, "web", idleCommand())

			stdout, stderr, code := run(t, answer, "del", "web", "--config", cfgPath)
			if code != int(errs.OK) {
				t.Fatalf("exit = %d, want 0 (stderr: %s)", code, stderr)
			}
			if strings.Contains(stdout, "deleted") {
				t.Fatalf("nothing must be deleted, got %q", stdout)
			}

			cfg, err := config.Load(cfgPath)
			if err != nil {
				t.Fatalf("Load: %v", err)
			}
			if _, ok := cfg.Get("web"); !ok {
				t.Fatal("the service must be kept")
			}
		})
	}
}

func TestDelRefusesARunningServiceWithoutForce(t *testing.T) {
	cfgPath := serviceFixture(t, "web", idleCommand())
	t.Cleanup(func() { _, _, _ = cli(t, "stop", "web", "--config", cfgPath) })

	if _, stderr, code := cli(t, "start", "web", "--config", cfgPath); code != int(errs.OK) {
		t.Fatalf("start exit = %d (stderr: %s)", code, stderr)
	}

	stdout, stderr, code := run(t, "y\n", "del", "web", "--config", cfgPath)
	if code == int(errs.OK) {
		t.Fatalf("del must fail while the service runs (stdout: %s)", stdout)
	}
	if !strings.Contains(stderr, "is running (pid ") || !strings.Contains(stderr, "use --force") {
		t.Fatalf("stderr = %q", stderr)
	}
	if strings.Contains(stdout, "deleted") {
		t.Fatalf("nothing must be deleted, got %q", stdout)
	}

	cfg, err := config.Load(cfgPath)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if _, ok := cfg.Get("web"); !ok {
		t.Fatal("the service must be kept")
	}
}

func TestDelForceStopsTheServiceFirst(t *testing.T) {
	cfgPath := serviceFixture(t, "web", idleCommand())
	t.Cleanup(func() { _, _, _ = cli(t, "stop", "web", "--config", cfgPath) })

	if _, stderr, code := cli(t, "start", "web", "--config", cfgPath); code != int(errs.OK) {
		t.Fatalf("start exit = %d (stderr: %s)", code, stderr)
	}

	stdout, stderr, code := run(t, "y\n", "del", "web", "--force", "--config", cfgPath)
	if code != int(errs.OK) {
		t.Fatalf("exit = %d, want 0 (stderr: %s)", code, stderr)
	}
	if !strings.Contains(stdout, "Stopping service 'web' (pid ") {
		t.Errorf("the stopping notice is missing:\n%s", stdout)
	}
	if !strings.HasSuffix(stdout, "Service 'web' deleted.\n") {
		t.Errorf("stdout = %q", stdout)
	}

	if _, err := os.Stat(config.PIDPath(cfgPath, "web")); !os.IsNotExist(err) {
		t.Fatalf("the pid file must be removed as well, got %v", err)
	}
	psOut, _, _ := cli(t, "ps", "--config", cfgPath)
	if strings.Contains(psOut, "web") {
		t.Fatalf("the service must be gone from ps:\n%s", psOut)
	}
}

// Declining after --force leaves the service stopped but still configured, which
// is exactly the transcript in dev.md §5.8.
func TestDelForceDeclinedLeavesTheServiceStopped(t *testing.T) {
	cfgPath := serviceFixture(t, "web", idleCommand())

	if _, stderr, code := cli(t, "start", "web", "--config", cfgPath); code != int(errs.OK) {
		t.Fatalf("start exit = %d (stderr: %s)", code, stderr)
	}

	stdout, stderr, code := run(t, "n\n", "del", "web", "--force", "--config", cfgPath)
	if code != int(errs.OK) {
		t.Fatalf("exit = %d, want 0 (stderr: %s)", code, stderr)
	}
	if !strings.Contains(stdout, "Stopping service 'web' (pid ") {
		t.Errorf("the stopping notice is missing:\n%s", stdout)
	}

	cfg, err := config.Load(cfgPath)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if _, ok := cfg.Get("web"); !ok {
		t.Fatal("the service must be kept")
	}
	psOut, _, _ := cli(t, "ps", "--config", cfgPath)
	if !strings.Contains(psOut, "stopped") {
		t.Fatalf("the service must be stopped:\n%s", psOut)
	}
}

func TestDelRemovesAStalePIDFile(t *testing.T) {
	cfgPath := serviceFixture(t, "web", idleCommand())
	pidPath := config.PIDPath(cfgPath, "web")
	if err := os.MkdirAll(filepath.Dir(pidPath), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(pidPath, []byte(`{"pid": 999999}`), 0o600); err != nil {
		t.Fatal(err)
	}

	if _, stderr, code := run(t, "y\n", "del", "web", "--config", cfgPath); code != int(errs.OK) {
		t.Fatalf("exit = %d, want 0 (stderr: %s)", code, stderr)
	}
	if _, err := os.Stat(pidPath); !os.IsNotExist(err) {
		t.Fatalf("the stale pid file must be removed, got %v", err)
	}
}

func TestDelUnknownServiceExitsWithThree(t *testing.T) {
	cfgPath := serviceFixture(t, "web", idleCommand())

	_, stderr, code := run(t, "", "del", "ghost", "--config", cfgPath)
	if code != int(errs.NotFound) {
		t.Fatalf("exit = %d, want %d", code, errs.NotFound)
	}
	if !strings.Contains(stderr, "service 'ghost' not found") {
		t.Fatalf("stderr = %q", stderr)
	}
}

func TestDelRequiresExactlyOneServiceName(t *testing.T) {
	cfgPath := serviceFixture(t, "web", idleCommand())

	for _, args := range [][]string{
		{"del", "--config", cfgPath},
		{"del", "a", "b", "--config", cfgPath},
	} {
		if _, stderr, code := run(t, "", args...); code != int(errs.Usage) {
			t.Errorf("%v exit = %d, want %d (stderr: %s)", args, code, errs.Usage, stderr)
		}
	}
}
