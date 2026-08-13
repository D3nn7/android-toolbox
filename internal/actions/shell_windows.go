//go:build windows

package actions

import (
	"context"
	"os/exec"
	"strings"
	"syscall"
)

// shellCommand runs command via cmd.exe. Go's os/exec normally re-quotes
// every argument using MSVCRT argv-escaping rules before building the
// Windows command line; since our command string already contains its own
// double-quoted path segments (see quoteForShell), that automatic escaping
// would backslash-escape those quotes into a form cmd.exe's own (different,
// simpler) quoting rules cannot parse back out - e.g. `"C:\a b\adb.exe"`
// becomes `\"C:\a b\adb.exe\"` and cmd reports the exe as "not recognized".
// Setting SysProcAttr.CmdLine bypasses that re-quoting entirely and hands
// cmd.exe exactly the command line we built.
func shellCommand(ctx context.Context, command string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, "cmd")
	// cmd.exe's own (undocumented-ish) quote-stripping rule for /C only
	// preserves inner quotes verbatim in one narrow case: when the whole
	// remainder is wrapped in exactly one extra outer quote pair, cmd's
	// "legacy" fallback strips precisely the first character (our opening
	// wrapper quote) and the last quote character in the string (which,
	// because we appended the wrapper's closing quote at the very end,
	// is that same wrapper quote) - leaving every inner "..."-quoted
	// segment (e.g. quoted adb paths) and any && chaining untouched.
	// Without this wrapper, cmd's rule instead strips the first quote and
	// the last quote *anywhere* in the string, which for a multi-command
	// chain silently unbalances the inner quotes and breaks parsing.
	cmd.SysProcAttr = &syscall.SysProcAttr{CmdLine: `/C "` + command + `"`}
	return cmd
}

// effectiveCommandLine returns the command line that will actually be sent
// to CreateProcess - used by tests, since on Windows it lives in
// SysProcAttr.CmdLine rather than cmd.Args once shellCommand has set it.
func effectiveCommandLine(cmd *exec.Cmd) string {
	if cmd.SysProcAttr != nil && cmd.SysProcAttr.CmdLine != "" {
		return cmd.SysProcAttr.CmdLine
	}
	return strings.Join(cmd.Args, " ")
}
