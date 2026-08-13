package main

import (
	"fmt"

	"github.com/spf13/cobra"

	"android-toolbox/internal/adb"
	"android-toolbox/internal/device"
	"android-toolbox/internal/toolsmanager"
)

func newDevicesCmd() *cobra.Command {
	var verbose bool

	cmd := &cobra.Command{
		Use:   "devices",
		Short: "Lists connected Android devices",
		RunE: func(cmd *cobra.Command, args []string) error {
			ac := appContextFrom(cmd.Context())
			mgr := toolsmanager.New(ac.Paths.ToolsDir)
			tool, err := mgr.ResolveADB()
			if err != nil {
				return err
			}
			client := adb.New(tool.Path)

			devices, err := client.ListDevices(cmd.Context())
			if err != nil {
				return err
			}

			out := cmd.OutOrStdout()
			if len(devices) == 0 {
				fmt.Fprintln(out, "No devices connected.")
				return nil
			}

			for _, d := range devices {
				fmt.Fprintf(out, "%-20s %-12s %s\n", d.Serial, d.State, d.Model)
				if verbose && d.Connected() {
					info, err := device.Collect(cmd.Context(), client, d.Serial)
					if err != nil {
						fmt.Fprintf(out, "  Failed to fetch device info: %v\n", err)
						continue
					}
					fmt.Fprintf(out, "  Manufacturer: %s\n", info.Manufacturer)
					fmt.Fprintf(out, "  Android:      %s (SDK %s)\n", info.AndroidVersion, info.SDK)
					fmt.Fprintf(out, "  Resolution:   %s\n", info.Resolution)
					fmt.Fprintf(out, "  IP:           %s\n", info.IPAddress)
					fmt.Fprintf(out, "  Battery:      %d%% (%s)\n", info.Battery.Level, info.Battery.StatusText())
				}
			}
			return nil
		},
	}

	cmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "Show additional device information")
	return cmd
}
