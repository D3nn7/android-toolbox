package main

import (
	"context"
	"fmt"
	"runtime"

	"github.com/spf13/cobra"

	"android-toolbox/internal/adb"
	"android-toolbox/internal/avd"
	"android-toolbox/internal/toolsmanager"
)

func newEmulatorCmd() *cobra.Command {
	emulatorCmd := &cobra.Command{
		Use:   "emulator",
		Short: "Create and manage Android Virtual Devices (AVDs)",
	}

	emulatorCmd.AddCommand(newEmulatorSetupCmd())
	emulatorCmd.AddCommand(newEmulatorListCmd())
	emulatorCmd.AddCommand(newEmulatorCreateCmd())
	emulatorCmd.AddCommand(newEmulatorStartCmd())
	emulatorCmd.AddCommand(newEmulatorStopCmd())
	emulatorCmd.AddCommand(newEmulatorDeleteCmd())

	return emulatorCmd
}

// newAvdManager resolves every tool the emulator manager needs and builds an
// *avd.Manager from them - shared by every subcommand below. Individual
// resolution failures aren't fatal here (an empty path degrades to a clear
// per-operation error, same convention as internal/scrcpy.Launcher/
// internal/avd.Manager themselves), so e.g. "emulator list" still works
// with only avdmanager resolved.
func newAvdManager(toolsDir string) *avd.Manager {
	mgr := toolsmanager.New(toolsDir)
	avdManagerTool, _ := mgr.ResolveAvdManager()
	sdkManagerTool, _ := mgr.ResolveSdkManager()
	emulatorTool, _ := mgr.ResolveEmulator()
	return avd.New(avdManagerTool.Path, sdkManagerTool.Path, emulatorTool.Path, mgr.SdkRoot())
}

func newEmulatorSetupCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "setup",
		Short: "Downloads the Android cmdline-tools and the emulator package, and reports tool status",
		RunE: func(cmd *cobra.Command, args []string) error {
			ac := appContextFrom(cmd.Context())
			out := cmd.OutOrStdout()
			ctx := cmd.Context()
			progress := func(msg string) { fmt.Fprintln(out, msg) }

			if _, err := toolsmanager.ResolveJava(); err != nil {
				fmt.Fprintln(out, "java:       not found -", err)
			} else {
				fmt.Fprintln(out, "java:       OK")
			}

			mgr := toolsmanager.New(ac.Paths.ToolsDir)
			if _, err := mgr.ResolveSdkManager(); err != nil {
				if err := mgr.FetchCmdlineTools(ctx, runtime.GOOS, runtime.GOARCH, progress); err != nil {
					return fmt.Errorf("cmdline-tools: %w", err)
				}
			}

			sdkManager, sdkErr := mgr.ResolveSdkManager()
			if sdkErr != nil {
				fmt.Fprintln(out, "sdkmanager: not found -", sdkErr)
			} else {
				fmt.Fprintf(out, "sdkmanager: %s (%s)\n", sdkManager.Path, sdkManager.Source)
			}
			avdManager, avdErr := mgr.ResolveAvdManager()
			if avdErr != nil {
				fmt.Fprintln(out, "avdmanager: not found -", avdErr)
			} else {
				fmt.Fprintf(out, "avdmanager: %s (%s)\n", avdManager.Path, avdManager.Source)
			}

			// The emulator package (unlike cmdline-tools) has no direct
			// download URL at all - installed the same way a missing system
			// image is installed for the create wizard, via the just-
			// resolved sdkmanager, rather than telling the user to run a
			// bare "sdkmanager" command they'd have no way to locate (it's
			// bundled, not on PATH).
			if _, err := mgr.ResolveEmulator(); err != nil && sdkErr == nil {
				fmt.Fprintln(out, "emulator:   installing...")
				if err := toolsmanager.InstallSdkPackage(ctx, sdkManager.Path, mgr.SdkRoot(), "emulator", progress); err != nil {
					fmt.Fprintln(out, "emulator:   install failed -", err)
				}
			}
			emulator, emuErr := mgr.ResolveEmulator()
			if emuErr != nil {
				fmt.Fprintln(out, "emulator:   not found -", emuErr)
			} else {
				fmt.Fprintf(out, "emulator:   %s (%s)\n", emulator.Path, emulator.Source)
			}
			return nil
		},
	}
}

func newEmulatorListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "Lists locally defined Android Virtual Devices",
		RunE: func(cmd *cobra.Command, args []string) error {
			ac := appContextFrom(cmd.Context())
			out := cmd.OutOrStdout()

			manager := newAvdManager(ac.Paths.ToolsDir)
			avds, err := manager.List(cmd.Context())
			if err != nil {
				return err
			}
			if len(avds) == 0 {
				fmt.Fprintln(out, "No AVDs defined.")
				return nil
			}

			for _, a := range avds {
				if a.Broken {
					fmt.Fprintf(out, "%-24s [broken] %s\n", a.Name, a.Error)
					continue
				}
				fmt.Fprintf(out, "%-24s %-16s %s\n", a.Name, a.ABI, a.Target)
			}
			return nil
		},
	}
}

