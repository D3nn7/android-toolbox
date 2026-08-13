package config

import (
	"path/filepath"
	"reflect"
	"testing"
)

func testPaths(t *testing.T) Paths {
	t.Helper()
	dir := t.TempDir()
	return Paths{
		ConfigDir:    dir,
		SettingsFile: filepath.Join(dir, "settings.yaml"),
	}
}

func TestLanguageDefaultsToEnglish(t *testing.T) {
	cases := []struct {
		name string
		lang string
		want string
	}{
		{"empty", "", "en"},
		{"english", "en", "en"},
		{"german", "de", "de"},
		{"unknown value falls back", "fr", "en"},
		{"typo falls back", "De", "en"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := Settings{UI: UISettings{Language: tc.lang}}
			if got := s.Language(); got != tc.want {
				t.Errorf("Language() with UI.Language=%q = %q, want %q", tc.lang, got, tc.want)
			}
		})
	}
}

func TestDefaultSettings(t *testing.T) {
	s := Default()
	if s.AI.Provider != "claude" {
		t.Errorf("expected default AI provider to be claude, got %q", s.AI.Provider)
	}
	if s.Language() != "en" {
		t.Errorf("expected default language to be en, got %q", s.Language())
	}
	if !s.UI.ShowStartupAnimation || !s.UI.ShowHealthcheck || !s.UI.AutoCheckToolUpdates {
		t.Errorf("expected all default UI toggles to be on, got %+v", s.UI)
	}
	if s.Install.AliasName != "atbx" {
		t.Errorf("expected default alias to be atbx, got %q", s.Install.AliasName)
	}
}

func TestLoadSettingsSeedsDefaultsOnFirstRun(t *testing.T) {
	p := testPaths(t)

	s, err := LoadSettings(p)
	if err != nil {
		t.Fatalf("LoadSettings returned an error: %v", err)
	}
	if !reflect.DeepEqual(s, Default()) {
		t.Fatalf("expected freshly seeded settings to equal Default(), got %+v", s)
	}

	// The file should now exist and be loadable on a second call too.
	again, err := LoadSettings(p)
	if err != nil {
		t.Fatalf("second LoadSettings returned an error: %v", err)
	}
	if !reflect.DeepEqual(again, s) {
		t.Fatalf("expected the second load to match the first, got %+v vs %+v", again, s)
	}
}

func TestSaveAndLoadSettingsRoundTrip(t *testing.T) {
	p := testPaths(t)

	want := Default()
	want.UI.Language = "de"
	want.AI.Claude.Command = "my-claude"
	want.AI.Claude.TimeoutSeconds = 42

	if err := SaveSettings(p, want); err != nil {
		t.Fatalf("SaveSettings returned an error: %v", err)
	}

	got, err := LoadSettings(p)
	if err != nil {
		t.Fatalf("LoadSettings returned an error: %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("round trip mismatch: got %+v, want %+v", got, want)
	}
}
