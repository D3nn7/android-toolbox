// Package config resolves application paths and loads/saves user settings and state.
package config

import (
	"os"
	"path/filepath"
)

const appDirName = "android-toolbox"

// Paths holds every filesystem location the application reads from or writes to.
type Paths struct {
	ConfigDir    string
	ActionsFile  string
	SettingsFile string
	StateFile    string
	BackupDir    string
	ToolsDir     string
	LogsDir      string
	AIPromptFile string
}

// Resolve returns the OS-appropriate application paths, creating the base
// directories if they do not yet exist.
func Resolve() (Paths, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return Paths{}, err
	}

	configDir := filepath.Join(base, appDirName)
	p := Paths{
		ConfigDir:    configDir,
		ActionsFile:  filepath.Join(configDir, "actions.yaml"),
		SettingsFile: filepath.Join(configDir, "settings.yaml"),
		StateFile:    filepath.Join(configDir, "state.json"),
		BackupDir:    filepath.Join(configDir, ".backup"),
		ToolsDir:     filepath.Join(configDir, "tools"),
		LogsDir:      filepath.Join(configDir, "logs"),
		AIPromptFile: filepath.Join(configDir, "ai", "system_prompt.md"),
	}

	for _, dir := range []string{p.ConfigDir, p.BackupDir, p.ToolsDir, p.LogsDir, filepath.Join(configDir, "ai")} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return Paths{}, err
		}
	}

	return p, nil
}
