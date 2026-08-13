package main

import (
	"fmt"
	"os"
	"runtime"

	"github.com/spf13/cobra"

	"android-toolbox/internal/buildinfo"
	"android-toolbox/internal/selfupdate"
)

// newSelfUpdateCmd checks GitHub for a newer android-toolbox release and,
// unless --check is given, downloads and installs it in place.
//
// PersistentPreRunE is overridden to a no-op for the same reason
// dangerous-reset's is: this command must keep working even if
// settings.yaml/state.json are broken, since "get me a fresh, known-good
// build" is exactly the situation where depending on today's (possibly
// broken) config would be self-defeating.
func newSelfUpdateCmd() *cobra.Command {
	var checkOnly, yes bool

	cmd := &cobra.Command{
		Use:               "self-update",
		Short:             "Checks for a new android-toolbox version and installs it",
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error { return nil },
		RunE: func(cmd *cobra.Command, args []string) error {
			out := cmd.OutOrStdout()
			ctx := cmd.Context()

			rel, err := selfupdate.LatestRelease(ctx)
			if err != nil {
				return fmt.Errorf("version check failed: %w", err)
			}

			if !selfupdate.IsNewer(buildinfo.Version, rel.Version) {
				fmt.Fprintf(out, "android-toolbox is up to date (%s).\n", buildinfo.Version)
				return nil
			}

			fmt.Fprintf(out, "New version available: %s (current: %s)\n", rel.Version, buildinfo.Version)
			if checkOnly {
				if rel.HTMLURL != "" {
					fmt.Fprintf(out, "Release notes: %s\n", rel.HTMLURL)
				}
				return nil
			}

			asset, err := selfupdate.AssetFor(rel, runtime.GOOS, runtime.GOARCH)
			if err != nil {
				return err
			}

			if !yes {
				fmt.Fprintf(out, "Update to %s? [y/N] ", rel.Version)
				if !confirmYesNo(cmd.InOrStdin()) {
					fmt.Fprintln(out, "Cancelled.")
					return nil
				}
			}

			exePath, err := os.Executable()
			if err != nil {
				return fmt.Errorf("could not determine own executable path: %w", err)
			}

			progress := func(msg string) { fmt.Fprintln(out, msg) }
			if err := selfupdate.Apply(ctx, asset, exePath, runtime.GOOS, progress); err != nil {
				return fmt.Errorf("update failed: %w", err)
			}
			fmt.Fprintln(out, "Update installed - please restart android-toolbox.")
			return nil
		},
	}

	cmd.Flags().BoolVar(&checkOnly, "check", false, "Only check, don't install")
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "Update without confirmation")
	return cmd
}
