//go:build !windows

package install

import (
	"fmt"
	"os"
	"path/filepath"
)

// Install symlinks exePath into ~/.local/bin as both appName and aliasName.
// Unlike Windows there is no single portable way to persist a PATH change
// from Go across every shell (bash/zsh/fish rc files all differ), so if
// ~/.local/bin isn't already on PATH this just reports that back via Note
// instead of guessing which rc file to edit.
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
		note = fmt.Sprintf("%s is not on PATH. Add this line to your shell configuration: export PATH=\"$PATH:%s\"", installDir, installDir)
	}

	return Result{
		InstallDir:     installDir,
		InstalledFiles: []string{mainDest, aliasDest},
		OnPath:         onPath,
		Note:           note,
	}, nil
}
