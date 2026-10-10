package process

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/LiZeC123/lsm/internal/errs"
)

func TestPIDFileRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "run", "web.pid")

	want := PIDInfo{
		PID:       4242,
		StartedAt: 1712345678,
		WorkDir:   "/home/user/web",
		Command:   `node ./server.js --bind "127.0.0.1:8080"`,
	}
	if err := WritePIDFile(path, want); err != nil {
		t.Fatalf("WritePIDFile: %v", err)
	}

	got, err := ReadPIDFile(path)
	if err != nil {
		t.Fatalf("ReadPIDFile: %v", err)
	}
	if got == nil {
		t.Fatal("ReadPIDFile returned no info")
	}
	if *got != want {
		t.Fatalf("round trip = %+v, want %+v", *got, want)
	}
}

func TestWritePIDFileCreatesAPrivateRunDirectory(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "run", "web.pid")
	if err := WritePIDFile(path, PIDInfo{PID: 1}); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(path); err != nil {
		t.Fatalf("the pid file must exist: %v", err)
	}

	// Windows only tracks a read-only flag, so the Unix permission bits cannot
	// be asserted there.
	if runtime.GOOS == "windows" {
		return
	}
	dirInfo, err := os.Stat(filepath.Dir(path))
	if err != nil {
		t.Fatal(err)
	}
	if got := dirInfo.Mode().Perm(); got != 0o700 {
		t.Errorf("run dir mode = %o, want 700", got)
	}
	fileInfo, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := fileInfo.Mode().Perm(); got != 0o600 {
		t.Errorf("pid file mode = %o, want 600", got)
	}
}

func TestReadPIDFileMissingIsNotAnError(t *testing.T) {
	info, err := ReadPIDFile(filepath.Join(t.TempDir(), "gone.pid"))
	if err != nil {
		t.Fatalf("ReadPIDFile: %v", err)
	}
	if info != nil {
		t.Fatalf("want nil info, got %+v", *info)
	}
}

func TestReadPIDFileRejectsBrokenContent(t *testing.T) {
	dir := t.TempDir()

	tests := []struct {
		name    string
		content string
		wantMsg string
	}{
		{"invalid json", "not json at all", "cannot parse"},
		{"zero pid", `{"pid": 0}`, "invalid pid"},
		{"negative pid", `{"pid": -3}`, "invalid pid"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(dir, "broken.pid")
			if err := os.WriteFile(path, []byte(tc.content), 0o600); err != nil {
				t.Fatal(err)
			}
			_, err := ReadPIDFile(path)
			if err == nil {
				t.Fatal("want an error")
			}
			if got := errs.CodeOf(err); got != errs.ProcessOp {
				t.Errorf("CodeOf = %d, want %d", got, errs.ProcessOp)
			}
			if !strings.Contains(err.Error(), tc.wantMsg) {
				t.Errorf("error %q does not contain %q", err, tc.wantMsg)
			}
		})
	}
}

func TestRemovePIDFileToleratesAMissingFile(t *testing.T) {
	if err := RemovePIDFile(filepath.Join(t.TempDir(), "gone.pid")); err != nil {
		t.Fatalf("RemovePIDFile: %v", err)
	}
}

func TestUptime(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)

	if got := (&PIDInfo{StartedAt: now.Add(-90 * time.Minute).Unix()}).Uptime(now); got != 90*time.Minute {
		t.Errorf("Uptime = %v, want 90m", got)
	}
	// A clock that jumped backwards must not produce a negative uptime.
	if got := (&PIDInfo{StartedAt: now.Add(time.Hour).Unix()}).Uptime(now); got != 0 {
		t.Errorf("Uptime = %v, want 0", got)
	}
	if got := (*PIDInfo)(nil).Uptime(now); got != 0 {
		t.Errorf("Uptime on nil = %v, want 0", got)
	}
	if got := (&PIDInfo{}).Uptime(now); got != 0 {
		t.Errorf("Uptime without a start time = %v, want 0", got)
	}
}
