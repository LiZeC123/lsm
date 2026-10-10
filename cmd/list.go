package cmd

import (
	"github.com/LiZeC123/lsm/internal/config"
	"github.com/LiZeC123/lsm/internal/ui"
	"github.com/spf13/cobra"
)

func newListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List every configured service",
		Args:  argsNone(),
		RunE: func(cmd *cobra.Command, _ []string) error {
			path, err := resolveConfigPath()
			if err != nil {
				return err
			}
			cfg, err := config.Load(path)
			if err != nil {
				return err
			}

			table := ui.NewTable(cmd.OutOrStdout(), "NAME", "COMMAND", "WORK_DIR")
			for _, name := range cfg.Names() {
				svc := cfg.Services[name]
				table.Row(name, svc.Command, svc.WorkDir)
			}
			return table.Flush()
		},
	}
}
