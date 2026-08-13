package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/spf13/cobra"

	"android-toolbox/internal/actions"
	"android-toolbox/internal/config"
	"android-toolbox/internal/toolsmanager"
)

// newDangerousResetCmd wipes android-toolbox's own config directory (custom
// actions, settings, saved state, backups, logs, the downloaded adb/scrcpy
// copies) and re-seeds everything from scratch, as if freshly installed.
// Nothing outside that directory is touched - a system-wide adb install
// elsewhere, or any other tool on the machine, is unaffected.
//
// PersistentPreRunE is deliberately overridden (not left to inherit the
// root command's) rather than going through the usual newAppContext(): that
// loads and parses the very settings.yaml/state.json this command exists to
// recover from, so a corrupted one would make dangerous-reset itself fail
// before it ever got a chance to fix anything.
func newDangerousResetCmd() *cobra.Command {
	var yes bool

	cmd := &cobra.Command{
		Use:   "dangerous-reset",
		Short: "Deletes all local android-toolbox data and reinstalls everything",
		Long: "Deletes the entire android-toolbox configuration directory - custom\n" +
			"actions, settings, saved state, backups, logs, and the downloaded\n" +
			"adb/scrcpy copies - and then sets everything back up: default actions,\n" +
			"default settings, and a fresh download of adb/scrcpy. Other tools on\n" +
			"this system (e.g. a system-wide adb install) are left untouched.",
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error { return nil },
		RunE: func(cmd *cobra.Command, args []string) error {
			out := cmd.OutOrStdout()

			paths, err := config.Resolve()
			if err != nil {
				return fmt.Errorf("could not determine configuration paths: %w", err)
			}

			if !yes {
				fmt.Fprintf(out, "This will irreversibly delete %s\n", paths.ConfigDir)
				fmt.Fprint(out, "(custom actions, settings, backups, downloaded tools). Continue? [y/N] ")
				if !confirmYesNo(cmd.InOrStdin()) {
					fmt.Fprintln(out, "Cancelled.")
					return nil
				}
			}

			return runDangerousReset(cmd.Context(), out, paths, fetchToolsForReset)
		},
	}

	cmd.Flags().BoolVar(&yes, "yes", false, "Run without confirmation")
	return cmd
}

// confirmYesNo reads a single line from in and reports whether it
// was an affirmative "y"/"Y".
func confirmYesNo(in io.Reader) bool {
	reader := bufio.NewReader(in)
	line, _ := reader.ReadString('\n')
	return strings.TrimSpace(strings.ToLower(line)) == "y"
}

func fetchToolsForReset(ctx context.Context, toolsDir string, progress func(string)) error {
	return toolsmanager.New(toolsDir).FetchAll(ctx, runtime.GOOS, runtime.GOARCH, progress)
}

// ensureConfigDirs recreates the directory skeleton for an already-resolved
// Paths value - the same directories config.Resolve() itself creates, but
// usable after paths.ConfigDir has just been removed without needing to
// call Resolve() again (which always re-derives the real OS user-config
// directory, so it can't be pointed at a test's temp directory instead).
func ensureConfigDirs(p config.Paths) error {
	for _, dir := range []string{p.ConfigDir, p.BackupDir, p.ToolsDir, p.LogsDir, filepath.Dir(p.AIPromptFile)} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	return nil
}

// runDangerousReset does the actual wipe-and-reseed for paths.ConfigDir:
// remove it entirely, recreate the directory skeleton, reseed the default
// settings/actions files, then re-fetch adb/scrcpy via fetch (injected so
// tests can substitute a fake instead of a real network download).
func runDangerousReset(ctx context.Context, out io.Writer, paths config.Paths, fetch func(ctx context.Context, toolsDir string, progress func(string)) error) error {
	fmt.Fprintf(out, "Deleting %s ...\n", paths.ConfigDir)
	if err := os.RemoveAll(paths.ConfigDir); err != nil {
		return fmt.Errorf("could not delete configuration directory: %w", err)
	}

	if err := ensureConfigDirs(paths); err != nil {
		return fmt.Errorf("could not recreate configuration directory: %w", err)
	}
	if _, err := config.LoadSettings(paths); err != nil {
		return fmt.Errorf("could not create default settings: %w", err)
	}
	if _, err := actions.Load(paths.ActionsFile, actions.DefaultActionsYAML); err != nil {
		return fmt.Errorf("could not create default actions: %w", err)
	}

	fmt.Fprintln(out, "Re-downloading adb and scrcpy ...")
	if err := fetch(ctx, paths.ToolsDir, func(msg string) { fmt.Fprintln(out, msg) }); err != nil {
		return fmt.Errorf("reinstalling tools failed: %w", err)
	}

	fmt.Fprintln(out, "Reset complete.")
	return nil
}
