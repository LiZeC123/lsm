package ui

import (
	"testing"
	"time"
)

// The first four cases are the exact renderings shown in dev.md §5.5.
func TestFormatUptime(t *testing.T) {
	tests := []struct {
		in   time.Duration
		want string
	}{
		{0, "0s"},
		{-time.Minute, "0s"},
		{45 * time.Second, "45s"},
		{59 * time.Second, "59s"},
		{time.Minute, "1m"},
		{8 * time.Minute, "8m"},
		{481 * time.Second, "8m"},
		{2*time.Hour + 13*time.Minute, "2h13m"},
		{2*time.Hour + 13*time.Minute + 59*time.Second, "2h13m"},
		{time.Hour, "1h0m"},
		{24 * time.Hour, "1d0h"},
		{3*24*time.Hour + 5*time.Hour, "3d5h"},
		{48*time.Hour + 30*time.Minute, "2d0h"},
	}
	for _, tc := range tests {
		if got := FormatUptime(tc.in); got != tc.want {
			t.Errorf("FormatUptime(%v) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
