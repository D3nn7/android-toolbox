package main

import (
	"fmt"

	"github.com/spf13/cobra"

	"android-toolbox/internal/backup"
)

func newBackupCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "backup",
		Short: "Manually creates a backup of the configuration directory",
		RunE: func(cmd *cobra.Command, args []string) error {
			ac := appContextFrom(cmd.Context())
			out := cmd.OutOrStdout()

			files := []string{ac.Paths.ActionsFile, ac.Paths.SettingsFile}
			count := 0
			for _, f := range files {
				if err := backup.Snapshot(ac.Paths.BackupDir, f); err != nil {
					return fmt.Errorf("backup of %s failed: %w", f, err)
				}
				count++
			}
			fmt.Fprintf(out, "Backup created in %s (%d file(s) saved).\n", ac.Paths.BackupDir, count)
			return nil
		},
	}
}
