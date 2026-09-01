//go:build windows

package toolsmanager

import (
	"context"
	"os/exec"
	"strings"
)

// ScriptCommand builds an *exec.Cmd for running one of cmdline-tools'
// script entry points (sdkmanager/avdmanager). Windows' CreateProcess cannot
// launch a .bat/.cmd file directly - only cmd.exe's batch interpreter can -
// so .bat/.cmd scripts are run via "cmd /c script.bat args...", letting
// Go's normal argv-escaping quote each argument (no manual quoting needed
// here, unlike shell_windows.go's shellCommand: that one re-assembles an
// already-quoted string, this one passes plain unquoted tokens straight
// through). A real .exe (the emulator binary) is run directly.
func ScriptCommand(ctx context.Context, path string, args ...string) *exec.Cmd {
	lower := strings.ToLower(path)
	if strings.HasSuffix(lower, ".bat") || strings.HasSuffix(lower, ".cmd") {
		full := append([]string{"/c", path}, args...)
		return exec.CommandContext(ctx, "cmd", full...)
	}
	return exec.CommandContext(ctx, path, args...)
}
