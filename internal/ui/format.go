package ui

import (
	"fmt"
	"time"
)

// FormatUptime renders a service uptime the way the ps table shows it
// (dev.md §5.5): seconds below a minute, then minutes, then hours and minutes,
// then days and hours. Durations are truncated, never rounded up.
func FormatUptime(d time.Duration) string {
	if d <= 0 {
		return "0s"
	}
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}

	d = d.Truncate(time.Minute)
	days := int(d.Hours()) / 24
	hours := int(d.Hours()) % 24
	minutes := int(d.Minutes()) % 60

	switch {
	case days > 0:
		return fmt.Sprintf("%dd%dh", days, hours)
	case hours > 0:
		return fmt.Sprintf("%dh%dm", hours, minutes)
	default:
		return fmt.Sprintf("%dm", minutes)
	}
}
