package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"android-toolbox/internal/config"
	"android-toolbox/internal/install"
)

const appName = "android-toolbox"

func newInstallCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "install",
		Short: "Installs android-toolbox system-wide (PATH)",
		RunE: func(cmd *cobra.Command, args []string) error {
			ac := appContextFrom(cmd.Context())
			out := cmd.OutOrStdout()

			exePath, err := os.Executable()
			if err != nil {
				return fmt.Errorf("could not determine own path: %w", err)
			}

			alias := ac.Settings.Install.AliasName
			if alias == "" {
				alias = "atbx"
			}

			res, err := install.Install(exePath, appName, alias)
			if err != nil {
				return err
			}

			fmt.Fprintf(out, "Installed to %s\n", res.InstallDir)
			for _, f := range res.InstalledFiles {
				fmt.Fprintf(out, "  - %s\n", f)
			}
			if res.Note != "" {
				fmt.Fprintln(out, res.Note)
			}

			newState, err := config.MarkFirstRunComplete(ac.Paths, ac.State)
			if err == nil {
				ac.State = newState
			}
			return nil
		},
	}
}
