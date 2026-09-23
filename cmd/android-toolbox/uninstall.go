package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"android-toolbox/internal/config"
	"android-toolbox/internal/install"
)

// newUninstallCmd reverses `install`: it removes the PATH
// symlinks/registry entry and shell rc PATH line install created, then
// deletes the whole android-toolbox configuration directory (settings,
// actions, backups, logs, and the downloaded adb/scrcpy/SDK copies).
//
// It deliberately never touches a real Android SDK (ANDROID_HOME/
// ANDROID_SDK_ROOT) or ~/.android/avd - those belong to the system's
// Android tooling, not to android-toolbox, and it never wrote to them in
// the first place (see internal/avd's AvdHome).
//
// PersistentPreRunE is overridden for the same reason as dangerous-reset's:
// this must still work against a corrupted settings.yaml/state.json.
func newUninstallCmd() *cobra.Command {
	var yes bool

	cmd := &cobra.Command{
		Use:   "uninstall",
		Short: "Removes android-toolbox from this machine",
		Long: "Removes the PATH entry `install` created and deletes the entire\n" +
			"android-toolbox configuration directory - custom actions, settings,\n" +
			"saved state, backups, logs, and the downloaded adb/scrcpy copies.\n" +
			"A real Android SDK (ANDROID_HOME/ANDROID_SDK_ROOT) and ~/.android/avd\n" +
			"are never touched - android-toolbox doesn't own them. The\n" +
			"android-toolbox binary itself is not deleted; remove it manually.",
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error { return nil },
		RunE: func(cmd *cobra.Command, args []string) error {
			out := cmd.OutOrStdout()

			paths, err := config.Resolve()
			if err != nil {
				return fmt.Errorf("could not determine configuration paths: %w", err)
			}

			alias := "atbx"
			if s, err := config.LoadSettings(paths); err == nil && s.Install.AliasName != "" {
				alias = s.Install.AliasName
			}

			if !yes {
				fmt.Fprintf(out, "This will remove android-toolbox from PATH and irreversibly delete %s\n", paths.ConfigDir)
				fmt.Fprint(out, "(custom actions, settings, backups, downloaded tools). Continue? [y/N] ")
				if !confirmYesNo(cmd.InOrStdin()) {
					fmt.Fprintln(out, "Cancelled.")
					return nil
				}
			}

			res, err := install.Uninstall(appName, alias)
			if err != nil {
				return fmt.Errorf("could not remove PATH entry: %w", err)
			}
			for _, f := range res.RemovedFiles {
				fmt.Fprintf(out, "Removed %s\n", f)
			}
			if res.Note != "" {
				fmt.Fprintln(out, res.Note)
			}

			fmt.Fprintf(out, "Deleting %s ...\n", paths.ConfigDir)
			if err := os.RemoveAll(paths.ConfigDir); err != nil {
				return fmt.Errorf("could not delete configuration directory: %w", err)
			}

			fmt.Fprintln(out, "Uninstall complete. The android-toolbox binary itself was left in place - delete it manually.")
			return nil
		},
	}

	cmd.Flags().BoolVar(&yes, "yes", false, "Run without confirmation")
	return cmd
}
