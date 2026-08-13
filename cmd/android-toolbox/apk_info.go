package main

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"android-toolbox/internal/apkinfo"
)

func newAPKInfoCmd() *cobra.Command {
	var asJSON bool

	cmd := &cobra.Command{
		Use:   "apk-info <path-to-apk>",
		Short: "Analyzes an APK file (package, version, permissions, signing)",
		Long: "Analyzes an .apk file without external tools (no aapt, no Android SDK) -\n" +
			"a pure Go implementation that works identically on Windows, Linux, and\n" +
			"macOS. Reads AndroidManifest.xml (package name, version, SDK level,\n" +
			"permissions, activities) and, if present, the APK Signing Block v2/v3\n" +
			"for signing-certificate information.",
		Args: cobra.ExactArgs(1),
		// Overridden to a no-op for the same reason self-update's and
		// dangerous-reset's are: this command is a standalone file
		// analyzer that needs no config/settings/state at all, so it
		// shouldn't depend on (or be broken by) any of that being present.
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error { return nil },
		RunE: func(cmd *cobra.Command, args []string) error {
			info, err := apkinfo.Analyze(args[0])
			if err != nil {
				return err
			}

			out := cmd.OutOrStdout()
			if asJSON {
				enc := json.NewEncoder(out)
				enc.SetIndent("", "  ")
				return enc.Encode(info)
			}
			fmt.Fprint(out, formatAPKInfo(info))
			return nil
		},
	}

	cmd.Flags().BoolVar(&asJSON, "json", false, "Output as JSON instead of human-readable text")
	return cmd
}

func formatAPKInfo(info apkinfo.Info) string {
	var b strings.Builder

	fmt.Fprintf(&b, "File:         %s\n", info.Path)
	fmt.Fprintf(&b, "Size:         %s (%d bytes)\n", formatByteSize(info.SizeBytes), info.SizeBytes)
	fmt.Fprintf(&b, "SHA-256:      %s\n", info.SHA256)
	fmt.Fprintf(&b, "Entries:      %d\n\n", info.EntryCount)

	m := info.Manifest
	fmt.Fprintf(&b, "Package:      %s\n", orDashAPK(m.PackageName))
	fmt.Fprintf(&b, "Version:      %s (Code %d)\n", orDashAPK(m.VersionName), m.VersionCode)
	fmt.Fprintf(&b, "Min SDK:      %d\n", m.MinSDK)
	fmt.Fprintf(&b, "Target SDK:   %d\n", m.TargetSDK)
	if m.CompileSDK != 0 {
		fmt.Fprintf(&b, "Compile SDK:  %d\n", m.CompileSDK)
	}
	fmt.Fprintf(&b, "App label:    %s\n", orDashAPK(m.ApplicationLabel))
	if m.MainActivity != "" {
		fmt.Fprintf(&b, "Main activity: %s\n", m.MainActivity)
	}

	fmt.Fprintf(&b, "\nPermissions (%d):\n", len(m.Permissions))
	for _, p := range m.Permissions {
		fmt.Fprintf(&b, "  - %s\n", p)
	}
	if len(m.Features) > 0 {
		fmt.Fprintf(&b, "\nHardware/feature requirements (%d):\n", len(m.Features))
		for _, f := range m.Features {
			fmt.Fprintf(&b, "  - %s\n", f)
		}
	}
	if len(m.Activities) > 0 {
		fmt.Fprintf(&b, "\nActivities (%d):\n", len(m.Activities))
		for _, a := range m.Activities {
			fmt.Fprintf(&b, "  - %s\n", a)
		}
	}

	b.WriteString("\nSigning:\n")
	s := info.Signing
	switch {
	case s.SchemeV2 || s.SchemeV3:
		var schemes []string
		if s.SchemeV2 {
			schemes = append(schemes, "v2")
		}
		if s.SchemeV3 {
			schemes = append(schemes, "v3")
		}
		fmt.Fprintf(&b, "  Scheme:     %s\n", strings.Join(schemes, ", "))
		for i, c := range s.Certificates {
			fmt.Fprintf(&b, "  Certificate #%d:\n", i+1)
			fmt.Fprintf(&b, "    Subject:  %s\n", c.Subject)
			fmt.Fprintf(&b, "    Issuer:   %s\n", c.Issuer)
			fmt.Fprintf(&b, "    Serial:   %s\n", c.SerialNumber)
			fmt.Fprintf(&b, "    Valid:    %s to %s\n", c.NotBefore, c.NotAfter)
			fmt.Fprintf(&b, "    SHA-256:  %s\n", c.SHA256)
		}
	case s.SchemeV1Only:
		b.WriteString("  Scheme:     v1 (JAR signature) - certificate details not decoded\n")
	default:
		b.WriteString("  No signature block found (unsigned or unknown format)\n")
	}
	if s.Err != "" {
		fmt.Fprintf(&b, "  Note:       Signature block found, but could not be fully parsed: %s\n", s.Err)
	}

	return b.String()
}

func orDashAPK(s string) string {
	if s == "" {
		return "-"
	}
	return s
}

// formatByteSize renders n as a human-readable size (KB/MB/GB), matching
// common convention (1024-based units, one decimal place).
func formatByteSize(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for v := n / unit; v >= unit; v /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(n)/float64(div), "KMGTPE"[exp])
}
