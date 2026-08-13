package healthcheck

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"android-toolbox/internal/actions"
	"android-toolbox/internal/config"
)

func TestReportHasFailures(t *testing.T) {
	cases := []struct {
		name string
		r    Report
		want bool
	}{
		{"empty", Report{}, false},
		{"all ok", Report{Results: []Result{{Severity: OK}, {Severity: Warn}}}, false},
		{"one failure", Report{Results: []Result{{Severity: OK}, {Severity: Fail}}}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.r.HasFailures(); got != tc.want {
				t.Errorf("HasFailures() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestCheckConfigDir(t *testing.T) {
	if got := checkConfigDir(config.Paths{}); got.Severity != Fail {
		t.Errorf("expected an empty ConfigDir to fail, got %+v", got)
	}
	if got := checkConfigDir(config.Paths{ConfigDir: "/some/dir"}); got.Severity != OK {
		t.Errorf("expected a non-empty ConfigDir to pass, got %+v", got)
	}
}

func TestCheckActionsFileSeedsAndPasses(t *testing.T) {
	dir := t.TempDir()
	paths := config.Paths{ActionsFile: filepath.Join(dir, "actions.yaml")}

	got := checkActionsFile(paths)
	if got.Severity != OK {
		t.Fatalf("expected a freshly seeded actions.yaml to pass, got %+v", got)
	}
	if !strings.Contains(got.Detail, "actions loaded") {
		t.Errorf("expected the detail to mention loaded actions, got %q", got.Detail)
	}
}

func TestCheckActionsFileReportsInvalidEntries(t *testing.T) {
	dir := t.TempDir()
	actionsFile := filepath.Join(dir, "actions.yaml")
	invalidYAML := []byte(`
- id: bad-shell
  name: "Missing command"
  description: "shell action with no command"
  tool: shell
  command: ""
`)
	if err := os.WriteFile(actionsFile, invalidYAML, 0o644); err != nil {
		t.Fatalf("WriteFile returned an error: %v", err)
	}

	got := checkActionsFile(config.Paths{ActionsFile: actionsFile})
	if got.Severity != Warn {
		t.Fatalf("expected invalid entries to be a warning, got %+v", got)
	}
	if !strings.Contains(got.Detail, "invalid action") {
		t.Errorf("expected the detail to mention invalid actions, got %q", got.Detail)
	}
}

func TestFormatActionCount(t *testing.T) {
	if got := formatActionCount(0); got != "0 actions loaded" {
		t.Errorf("formatActionCount(0) = %q", got)
	}
	if got := formatActionCount(1); got != "1 action loaded" {
		t.Errorf("formatActionCount(1) = %q", got)
	}
	if got := formatActionCount(5); got != "5 actions loaded" {
		t.Errorf("formatActionCount(5) = %q", got)
	}
}

func TestFormatInvalidActions(t *testing.T) {
	got := formatInvalidActions([]actions.InvalidAction{
		{ID: "a", Reason: "bad"},
		{ID: "b", Reason: "worse"},
	})
	want := "2 invalid action(s): a (bad); b (worse)"
	if got != want {
		t.Errorf("formatInvalidActions() = %q, want %q", got, want)
	}
}

func TestItoa(t *testing.T) {
	cases := map[int]string{0: "0", 1: "1", 42: "42", -7: "-7"}
	for n, want := range cases {
		if got := itoa(n); got != want {
			t.Errorf("itoa(%d) = %q, want %q", n, got, want)
		}
	}
}

func TestCheckAIProviderNotOnPath(t *testing.T) {
	emptyPathDir := t.TempDir()
	t.Setenv("PATH", emptyPathDir)

	got := checkAIProvider(config.Settings{AI: config.AISettings{Provider: "claude"}})
	if got.Severity != Warn {
		t.Fatalf("expected a missing AI CLI to be a warning, got %+v", got)
	}
	if !strings.Contains(got.Name, "claude") {
		t.Errorf("expected the check name to mention the provider, got %q", got.Name)
	}
}

func TestRunProducesFiveResults(t *testing.T) {
	dir := t.TempDir()
	emptyPathDir := t.TempDir()
	t.Setenv("PATH", emptyPathDir)

	paths := config.Paths{
		ConfigDir:   dir,
		ActionsFile: filepath.Join(dir, "actions.yaml"),
		ToolsDir:    filepath.Join(dir, "tools"),
	}
	report := Run(context.Background(), paths, config.Default())
	if len(report.Results) != 5 {
		t.Fatalf("expected 5 results (config dir, actions, adb, scrcpy, AI provider), got %d", len(report.Results))
	}
}
