package cmd

import (
	"strings"
	"testing"

	"github.com/LiZeC123/lsm/internal/errs"
	"github.com/LiZeC123/lsm/internal/version"
)

func TestVersionCommandPrintsTheBuildStamp(t *testing.T) {
	stdout, stderr, code := cli(t, "version")
	if code != int(errs.OK) {
		t.Fatalf("exit = %d, want 0 (stderr: %s)", code, stderr)
	}
	if got := strings.TrimSpace(stdout); got != version.String() {
		t.Fatalf("stdout = %q, want %q", got, version.String())
	}
}

// `lsm --version` and `lsm version` must agree.
func TestVersionFlagMatchesTheCommand(t *testing.T) {
	fromCmd, _, cmdCode := cli(t, "version")
	fromFlag, stderr, flagCode := cli(t, "--version")

	if cmdCode != int(errs.OK) || flagCode != int(errs.OK) {
		t.Fatalf("exits = %d and %d, want 0 (stderr: %s)", cmdCode, flagCode, stderr)
	}
	if fromFlag != fromCmd {
		t.Fatalf("--version = %q, version = %q", fromFlag, fromCmd)
	}
}

func TestVersionRejectsArguments(t *testing.T) {
	if _, stderr, code := cli(t, "version", "extra"); code != int(errs.Usage) {
		t.Fatalf("exit = %d, want %d (stderr: %s)", code, errs.Usage, stderr)
	}
}