func newEmulatorCreateCmd() *cobra.Command {
	var name, image, device string
	var sdcardMB int
	var force bool

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Creates a new AVD, downloading its system image first if needed",
		RunE: func(cmd *cobra.Command, args []string) error {
			ac := appContextFrom(cmd.Context())
			out := cmd.OutOrStdout()
			ctx := cmd.Context()

			mgr := toolsmanager.New(ac.Paths.ToolsDir)
			manager := newAvdManager(ac.Paths.ToolsDir)
			if manager.SdkManagerPath == "" || manager.AvdManagerPath == "" {
				return fmt.Errorf("sdkmanager/avdmanager not available - run 'android-toolbox emulator setup'")
			}

			installed, _, err := toolsmanager.ListSdkPackages(ctx, manager.SdkManagerPath, mgr.SdkRoot())
			if err != nil {
				return err
			}
			haveImage := false
			for _, p := range installed {
				if p == image {
					haveImage = true
					break
				}
			}
			if !haveImage {
				fmt.Fprintf(out, "Downloading system image %s...\n", image)
				progress := func(msg string) { fmt.Fprintln(out, msg) }
				if err := toolsmanager.InstallSdkPackage(ctx, manager.SdkManagerPath, mgr.SdkRoot(), image, progress); err != nil {
					return fmt.Errorf("system image: %w", err)
				}
			}

			if err := manager.Create(ctx, avd.CreateOptions{
				Name:        name,
				SystemImage: image,
				Device:      device,
				SDCardMB:    sdcardMB,
				Force:       force,
			}); err != nil {
				return err
			}
			fmt.Fprintf(out, "AVD %q created.\n", name)
			return nil
		},
	}
	cmd.Flags().StringVar(&name, "name", "", "AVD name (required)")
	cmd.Flags().StringVar(&image, "image", "", `System image package, e.g. "system-images;android-34;google_apis;x86_64" (required)`)
	cmd.Flags().StringVar(&device, "device", "", "Hardware device profile id, e.g. \"pixel_6\" (see 'avdmanager list device')")
	cmd.Flags().IntVar(&sdcardMB, "sdcard", 0, "SD card size in MB (0 = none)")
	cmd.Flags().BoolVar(&force, "force", false, "Overwrite an existing AVD of the same name")
	_ = cmd.MarkFlagRequired("name")
	_ = cmd.MarkFlagRequired("image")
	return cmd
}

func newEmulatorStartCmd() *cobra.Command {
	var headless bool

	cmd := &cobra.Command{
		Use:   "start <name>",
		Short: "Starts an emulator by AVD name",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ac := appContextFrom(cmd.Context())
			mgr := toolsmanager.New(ac.Paths.ToolsDir)
			emulatorTool, err := mgr.ResolveEmulator()
			if err != nil {
				return err
			}
			adbTool, err := mgr.ResolveADB()
			if err != nil {
				return err
			}
			if err := mgr.EnsureEmulatorPlatformTools(adbTool.Path); err != nil {
				return err
			}

			windowed := ac.Settings.Emulator.Windowed && !headless
			launcher := avd.NewLauncher(emulatorTool.Path, ac.Paths.LogsDir, mgr.SdkRoot())
			result, err := launcher.Launch(args[0], windowed, ac.Settings.Emulator.ExtraArgs, nil)
			if err != nil {
				return err
			}
			out := cmd.OutOrStdout()
			fmt.Fprintf(out, "Starting %q...\n", args[0])
			if result.LogPath != "" {
				fmt.Fprintf(out, "Log: %s\n", result.LogPath)
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&headless, "headless", false, "Start without a visible window, overriding settings.yaml")
	return cmd
}

func newEmulatorStopCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "stop <name>",
		Short: "Stops a running emulator by AVD name",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ac := appContextFrom(cmd.Context())
			ctx := cmd.Context()
			mgr := toolsmanager.New(ac.Paths.ToolsDir)
			adbTool, err := mgr.ResolveADB()
			if err != nil {
				return err
			}
			client := adb.New(adbTool.Path)

			serial, err := findRunningAVDSerial(ctx, client, args[0])
			if err != nil {
				return err
			}
			if serial == "" {
				return fmt.Errorf("no running emulator found for AVD %q", args[0])
			}
			if _, err := client.Emu(ctx, serial, "kill"); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Stopped %q (%s).\n", args[0], serial)
			return nil
		},
	}
}

// findRunningAVDSerial returns the emulator-* serial currently backing avdName,
// or "" if none is running.
func findRunningAVDSerial(ctx context.Context, client *adb.Client, avdName string) (string, error) {
	devices, err := client.ListDevices(ctx)
	if err != nil {
		return "", err
	}
	for _, d := range devices {
		if !adb.IsEmulatorSerial(d.Serial) || !d.Connected() {
			continue
		}
		name, err := client.EmuAVDName(ctx, d.Serial)
		if err == nil && name == avdName {
			return d.Serial, nil
		}
	}
	return "", nil
}

func newEmulatorDeleteCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "delete <name>",
		Short: "Deletes an AVD",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ac := appContextFrom(cmd.Context())
			manager := newAvdManager(ac.Paths.ToolsDir)
			if err := manager.Delete(cmd.Context(), args[0]); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "AVD %q deleted.\n", args[0])
			return nil
		},
	}
}
