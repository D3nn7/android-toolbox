package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"android-toolbox/internal/actions"
	"android-toolbox/internal/ai"
	"android-toolbox/internal/backup"
)

func newAICmd() *cobra.Command {
	var yes bool

	cmd := &cobra.Command{
		Use:   "ai <request>",
		Short: "Generates a new action from an AI request",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ac := appContextFrom(cmd.Context())
			out := cmd.OutOrStdout()

			provider, err := ai.New(ac.Settings.AI.Provider, ac.Settings.AI.Claude.Command, ac.Settings.AI.Claude.TimeoutSeconds, ac.Paths.AIPromptFile)
			if err != nil {
				return err
			}
			if err := provider.Available(); err != nil {
				return err
			}

			set, err := actions.Load(ac.Paths.ActionsFile, actions.DefaultActionsYAML)
			if err != nil {
				return err
			}
			existingIDs := make([]string, len(set.Actions))
			for i, a := range set.Actions {
				existingIDs[i] = a.ID
			}

			fmt.Fprintln(out, "Asking AI provider, please wait...")
			draft, err := provider.GenerateAction(cmd.Context(), ai.GenerateRequest{
				UserPrompt:  strings.Join(args, " "),
				ExistingIDs: existingIDs,
			})
			if err != nil {
				return err
			}

			action := draft.ToAction()
			fmt.Fprintf(out, "\nProposal:\n  id:          %s\n  name:        %s\n  description: %s\n  category:    %s\n  tool:        %s\n  command:     %s\n  confirm:     %v\n  interactive: %v\n",
				action.ID, action.Name, action.Description, action.Category, action.Tool, action.Command, action.Confirm, action.Interactive)
			if len(action.Params) > 0 {
				fmt.Fprintln(out, "  params:")
				for _, p := range action.Params {
					fmt.Fprintf(out, "    - %s (%s), default=%q\n", p.Name, p.Label, p.Default)
				}
			}

			if !yes {
				fmt.Fprint(out, "\nSave as a new action? [y/N] ")
				reader := bufio.NewReader(os.Stdin)
				line, _ := reader.ReadString('\n')
				if strings.TrimSpace(strings.ToLower(line)) != "y" {
					fmt.Fprintln(out, "Cancelled.")
					return nil
				}
			}

			err = backup.BeforeWrite(ac.Paths.BackupDir, ac.Paths.ActionsFile, func() error {
				return actions.Append(ac.Paths.ActionsFile, action)
			})
			if err != nil {
				return err
			}
			fmt.Fprintf(out, "Action %q saved.\n", action.ID)
			return nil
		},
	}

	cmd.Flags().BoolVar(&yes, "yes", false, "Save without confirmation")
	return cmd
}
