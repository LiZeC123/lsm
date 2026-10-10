package version

import "testing"

// snapshot saves and restores the link-time variables so the tests can stand in
// for the release pipeline.
func snapshot(t *testing.T) {
	t.Helper()
	version, commit, date := Version, Commit, Date
	t.Cleanup(func() { Version, Commit, Date = version, commit, date })
}

func TestStringWithoutInjectedMetadata(t *testing.T) {
	snapshot(t)
	Version, Commit, Date = "dev", "", ""

	if got, want := String(), "lsm dev"; got != want {
		t.Fatalf("String() = %q, want %q", got, want)
	}
}

func TestStringWithFullMetadata(t *testing.T) {
	snapshot(t)
	Version, Commit, Date = "v0.1.0", "abc1234", "2026-10-10T10:00:00Z"

	want := "lsm v0.1.0 (commit abc1234, built 2026-10-10T10:00:00Z)"
	if got := String(); got != want {
		t.Fatalf("String() = %q, want %q", got, want)
	}
}

func TestStringOmitsMissingParts(t *testing.T) {
	tests := []struct {
		name                  string
		version, commit, date string
		want                  string
	}{
		{"only a commit", "v0.1.0", "abc1234", "", "lsm v0.1.0 (commit abc1234)"},
		{"only a date", "v0.1.0", "", "2026-10-10T10:00:00Z", "lsm v0.1.0 (built 2026-10-10T10:00:00Z)"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			snapshot(t)
			Version, Commit, Date = tc.version, tc.commit, tc.date
			if got := String(); got != tc.want {
				t.Fatalf("String() = %q, want %q", got, tc.want)
			}
		})
	}
}
