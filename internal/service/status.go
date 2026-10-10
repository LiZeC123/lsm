package service

import (
	"time"

	"github.com/LiZeC123/lsm/internal/process"
)

// Status describes one configured service at a point in time.
type Status struct {
	Name    string
	Command string
	WorkDir string

	// Running reports whether the recorded process still exists.
	Running bool
	// PID is the recorded process id; only meaningful while Running.
	PID int
	// StartedAt is the recorded start time; zero when the service never
	// started or the PID file carried no timestamp.
	StartedAt time.Time
	// Uptime is how long the process has been running.
	Uptime time.Duration
	// StalePID reports that a PID file exists but its process is gone
	// (dev.md §7.3). Such a file is ignored by start and removed by stop.
	StalePID bool
}

// Status reports the runtime state of every configured service, in sorted
// order. It only observes: a stale PID file is reported, never removed, because
// ps must not modify anything (dev.md §5.5).
func (m *Manager) Status() ([]Status, error) {
	now := time.Now()
	names := m.cfg.Names()
	statuses := make([]Status, 0, len(names))

	for _, name := range names {
		svc := m.cfg.Services[name]
		status := Status{Name: name, Command: svc.Command, WorkDir: svc.WorkDir}

		info, err := process.ReadPIDFile(m.pidPath(name))
		if err != nil {
			return nil, err
		}
		switch {
		case info == nil:
			// never started through lsm
		case process.IsAlive(*info):
			status.Running = true
			status.PID = info.PID
			status.Uptime = info.Uptime(now)
			if info.StartedAt > 0 {
				status.StartedAt = time.Unix(info.StartedAt, 0)
			}
		default:
			status.StalePID = true
		}
		statuses = append(statuses, status)
	}
	return statuses, nil
}
