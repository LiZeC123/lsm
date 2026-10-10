package cmd

import (
	"strconv"

	"github.com/LiZeC123/lsm/internal/ui"
	"github.com/spf13/cobra"
)

func newPsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "ps",
		Short: "Show the runtime status of every configured service",
		Args:  argsNone(),
		RunE: func(cmd *cobra.Command, _ []string) error {
			s, err := newSession()
			if err != nil {
				return err
			}
			statuses, err := s.mgr.Status()
			if err != nil {
				return err
			}

			table := ui.NewTable(cmd.OutOrStdout(), "NAME", "STATUS", "PID", "UPTIME")
			for _, status := range statuses {
				if !status.Running {
					table.Row(status.Name, "stopped", "-", "-")
					continue
				}
				uptime := "-"
				if !status.StartedAt.IsZero() {
					uptime = ui.FormatUptime(status.Uptime)
				}
				table.Row(status.Name, "running", strconv.Itoa(status.PID), uptime)
			}
			return table.Flush()
		},
	}
}
