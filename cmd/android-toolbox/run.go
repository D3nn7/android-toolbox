package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"android-toolbox/internal/actions"
	"android-toolbox/internal/adb"
	"android-toolbox/internal/scrcpy"
	"android-toolbox/internal/toolsmanager"
)

func newRunCmd() *cobra.Command {
	var serial string
	var paramFlags []string

	cmd := &cobra.Command{
		Use:   "run <action-id>",
		Short: "Runs a configured action non-interactively",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ac := appContextFrom(cmd.Context())

			set, err := actions.Load(ac.Paths.ActionsFile, actions.DefaultActionsYAML)
			if err != nil {
				return err
			}
			action, ok := set.Find(args[0])
			if !ok {
				return fmt.Errorf("unknown action %q (see 'android-toolbox tools status' / actions.yaml)", args[0])
			}

			mgr := toolsmanager.New(ac.Paths.ToolsDir)
			adbTool, err := mgr.ResolveADB()
			if err != nil {
				return err
			}

			var launcher *scrcpy.Launcher
			if scrcpyTool, err := mgr.ResolveScrcpy(); err == nil {
				launcher = scrcpy.New(scrcpyTool.Path, ac.Settings.Scrcpy.DefaultArgs, ac.Paths.LogsDir)
			}

			if serial == "" {
				client := adb.New(adbTool.Path)
				devices, err := client.ListDevices(cmd.Context())
				if err != nil {
					return err
				}
				if len(devices) != 1 {
					return fmt.Errorf("no --serial given and not exactly 1 device connected (%d found)", len(devices))
				}
				serial = devices[0].Serial
			}

			supplied := map[string]string{}
			for _, kv := range paramFlags {
				for i := 0; i < len(kv); i++ {
					if kv[i] == '=' {
						supplied[kv[:i]] = kv[i+1:]
						break
					}
				}
			}

			if action.Confirm {
				fmt.Fprintf(cmd.OutOrStdout(), "Action %q requires confirmation. Pass --yes to run it anyway.\n", action.ID)
				yes, _ := cmd.Flags().GetBool("yes")
				if !yes {
					return fmt.Errorf("aborted (--yes missing)")
				}
			}

			executor := actions.NewExecutor(adbTool.Path, launcher)

			if action.Tool == actions.ToolScrcpy {
				proc, err := executor.StartScrcpy(action, serial, supplied)
				if err != nil {
					return err
				}
				fmt.Fprintf(cmd.OutOrStdout(), "scrcpy started (PID %d)\n", proc.Process.Pid)
				return nil
			}

			return executor.RunSync(cmd.Context(), action, serial, supplied, os.Stdout, os.Stderr)
		},
	}

	cmd.Flags().StringVar(&serial, "serial", "", "Target device (default: automatic, if exactly 1 is connected)")
	cmd.Flags().StringArrayVar(&paramFlags, "param", nil, "Parameter in the form name=value (repeatable)")
	cmd.Flags().Bool("yes", false, "Run confirmation-requiring actions without asking")
	return cmd
}
