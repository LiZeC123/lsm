package errs

import (
	"errors"
	"fmt"
	"testing"
)

func TestCodeOf(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want Code
	}{
		{"nil", nil, OK},
		{"plain error", errors.New("boom"), General},
		{"new", New(NotFound, "service '%s' not found", "web"), NotFound},
		{"classify", Classify(Usage, errors.New("unknown flag: --bogus")), Usage},
		{"wrapped by fmt", fmt.Errorf("service 'web': %w", New(ConfigRead, "bad")), ConfigRead},
		{"wrapped by errs", Wrap(ProcessOp, errors.New("exit"), "launch failed"), ProcessOp},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := CodeOf(tc.err); got != tc.want {
				t.Fatalf("CodeOf = %d, want %d", got, tc.want)
			}
		})
	}
}

func TestWrapKeepsTheCauseVisible(t *testing.T) {
	err := Wrap(ConfigRead, errors.New("no such file"), "cannot read config '%s'", "/tmp/x")
	if got, want := err.Error(), "cannot read config '/tmp/x': no such file"; got != want {
		t.Fatalf("Error() = %q, want %q", got, want)
	}
	if !errors.Is(err, err.Err) {
		t.Fatal("wrapped cause must stay reachable through errors.Is")
	}
}

func TestClassifyOfNilIsNil(t *testing.T) {
	if got := Classify(Usage, nil); got != nil {
		t.Fatalf("Classify(nil) = %v, want nil", got)
	}
}

func TestNewUsesTheFormattedMessage(t *testing.T) {
	err := New(AlreadyRunning, "service '%s' is already running (pid %d)", "web", 42)
	if got, want := err.Error(), "service 'web' is already running (pid 42)"; got != want {
		t.Fatalf("Error() = %q, want %q", got, want)
	}
}
