package cmd

import (
	"strings"
	"testing"

	"github.com/LiZeC123/lsm/internal/errs"
)

// psRow returns the fields of the row whose first column is name.
func psRow(t *testing.T, stdout, name string) []string {
	t.Helper()
	for _, line := range strings.Split(strings.TrimSuffix(stdout, "\n"), "\n") {
		fields := strings.Fields(line)
		if len(fields) > 0 && fields[0] == name {
			return fields
		}
	}
	t.Fatalf("no ps row for %q in:\n%s", name, stdout)
	return nil
}

func TestPsShowsAStoppedService(t *testing.T) {
	cfgPath := serviceFixture(t, "web", idleCommand())

	stdout, stderr, code := cli(t, "ps", "--config", cfgPath)
	if code != int(errs.OK) {
		t.Fatalf("exit = %d, want 0 (stderr: %s)", code, stderr)
	}
	if !strings.HasPrefix(stdout, "NAME") {
		t.Fatalf("first line is not the header: %q", stdout)
	}
	row := psRow(t, stdout, "web")
	want := []string{"web", "stopped", "-", "-"}
	if strings.Join(row, " ") != strings.Join(want, " ") {
		t.Fatalf("row = %v, want %v", row, want)
	}
}

func TestPsShowsARunningService(t *testing.T) {
	cfgPath := serviceFixture(t, "web", idleCommand())
	t.Cleanup(func() { _, _, _ = cli(t, "stop", "web", "--config", cfgPath) })

	if _, stderr, code := cli(t, "start", "web", "--config", cfgPath); code != int(errs.OK) {
		t.Fatalf("start exit = %d (stderr: %s)", code, stderr)
	}

	stdout, stderr, code := cli(t, "ps", "--config", cfgPath)
	if code != int(errs.OK) {
		t.Fatalf("exit = %d, want 0 (stderr: %s)", code, stderr)
	}
	row := psRow(t, stdout, "web")
	if row[1] != "running" {
		t.Fatalf("status = %q, want running", row[1])
	}
	if row[2] == "-" || row[2] == "" {
		t.Fatalf("a running service needs a pid, got %q", row[2])
	}
	if !strings.HasSuffix(row[3], "s") && !strings.HasSuffix(row[3], "m") {
		t.Fatalf("unexpected uptime %q", row[3])
	}
}

func TestRestartStartsAServiceThatIsNotRunning(t *testing.T) {
	cfgPath := serviceFixture(t, "web", idleCommand())
	t.Cleanup(func() { _, _, _ = cli(t, "stop", "web", "--config", cfgPath) })

	stdout, stderr, code := cli(t, "restart", "web", "--config", cfgPath)
	if code != int(errs.OK) {
		t.Fatalf("exit = %d, want 0 (stderr: %s)", code, stderr)
	}
	if !strings.HasPrefix(stdout, "service 'web' started (pid ") {
		t.Fatalf("stdout = %q", stdout)
	}
}

func TestRestartReplacesARunningService(t *testing.T) {
	cfgPath := serviceFixture(t, "web", idleCommand())
	t.Cleanup(func() { _, _, _ = cli(t, "stop", "web", "--config", cfgPath) })

	if _, stderr, code := cli(t, "start", "web", "--config", cfgPath); code != int(errs.OK) {
		t.Fatalf("start exit = %d (stderr: %s)", code, stderr)
	}

	stdout, stderr, code := cli(t, "restart", "web", "--config", cfgPath)
	if code != int(errs.OK) {
		t.Fatalf("exit = %d, want 0 (stderr: %s)", code, stderr)
	}
	if !strings.HasPrefix(stdout, "service 'web' restarted (pid ") {
		t.Fatalf("stdout = %q", stdout)
	}

	// The service must still be reported as running afterwards.
	psOut, _, code := cli(t, "ps", "--config", cfgPath)
	if code != int(errs.OK) {
		t.Fatalf("ps exit = %d", code)
	}
	if got := psRow(t, psOut, "web")[1]; got != "running" {
		t.Fatalf("status after restart = %q, want running", got)
	}
}

func TestRestartUnknownServiceExitsWithThree(t *testing.T) {
	cfgPath := serviceFixture(t, "web", idleCommand())

	_, stderr, code := cli(t, "restart", "ghost", "--config", cfgPath)
	if code != int(errs.NotFound) {
		t.Fatalf("exit = %d, want %d", code, errs.NotFound)
	}
	if !strings.Contains(stderr, "service 'ghost' not found") {
		t.Fatalf("stderr = %q", stderr)
	}
}

func TestPsAndRestartRejectStrayArguments(t *testing.T) {
	cfgPath := serviceFixture(t, "web", idleCommand())

	for _, args := range [][]string{
		{"ps", "extra", "--config", cfgPath},
		{"restart", "--config", cfgPath},
		{"restart", "a", "b", "--config", cfgPath},
	} {
		if _, stderr, code := cli(t, args...); code != int(errs.Usage) {
			t.Errorf("%v exit = %d, want %d (stderr: %s)", args, code, errs.Usage, stderr)
		}
	}
}
