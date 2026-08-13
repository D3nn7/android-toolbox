package app

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"android-toolbox/internal/buildinfo"
	"android-toolbox/internal/config"
)

// TestAppInfoAppearsOnHealthcheckAndSettings is a regression test for the
// explicit request to show the app's version and repo link somewhere in
// the healthcheck and settings screens.
func TestAppInfoAppearsOnHealthcheckAndSettings(t *testing.T) {
	paths := config.Paths{SettingsFile: filepath.Join(t.TempDir(), "settings.yaml")}
	m := New(context.Background(), paths, config.Settings{}, config.State{}, nil)
	m.width, m.height = 100, 30
	m.health.done = true

	health := m.viewHealthcheck()
	if !strings.Contains(health, buildinfo.Version) {
		t.Fatalf("expected the healthcheck screen to show the app version, got:\n%s", health)
	}
	if !strings.Contains(health, buildinfo.RepoURL) {
		t.Fatalf("expected the healthcheck screen to show the repo URL, got:\n%s", health)
	}

	m.settingsScreen = newSettingsScreen(m)
	m.current = screenSettings
	settings := m.viewSettings()
	if !strings.Contains(settings, buildinfo.Version) {
		t.Fatalf("expected the settings screen to show the app version, got:\n%s", settings)
	}
	if !strings.Contains(settings, buildinfo.RepoURL) {
		t.Fatalf("expected the settings screen to show the repo URL, got:\n%s", settings)
	}
}
