package app

import (
	"context"
	"strings"
	"testing"

	"android-toolbox/internal/config"
)

// TestDefaultLanguageIsEnglish is a regression test for the explicit
// requirement that the TUI defaults to English: an unset/empty
// Settings.UI.Language (e.g. a pre-i18n settings.yaml, or config.Settings{}
// as used throughout this package's other tests) must resolve to the
// English string table, not German.
func TestDefaultLanguageIsEnglish(t *testing.T) {
	m := New(context.Background(), config.Paths{}, config.Settings{}, config.State{}, nil)

	if got, want := m.settings.Language(), "en"; got != want {
		t.Fatalf("Settings.Language() = %q, want %q", got, want)
	}
	if m.text.RunHint != uiTextEN.RunHint {
		t.Fatalf("expected the default-language model to use uiTextEN, got RunHint = %q", m.text.RunHint)
	}
}

// TestLanguageSettingSwitchesRenderedText proves the i18n mechanism actually
// changes what's on screen, not just which uiText fields exist: building
// the Model with UI.Language "de" must render German chrome (e.g. the
// healthcheck screen's spinner caption), and "en" (or anything else) must
// render English.
func TestLanguageSettingSwitchesRenderedText(t *testing.T) {
	settingsDE := config.Settings{UI: config.UISettings{Language: "de"}}
	mDE := New(context.Background(), config.Paths{}, settingsDE, config.State{}, nil)
	outDE := mDE.viewHealthcheck()
	if !strings.Contains(outDE, "wird geprüft") {
		t.Fatalf("expected German healthcheck text with UI.Language=de, got:\n%s", outDE)
	}

	settingsEN := config.Settings{UI: config.UISettings{Language: "en"}}
	mEN := New(context.Background(), config.Paths{}, settingsEN, config.State{}, nil)
	outEN := mEN.viewHealthcheck()
	if !strings.Contains(outEN, "Checking environment") {
		t.Fatalf("expected English healthcheck text with UI.Language=en, got:\n%s", outEN)
	}

	settingsUnknown := config.Settings{UI: config.UISettings{Language: "fr"}}
	mUnknown := New(context.Background(), config.Paths{}, settingsUnknown, config.State{}, nil)
	outUnknown := mUnknown.viewHealthcheck()
	if !strings.Contains(outUnknown, "Checking environment") {
		t.Fatalf("expected an unrecognized language to fall back to English, got:\n%s", outUnknown)
	}
}
