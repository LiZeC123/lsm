package process

import (
	"bytes"
	"strings"
	"testing"

	"github.com/LiZeC123/lsm/internal/config"
)

// Run must behave like an ordinary foreground execution: the command's streams
// are passed through and its exit status is reported without an error.
func TestRunPassesStreamsThrough(t *testing.T) {
	svc := config.Service{Command: "echo run-output", WorkDir: t.TempDir()}

	var stdout, stderr bytes.Buffer
	status, err := Run(svc, strings.NewReader(""), &stdout, &stderr)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if status != 0 {
		t.Fatalf("status = %d, want 0 (stderr: %s)", status, stderr.String())
	}
	if !strings.Contains(stdout.String(), "run-output") {
		t.Fatalf("stdout = %q, want the command output", stdout.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}

func TestRunReturnsTheCommandExitStatus(t *testing.T) {
	svc := config.Service{Command: "exit 9", WorkDir: t.TempDir()}

	status, err := Run(svc, strings.NewReader(""), &bytes.Buffer{}, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("a finished command is not an error: %v", err)
	}
	if status != 9 {
		t.Fatalf("status = %d, want 9", status)
	}
}
