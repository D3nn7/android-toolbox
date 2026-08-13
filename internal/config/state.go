package config

import (
	"encoding/json"
	"os"
	"time"
)

// State tracks one-time decisions the app must not ask about again.
type State struct {
	FirstRunCompletedAt  string `json:"first_run_completed_at,omitempty"`
	PathInstallAsked     bool   `json:"path_install_asked"`
	PathInstalled        bool   `json:"path_installed"`
	PathInstallDecidedAt string `json:"path_install_decided_at,omitempty"`

	// Self-update check cache (internal/selfupdate, TUI startup notice) -
	// lets the app show "a new version is available" without re-querying
	// GitHub's release API on every single launch.
	LastUpdateCheckAt  string `json:"last_update_check_at,omitempty"`
	LatestKnownVersion string `json:"latest_known_version,omitempty"`

	// Third-party tool (adb/scrcpy) update check cache - same idea as the
	// self-update one above, just against Google's/scrcpy's own release
	// info instead of this project's.
	LastToolsUpdateCheckAt string `json:"last_tools_update_check_at,omitempty"`
	ADBLatestKnown         string `json:"adb_latest_known,omitempty"`
	ScrcpyLatestKnown      string `json:"scrcpy_latest_known,omitempty"`
}

// IsFirstRun reports whether the app has never completed its first-run flow.
func (s State) IsFirstRun() bool {
	return s.FirstRunCompletedAt == ""
}

// LoadState reads state.json, returning a zero-value State if it does not
// exist yet (i.e. this is the first run).
func LoadState(p Paths) (State, error) {
	data, err := os.ReadFile(p.StateFile)
	if os.IsNotExist(err) {
		return State{}, nil
	}
	if err != nil {
		return State{}, err
	}
	var s State
	if err := json.Unmarshal(data, &s); err != nil {
		return State{}, err
	}
	return s, nil
}

// SaveState writes state.json.
func SaveState(p Paths, s State) error {
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(p.StateFile, data, 0o644)
}

// MarkFirstRunComplete stamps the current time as the first-run completion
// time, persisting the change.
func MarkFirstRunComplete(p Paths, s State) (State, error) {
	s.FirstRunCompletedAt = time.Now().UTC().Format(time.RFC3339)
	return s, SaveState(p, s)
}
