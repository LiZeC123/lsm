package cmd

import (
	"github.com/LiZeC123/lsm/internal/ui"
	"github.com/spf13/cobra"
)

func newListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List every configured service",
		Args:  argsNone(),
		RunE: func(cmd *cobra.Command, _ []string) error {
			s, err := newSession()
			if err != nil {
				return err
			}

			table := ui.NewTable(cmd.OutOrStdout(), "NAME", "COMMAND", "WORK_DIR")
			for _, name := range s.cfg.Names() {
				svc := s.cfg.Services[name]
				table.Row(name, svc.Command, svc.WorkDir)
			}
			return table.Flush()
		},
	}
}
