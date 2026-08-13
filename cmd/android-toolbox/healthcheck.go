package main

import (
	"fmt"

	"github.com/spf13/cobra"

	"android-toolbox/internal/healthcheck"
)

func severityLabel(s healthcheck.Severity) string {
	switch s {
	case healthcheck.OK:
		return "[ OK ]"
	case healthcheck.Warn:
		return "[WARN]"
	default:
		return "[FAIL]"
	}
}

func printHealthReport(cmd *cobra.Command, report healthcheck.Report) {
	out := cmd.OutOrStdout()
	for _, r := range report.Results {
		fmt.Fprintf(out, "%s %-28s %s\n", severityLabel(r.Severity), r.Name, r.Detail)
		if r.Remediation != "" {
			fmt.Fprintf(out, "        -> %s\n", r.Remediation)
		}
	}
}

func newHealthcheckCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "healthcheck",
		Short: "Checks whether all required tools and configuration are present",
		RunE: func(cmd *cobra.Command, args []string) error {
			ac := appContextFrom(cmd.Context())
			report := healthcheck.Run(cmd.Context(), ac.Paths, ac.Settings)
			printHealthReport(cmd, report)
			if report.HasFailures() {
				return fmt.Errorf("healthcheck: at least one check failed")
			}
			return nil
		},
	}
}
