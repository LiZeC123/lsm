package process

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"time"

	"github.com/LiZeC123/lsm/internal/errs"
	"github.com/LiZeC123/lsm/internal/fsx"
)

// PIDInfo is the payload of a <name>.pid file (dev.md §7.1).
type PIDInfo struct {
	PID       int    `json:"pid"`
	StartedAt int64  `json:"started_at"`
	WorkDir   string `json:"work_dir"`
	Command   string `json:"command"`
}

const (
	// runDirMode protects the run directory, which only the owning user needs
	// to reach (dev.md §4.3).
	runDirMode fs.FileMode = 0o700
	// pidFileMode keeps the recorded command line private.
	pidFileMode fs.FileMode = 0o600
)

// ReadPIDFile decodes a PID file. A missing file yields (nil, nil): absence of
// state is not an error, it just means "nothing recorded yet" (dev.md §7.3).
func ReadPIDFile(path string) (*PIDInfo, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, errs.Wrap(errs.ProcessOp, err, "cannot read '%s'", path)
	}

	info := &PIDInfo{}
	if err := json.Unmarshal(data, info); err != nil {
		return nil, errs.Wrap(errs.ProcessOp, err, "cannot parse '%s'", path)
	}
	if info.PID <= 0 {
		return nil, errs.New(errs.ProcessOp, "cannot parse '%s': invalid pid %d", path, info.PID)
	}
	return info, nil
}

// WritePIDFile records info, creating the run directory on demand.
func WritePIDFile(path string, info PIDInfo) error {
	data, err := json.MarshalIndent(info, "", "  ")
	if err != nil {
		return errs.Wrap(errs.ProcessOp, err, "cannot encode pid file '%s'", path)
	}
	data = append(data, '\n')

	if err := fsx.WriteFileAtomic(path, data, pidFileMode, runDirMode); err != nil {
		return errs.Classify(errs.ProcessOp, err)
	}
	return nil
}

// RemovePIDFile deletes a PID file. A missing file is not an error.
func RemovePIDFile(path string) error {
	if err := os.Remove(path); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return errs.Wrap(errs.ProcessOp, err, "cannot remove '%s'", path)
	}
	return nil
}

// Uptime reports how long the process has been running at now, or 0 when the
// start time was never recorded.
func (p *PIDInfo) Uptime(now time.Time) time.Duration {
	if p == nil || p.StartedAt <= 0 {
		return 0
	}
	d := now.Sub(time.Unix(p.StartedAt, 0))
	if d < 0 {
		return 0
	}
	return d
}
