//go:build !windows

package install

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// Install symlinks exePath into ~/.local/bin as both appName and aliasName,
// then tries to persist that directory on PATH by appending an export line
// to the current shell's rc file (see shellRCFile) - without this, the
// symlinks exist but a freshly opened terminal still can't find the command,
// which is exactly the "installed, but not found after restarting the
// terminal" complaint this is meant to fix. If the rc file can't be
// determined or written, this falls back to reporting the manual step via
// Note, same as before.
func Install(exePath, appName, aliasName string) (Result, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return Result{}, fmt.Errorf("could not determine home directory: %w", err)
	}

	installDir := filepath.Join(home, ".local", "bin")
	if err := os.MkdirAll(installDir, 0o755); err != nil {
		return Result{}, fmt.Errorf("could not create install directory: %w", err)
	}

	mainDest := filepath.Join(installDir, appName)
	aliasDest := filepath.Join(installDir, aliasName)
	for _, dest := range []string{mainDest, aliasDest} {
		_ = os.Remove(dest) // symlink creation fails if the target already exists
		if err := os.Symlink(exePath, dest); err != nil {
			return Result{}, fmt.Errorf("symlink %s failed: %w", dest, err)
		}
	}

	onPath := false
	for _, p := range filepath.SplitList(os.Getenv("PATH")) {
		if p == installDir {
			onPath = true
			break
		}
	}

	note := ""
	if !onPath {
		note = persistPathViaShellRC(home, installDir)
	}

	return Result{
		InstallDir:     installDir,
		InstalledFiles: []string{mainDest, aliasDest},
		OnPath:         onPath,
		Note:           note,
	}, nil
}

// persistPathViaShellRC appends a PATH export line for installDir to the
// current user's shell rc file (picked via shellRCFile), unless it's already
// there. Returns a human-readable note describing what happened, always
// telling the user a new terminal is required either way.
func persistPathViaShellRC(home, installDir string) string {
	rcFile, exportLine := shellRCFile(home, installDir)
	if rcFile == "" {
		return fmt.Sprintf("%s is not on PATH. Add this line to your shell configuration: export PATH=\"$PATH:%s\"", installDir, installDir)
	}

	existing, readErr := os.ReadFile(rcFile)
	if readErr == nil && strings.Contains(string(existing), installDir) {
		return fmt.Sprintf("%s is not on PATH yet in this session, but %s already references it. Open a new terminal for the change to take effect.", installDir, rcFile)
	}

	if err := os.MkdirAll(filepath.Dir(rcFile), 0o755); err != nil {
		return fmt.Sprintf("%s is not on PATH. Add this line to your shell configuration: export PATH=\"$PATH:%s\"", installDir, installDir)
	}

	f, err := os.OpenFile(rcFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return fmt.Sprintf("%s is not on PATH. Add this line to your shell configuration: export PATH=\"$PATH:%s\"", installDir, installDir)
	}
	defer f.Close()

	block := fmt.Sprintf("\n# Added by android-toolbox install\n%s\n", exportLine)
	if _, err := f.WriteString(block); err != nil {
		return fmt.Sprintf("%s is not on PATH. Add this line to your shell configuration: export PATH=\"$PATH:%s\"", installDir, installDir)
	}

	return fmt.Sprintf("Added %s to PATH in %s. Open a new terminal for the change to take effect.", installDir, rcFile)
}

// shellRCFile picks which rc file to append a PATH change to, based on the
// user's login shell ($SHELL), and returns the matching export line for that
// shell's syntax. macOS defaults to zsh (since Catalina) and runs bash as a
// login shell (~/.bash_profile, not ~/.bashrc); Linux distros default to bash
// with ~/.bashrc for interactive non-login shells. Returns ("", "") if no
// shell could be determined, leaving the caller to fall back to a manual
// instruction.
func shellRCFile(home, installDir string) (rcFile, exportLine string) {
	shell := filepath.Base(os.Getenv("SHELL"))
	exportLine = fmt.Sprintf(`export PATH="$PATH:%s"`, installDir)

	switch {
	case strings.Contains(shell, "fish"):
		return filepath.Join(home, ".config", "fish", "config.fish"), fmt.Sprintf(`set -gx PATH $PATH %s`, installDir)
	case strings.Contains(shell, "zsh"):
		return filepath.Join(home, ".zshrc"), exportLine
	case strings.Contains(shell, "bash"):
		if runtime.GOOS == "darwin" {
			return filepath.Join(home, ".bash_profile"), exportLine
		}
		return filepath.Join(home, ".bashrc"), exportLine
	case shell == "":
		// $SHELL isn't set (e.g. some minimal/non-interactive environments) -
		// fall back to the OS default rather than giving up.
		if runtime.GOOS == "darwin" {
			return filepath.Join(home, ".zshrc"), exportLine
		}
		return filepath.Join(home, ".profile"), exportLine
	default:
		return "", ""
	}
}
