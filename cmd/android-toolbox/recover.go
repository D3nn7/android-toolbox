package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"android-toolbox/internal/backup"
)

func printBackupList(out io.Writer, entries []backup.Entry) {
	for i, e := range entries {
		fmt.Fprintf(out, "  [%d] %s  (%s)\n", i+1, e.OriginalName, e.Timestamp.Format("2006-01-02 15:04:05"))
	}
}

func newRecoverCmd() *cobra.Command {
	var listOnly bool

	cmd := &cobra.Command{
		Use:   "recover [index]",
		Short: "Restores an earlier configuration backup",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ac := appContextFrom(cmd.Context())
			out := cmd.OutOrStdout()

			entries, err := backup.List(ac.Paths.BackupDir)
			if err != nil {
				return err
			}
			if len(entries) == 0 {
				fmt.Fprintln(out, "No backups available.")
				return nil
			}

			fmt.Fprintln(out, "Available backups (newest first):")
			printBackupList(out, entries)

			if listOnly {
				return nil
			}

			var index int
			if len(args) == 1 {
				index, err = strconv.Atoi(args[0])
				if err != nil {
					return fmt.Errorf("invalid index %q", args[0])
				}
			} else {
				fmt.Fprint(out, "\nWhich backup should be restored? [number, empty = cancel] ")
				reader := bufio.NewReader(os.Stdin)
				line, _ := reader.ReadString('\n')
				line = strings.TrimSpace(line)
				if line == "" {
					fmt.Fprintln(out, "Cancelled.")
					return nil
				}
				index, err = strconv.Atoi(line)
				if err != nil {
					return fmt.Errorf("invalid input %q", line)
				}
			}

			if index < 1 || index > len(entries) {
				return fmt.Errorf("index out of range (1-%d)", len(entries))
			}

			entry := entries[index-1]
			if err := backup.Restore(entry, ac.Paths.ConfigDir, ac.Paths.BackupDir); err != nil {
				return err
			}
			fmt.Fprintf(out, "%s restored from backup dated %s.\n", entry.OriginalName, entry.Timestamp.Format("2006-01-02 15:04:05"))
			return nil
		},
	}

	cmd.Flags().BoolVar(&listOnly, "list", false, "Only list, don't restore anything")
	return cmd
}
