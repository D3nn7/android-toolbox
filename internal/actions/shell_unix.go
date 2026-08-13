//go:build !windows

package actions

import (
	"context"
	"os/exec"
	"strings"
)

func shellCommand(ctx context.Context, command string) *exec.Cmd {
	return exec.CommandContext(ctx, "sh", "-c", command)
}

// effectiveCommandLine returns a human-readable rendering of the command
// that will run - used by tests.
func effectiveCommandLine(cmd *exec.Cmd) string {
	return strings.Join(cmd.Args, " ")
}
