package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"android-toolbox/internal/config"
)

func newTestResetPaths(t *testing.T) config.Paths {
	t.Helper()
	base := t.TempDir()
	configDir := filepath.Join(base, "android-toolbox")
	return config.Paths{
		ConfigDir:    configDir,
		ActionsFile:  filepath.Join(configDir, "actions.yaml"),
		SettingsFile: filepath.Join(configDir, "settings.yaml"),
		StateFile:    filepath.Join(configDir, "state.json"),
		BackupDir:    filepath.Join(configDir, ".backup"),
		ToolsDir:     filepath.Join(configDir, "tools"),
		LogsDir:      filepath.Join(configDir, "logs"),
		AIPromptFile: filepath.Join(configDir, "ai", "system_prompt.md"),
	}
}

func TestConfirmYesNo(t *testing.T) {
	cases := map[string]bool{
		"y\n":   true,
		"Y\n":   true,
		"y":     true,
		"n\n":   false,
		"":      false,
		"yes\n": false, // only the exact "y" shortcut counts, matching the "ai" command's convention
	}
	for input, want := range cases {
		if got := confirmYesNo(strings.NewReader(input)); got != want {
			t.Errorf("confirmYesNo(%q) = %v, want %v", input, got, want)
		}
	}
}

// TestRunDangerousResetWipesAndReseeds is a regression test for the core
// promise of this command: whatever was there before (custom actions,
// stray files anywhere under the config dir) is actually gone afterward,
// and default settings/actions exist again - all confined to a temp
// directory, never the real OS user-config dir, and with a fake tool
// "fetch" so the test never touches the network.
func TestRunDangerousResetWipesAndReseeds(t *testing.T) {
	paths := newTestResetPaths(t)
	if err := ensureConfigDirs(paths); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(paths.ActionsFile, []byte("- id: my-custom-action\n  name: Custom\n  tool: shell\n  command: echo hi\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	strayFile := filepath.Join(paths.ConfigDir, "leftover.txt")
	if err := os.WriteFile(strayFile, []byte("should not survive"), 0o644); err != nil {
		t.Fatal(err)
	}

	var fetchCalled bool
	var fetchToolsDir string
	fakeFetch := func(ctx context.Context, toolsDir string, progress func(string)) error {
		fetchCalled = true
		fetchToolsDir = toolsDir
		progress("fake fetch progress")
		return nil
	}

	var out bytes.Buffer
	if err := runDangerousReset(context.Background(), &out, paths, fakeFetch); err != nil {
		t.Fatalf("runDangerousReset returned an error: %v\noutput:\n%s", err, out.String())
	}

	if _, err := os.Stat(strayFile); !os.IsNotExist(err) {
		t.Fatalf("expected the old config directory (and its stray file) to be wiped, stat err = %v", err)
	}

	data, err := os.ReadFile(paths.ActionsFile)
	if err != nil {
		t.Fatalf("expected a fresh actions.yaml to be seeded: %v", err)
	}
	if strings.Contains(string(data), "my-custom-action") {
		t.Fatal("expected the custom action to be gone, seeded with defaults instead")
	}

	if _, err := os.Stat(paths.SettingsFile); err != nil {
		t.Fatalf("expected a fresh settings.yaml to be seeded: %v", err)
	}

	if !fetchCalled {
		t.Fatal("expected the tool fetch step to run as part of the reset")
	}
	if fetchToolsDir != paths.ToolsDir {
		t.Fatalf("expected fetch to target %q, got %q", paths.ToolsDir, fetchToolsDir)
	}
}

// TestRunDangerousResetPropagatesFetchError proves a failed re-download is
// surfaced to the caller rather than silently ignored - the user would
// otherwise believe the reset fully succeeded while actually being left
// without adb/scrcpy.
func TestRunDangerousResetPropagatesFetchError(t *testing.T) {
	paths := newTestResetPaths(t)
	failingFetch := func(ctx context.Context, toolsDir string, progress func(string)) error {
		return context.DeadlineExceeded
	}

	var out bytes.Buffer
	err := runDangerousReset(context.Background(), &out, paths, failingFetch)
	if err == nil {
		t.Fatal("expected a fetch failure to be returned as an error")
	}
	if !strings.Contains(err.Error(), "reinstalling") {
		t.Fatalf("expected the error to mention the failed reinstall step, got: %v", err)
	}
}
