package actions

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadSeedsDefaultOnFirstRun(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "actions.yaml")

	set, err := Load(path, DefaultActionsYAML)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if len(set.Actions) == 0 {
		t.Fatal("expected default actions to be seeded, got none")
	}
	if len(set.Invalid) != 0 {
		t.Fatalf("expected no invalid default actions, got %v", set.Invalid)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected actions.yaml to be created: %v", err)
	}
}

func TestLoadRejectsInvalidEntriesWithoutFailingWholeFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "actions.yaml")

	yamlContent := []byte(`
- id: good-one
  name: "Good"
  tool: adb
  command: "shell echo hi"
- id: bad-tool
  name: "Bad"
  tool: nonsense
  command: "whatever"
- name: "Missing ID"
  tool: adb
  command: "whatever"
- id: good-one
  name: "Duplicate ID"
  tool: adb
  command: "shell echo dup"
`)
	if err := os.WriteFile(path, yamlContent, 0o644); err != nil {
		t.Fatal(err)
	}

	set, err := Load(path, DefaultActionsYAML)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if len(set.Actions) != 1 {
		t.Fatalf("expected exactly 1 valid action, got %d: %+v", len(set.Actions), set.Actions)
	}
	// bad-tool (invalid tool), the entry missing an ID, and the second
	// "good-one" (duplicate ID) are all rejected -> 3 invalid entries.
	if len(set.Invalid) != 3 {
		t.Fatalf("expected exactly 3 invalid entries, got %d: %+v", len(set.Invalid), set.Invalid)
	}
}

func TestByCategoryGroupsAndPreservesOrder(t *testing.T) {
	set := ActionSet{Actions: []Action{
		{ID: "a", Category: "Logs"},
		{ID: "b", Category: "Apps"},
		{ID: "c", Category: "Logs"},
		{ID: "d"},
	}}
	groups := set.ByCategory()
	if len(groups) != 3 {
		t.Fatalf("expected 3 groups, got %d: %+v", len(groups), groups)
	}
	if groups[0].Category != "Logs" || len(groups[0].Actions) != 2 {
		t.Fatalf("unexpected first group: %+v", groups[0])
	}
	if groups[2].Category != "General" {
		t.Fatalf("expected uncategorised group named General, got %q", groups[2].Category)
	}
}

func TestAppendRejectsDuplicateID(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "actions.yaml")
	if _, err := Load(path, DefaultActionsYAML); err != nil {
		t.Fatal(err)
	}

	err := Append(path, Action{ID: "logcat-snapshot", Name: "dup", Tool: ToolADB, Command: "logcat"})
	if err == nil {
		t.Fatal("expected error when appending duplicate ID")
	}
}

func TestIsBuiltinID(t *testing.T) {
	if !IsBuiltinID("logcat-snapshot") {
		t.Fatal("expected logcat-snapshot (shipped in actions.default.yaml) to be reported as built-in")
	}
	if IsBuiltinID("my-custom-action") {
		t.Fatal("expected an unrecognized ID to be reported as not built-in")
	}
}

func TestUpdateReplacesExistingActionInPlace(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "actions.yaml")
	if err := Append(path, Action{ID: "custom-1", Name: "Original", Tool: ToolShell, Command: "echo original"}); err != nil {
		t.Fatal(err)
	}

	updated := Action{ID: "custom-1", Name: "Renamed", Tool: ToolShell, Command: "echo renamed"}
	if err := Update(path, updated); err != nil {
		t.Fatalf("Update returned error: %v", err)
	}

	set, err := Load(path, DefaultActionsYAML)
	if err != nil {
		t.Fatal(err)
	}
	got, ok := set.Find("custom-1")
	if !ok {
		t.Fatal("expected custom-1 to still exist after Update")
	}
	if got.Name != "Renamed" || got.Command != "echo renamed" {
		t.Fatalf("expected the action to be replaced, got %+v", got)
	}
	count := 0
	for _, a := range set.Actions {
		if a.ID == "custom-1" {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("expected Update to replace in place rather than add a second entry, found %d entries for custom-1", count)
	}
}

func TestUpdateRejectsUnknownID(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "actions.yaml")
	if _, err := Load(path, DefaultActionsYAML); err != nil {
		t.Fatal(err)
	}

	err := Update(path, Action{ID: "does-not-exist", Name: "X", Tool: ToolShell, Command: "echo hi"})
	if err == nil {
		t.Fatal("expected an error when updating a nonexistent ID")
	}
}

func TestUpdateRejectsInvalidAction(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "actions.yaml")
	if err := Append(path, Action{ID: "custom-2", Name: "Original", Tool: ToolShell, Command: "echo hi"}); err != nil {
		t.Fatal(err)
	}

	// tool: shell requires a non-empty command.
	err := Update(path, Action{ID: "custom-2", Name: "Broken", Tool: ToolShell, Command: ""})
	if err == nil {
		t.Fatal("expected an error when updating with an invalid action")
	}
}
