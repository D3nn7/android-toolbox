package main

import (
	"fmt"

	"github.com/spf13/cobra"

	"android-toolbox/internal/buildinfo"
)

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Shows version information",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Fprintf(cmd.OutOrStdout(), "android-toolbox %s (commit %s)\n", buildinfo.Version, buildinfo.Commit)
			return nil
		},
	}
}
