package main

import (
	"fmt"
	"runtime"

	"github.com/spf13/cobra"

	"android-toolbox/internal/toolsmanager"
)

func newToolsCmd() *cobra.Command {
	toolsCmd := &cobra.Command{
		Use:   "tools",
		Short: "Manage the portable adb/scrcpy tools",
	}

	toolsCmd.AddCommand(newToolsFetchCmd())
	toolsCmd.AddCommand(newToolsStatusCmd())
	toolsCmd.AddCommand(newToolsCheckCmd())
	toolsCmd.AddCommand(newToolsUpdateCmd())

	return toolsCmd
}

func newToolsFetchCmd() *cobra.Command {
	var targetOS, targetArch string

	cmd := &cobra.Command{
		Use:   "fetch",
		Short: "Downloads adb and scrcpy for the given (default: current) system",
		RunE: func(cmd *cobra.Command, args []string) error {
			ac := appContextFrom(cmd.Context())
			mgr := toolsmanager.New(ac.Paths.ToolsDir)

			out := cmd.OutOrStdout()
			progress := func(msg string) { fmt.Fprintln(out, msg) }

			if err := mgr.FetchAll(cmd.Context(), targetOS, targetArch, progress); err != nil {
				return err
			}
			fmt.Fprintln(out, "Done.")
			return nil
		},
	}
	cmd.Flags().StringVar(&targetOS, "os", runtime.GOOS, "Target operating system (windows, linux, darwin)")
	cmd.Flags().StringVar(&targetArch, "arch", runtime.GOARCH, "Target architecture (amd64, arm64, 386)")
	return cmd
}

func newToolsStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Shows which adb/scrcpy binaries are currently in use",
		RunE: func(cmd *cobra.Command, args []string) error {
			ac := appContextFrom(cmd.Context())
			mgr := toolsmanager.New(ac.Paths.ToolsDir)
			out := cmd.OutOrStdout()

			adb, adbErr := mgr.ResolveADB()
			if adbErr != nil {
				fmt.Fprintln(out, "adb:    not found -", adbErr)
			} else {
				fmt.Fprintf(out, "adb:    %s (%s)\n", adb.Path, adb.Source)
			}

			scrcpy, scrcpyErr := mgr.ResolveScrcpy()
			if scrcpyErr != nil {
				fmt.Fprintln(out, "scrcpy: not found -", scrcpyErr)
			} else {
				fmt.Fprintf(out, "scrcpy: %s (%s)\n", scrcpy.Path, scrcpy.Source)
			}
			return nil
		},
	}
}

func newToolsCheckCmd() *cobra.Command {
	var targetOS, targetArch string

	cmd := &cobra.Command{
		Use:   "check",
		Short: "Checks (without downloading) whether newer adb/scrcpy versions are available",
		RunE: func(cmd *cobra.Command, args []string) error {
			ac := appContextFrom(cmd.Context())
			mgr := toolsmanager.New(ac.Paths.ToolsDir)
			out := cmd.OutOrStdout()
			ctx := cmd.Context()

			if status, err := mgr.CheckADBUpdate(ctx, targetOS); err != nil {
				fmt.Fprintln(out, "adb:    version check failed -", err)
			} else {
				fmt.Fprintln(out, "adb:   ", updateStatusLine(status))
			}

			scrcpyStatus := mgr.CheckScrcpyUpdate(ctx, targetOS, targetArch)
			fmt.Fprintln(out, "scrcpy:", updateStatusLine(scrcpyStatus))
			return nil
		},
	}
	cmd.Flags().StringVar(&targetOS, "os", runtime.GOOS, "Target operating system (windows, linux, darwin)")
	cmd.Flags().StringVar(&targetArch, "arch", runtime.GOARCH, "Target architecture (amd64, arm64, 386)")
	return cmd
}

// updateStatusLine renders a ToolUpdateStatus as one human-readable line.
// Installed is blank whenever the tool was never fetched by a version of
// this app that recorded a marker (a pre-existing bundled copy, or one from
// before this feature existed) - "unknown" makes that distinction clear
// rather than implying the tool is actually missing.
func updateStatusLine(status toolsmanager.ToolUpdateStatus) string {
	installed := status.Installed
	if installed == "" {
		installed = "unknown"
	}
	if status.Available {
		return fmt.Sprintf("update available (installed: %s, new: %s)", installed, status.Latest)
	}
	return fmt.Sprintf("up to date (%s)", installed)
}

func newToolsUpdateCmd() *cobra.Command {
	var targetOS, targetArch string
	var force bool

	cmd := &cobra.Command{
		Use:   "update",
		Short: "Updates adb/scrcpy if a newer version is available",
		RunE: func(cmd *cobra.Command, args []string) error {
			ac := appContextFrom(cmd.Context())
			mgr := toolsmanager.New(ac.Paths.ToolsDir)
			out := cmd.OutOrStdout()
			ctx := cmd.Context()
			progress := func(msg string) { fmt.Fprintln(out, msg) }

			adbStatus, adbErr := mgr.CheckADBUpdate(ctx, targetOS)
			switch {
			case adbErr != nil:
				fmt.Fprintln(out, "adb: version check failed -", adbErr)
			case force || adbStatus.Available:
				fmt.Fprintln(out, "adb: updating ...")
				if err := mgr.FetchADB(ctx, targetOS, progress); err != nil {
					return fmt.Errorf("adb: %w", err)
				}
			default:
				fmt.Fprintln(out, "adb: already up to date.")
			}

			scrcpyStatus := mgr.CheckScrcpyUpdate(ctx, targetOS, targetArch)
			if force || scrcpyStatus.Available {
				fmt.Fprintln(out, "scrcpy: updating ...")
				if err := mgr.FetchScrcpy(ctx, targetOS, targetArch, progress); err != nil {
					return fmt.Errorf("scrcpy: %w", err)
				}
			} else {
				fmt.Fprintln(out, "scrcpy: already up to date.")
			}

			fmt.Fprintln(out, "Done.")
			return nil
		},
	}
	cmd.Flags().StringVar(&targetOS, "os", runtime.GOOS, "Target operating system (windows, linux, darwin)")
	cmd.Flags().StringVar(&targetArch, "arch", runtime.GOARCH, "Target architecture (amd64, arm64, 386)")
	cmd.Flags().BoolVar(&force, "force", false, "Always re-download, even if already up to date")
	return cmd
}
