package config

import (
	"os"

	"gopkg.in/yaml.v3"
)

// Settings holds user-configurable application behaviour. It is persisted as
// YAML so it stays easy to hand-edit.
type Settings struct {
	AI      AISettings      `yaml:"ai"`
	Devices DeviceSettings  `yaml:"devices"`
	Scrcpy  ScrcpySettings  `yaml:"scrcpy"`
	Install InstallSettings `yaml:"install"`
	UI      UISettings      `yaml:"ui"`
}

type UISettings struct {
	// Language selects the TUI's display language: "en" (default) or "de".
	// Anything else (including empty/unset) falls back to "en" - see
	// Settings.Language.
	Language string `yaml:"language"`

	// ShowStartupAnimation toggles the animated ASCII splash screen shown
	// while the startup healthcheck runs. Off skips straight to the (also
	// toggleable) healthcheck screen instead.
	ShowStartupAnimation bool `yaml:"show_startup_animation"`

	// ShowHealthcheck toggles whether the healthcheck results screen is
	// shown once every check has passed. Off skips straight to device
	// selection in that case; a failed check is always shown regardless,
	// since the user needs to see what's wrong.
	ShowHealthcheck bool `yaml:"show_healthcheck"`

	// AutoCheckToolUpdates toggles the periodic background check for newer
	// adb/scrcpy builds (internal/toolsmanager, surfaced as a notice - see
	// internal/app/toolsupdate_check.go). This only ever checks, never
	// installs anything by itself; `android-toolbox tools update` (or the
	// Settings screen's "check now" action) always still works regardless
	// of this setting, since that's an explicit user action.
	AutoCheckToolUpdates bool `yaml:"auto_check_tool_updates"`
}

// Language returns the configured UI language, normalized to "en" or "de".
// Any other/empty value defaults to "en" rather than erroring, so a typo or
// pre-i18n settings.yaml never breaks startup.
func (s Settings) Language() string {
	if s.UI.Language == "de" {
		return "de"
	}
	return "en"
}

type AISettings struct {
	Provider string          `yaml:"provider"`
	Claude   ClaudeAISetting `yaml:"claude"`
}

type ClaudeAISetting struct {
	Command        string `yaml:"command"`
	TimeoutSeconds int    `yaml:"timeout_seconds"`
}

type DeviceSettings struct {
	RefreshIntervalSeconds int `yaml:"refresh_interval_seconds"`
}

type ScrcpySettings struct {
	DefaultArgs []string `yaml:"default_args"`
}

type InstallSettings struct {
	AliasName string `yaml:"alias_name"`
}

// Default returns the settings shipped on first run.
func Default() Settings {
	return Settings{
		AI: AISettings{
			Provider: "claude",
			Claude: ClaudeAISetting{
				Command:        "claude",
				TimeoutSeconds: 120,
			},
		},
		Devices: DeviceSettings{
			RefreshIntervalSeconds: 3,
		},
		Scrcpy: ScrcpySettings{
			DefaultArgs: []string{},
		},
		Install: InstallSettings{
			AliasName: "atbx",
		},
		UI: UISettings{
			Language:             "en",
			ShowStartupAnimation: true,
			ShowHealthcheck:      true,
			AutoCheckToolUpdates: true,
		},
	}
}

// LoadSettings reads settings.yaml, seeding it with defaults if it does not
// exist yet.
func LoadSettings(p Paths) (Settings, error) {
	data, err := os.ReadFile(p.SettingsFile)
	if os.IsNotExist(err) {
		s := Default()
		if err := SaveSettings(p, s); err != nil {
			return Settings{}, err
		}
		return s, nil
	}
	if err != nil {
		return Settings{}, err
	}

	s := Default()
	if err := yaml.Unmarshal(data, &s); err != nil {
		return Settings{}, err
	}
	return s, nil
}

// SaveSettings writes settings.yaml.
func SaveSettings(p Paths, s Settings) error {
	data, err := yaml.Marshal(s)
	if err != nil {
		return err
	}
	return os.WriteFile(p.SettingsFile, data, 0o644)
}
