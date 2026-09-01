//go:build !windows

package toolsmanager

import (
	"context"
	"os/exec"
)

// ScriptCommand runs path directly - unlike Windows, cmdline-tools' Unix
// shell scripts (sdkmanager/avdmanager) are directly executable via their
// own shebang line, so no interpreter wrapping is needed.
func ScriptCommand(ctx context.Context, path string, args ...string) *exec.Cmd {
	return exec.CommandContext(ctx, path, args...)
}
