package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"android-toolbox/internal/buildinfo"
)

func newRootCmd() *cobra.Command {
	var appCtx *appContext

	root := &cobra.Command{
		Use:   "android-toolbox",
		Short: "Toolbox for controlling connected Android devices",
		Long: "android-toolbox bundles adb, scrcpy, and custom actions into an\n" +
			"interactive console. Without a subcommand, it starts the interactive TUI.",
		Version:       buildinfo.Version,
		SilenceUsage:  true,
		SilenceErrors: true,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			ac, err := newAppContext()
			if err != nil {
				return fmt.Errorf("initialization failed: %w", err)
			}
			appCtx = ac
			cmd.SetContext(withAppContext(cmd.Context(), ac))
			return nil
		},
		PersistentPostRunE: func(cmd *cobra.Command, args []string) error {
			if appCtx != nil {
				return appCtx.Log.Close()
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return runTUI(cmd.Context())
		},
	}

	root.AddCommand(newVersionCmd())
	root.AddCommand(newHealthcheckCmd())
	root.AddCommand(newDevicesCmd())
	root.AddCommand(newToolsCmd())
	root.AddCommand(newRunCmd())
	root.AddCommand(newAICmd())
	root.AddCommand(newBackupCmd())
	root.AddCommand(newRecoverCmd())
	root.AddCommand(newInstallCmd())
	root.AddCommand(newDangerousResetCmd())
	root.AddCommand(newSelfUpdateCmd())
	root.AddCommand(newAPKInfoCmd())
	root.AddCommand(newEmulatorCmd())

	return root
}

func main() {
	if err := newRootCmd().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
}
