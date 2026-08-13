package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"android-toolbox/internal/actions"
)

// TestEveryBuiltinActionHasAGermanTranslation is a completeness check: if a
// new action is added to actions.default.yaml without a matching entry in
// builtinActionTranslationsDE, it would silently fall back to displaying in
// English even with UI.Language set to "de" - this fails loudly instead.
func TestEveryBuiltinActionHasAGermanTranslation(t *testing.T) {
	// Load needs a real path to read from; pointing it at a file that
	// already contains exactly DefaultActionsYAML's bytes makes it parse
	// that (rather than falling back to writing/reading a seed), giving a
	// real ActionSet without needing a second, ad hoc YAML parse path here.
	path := filepath.Join(t.TempDir(), "actions.yaml")
	if err := os.WriteFile(path, actions.DefaultActionsYAML, 0o644); err != nil {
		t.Fatal(err)
	}
	set, err := actions.Load(path, actions.DefaultActionsYAML)
	if err != nil {
		t.Fatalf("failed to parse DefaultActionsYAML: %v", err)
	}
	if len(set.Actions) == 0 {
		t.Fatal("expected DefaultActionsYAML to parse into at least one action")
	}
	for _, a := range set.Actions {
		if _, ok := builtinActionTranslationsDE[a.ID]; !ok {
			t.Errorf("built-in action %q has no German translation in builtinActionTranslationsDE", a.ID)
		}
	}
}

func TestLocalizeActionTranslatesBuiltinInGerman(t *testing.T) {
	a := actions.Action{ID: "device-reboot", Name: "Reboot", Description: "Restarts the device normally", Category: "Device"}

	got := localizeAction(a, uiTextDE)

	if got.Name != "Neustart" {
		t.Fatalf("Name = %q, want %q", got.Name, "Neustart")
	}
	if got.Description != "Startet das Gerät normal neu" {
		t.Fatalf("Description = %q, want the German translation", got.Description)
	}
	if got.ID != a.ID || got.Category != a.Category {
		t.Fatal("expected ID/Category to be untouched by localizeAction (Category is handled separately at the group level)")
	}
}

func TestLocalizeActionTranslatesParamLabels(t *testing.T) {
	a := actions.Action{
		ID: "file-pull", Name: "Pull File from Device",
		Params: []actions.Param{{Name: "remote_path", Label: "Path on the device"}},
	}

	got := localizeAction(a, uiTextDE)

	if got.Params[0].Label != "Pfad auf dem Gerät" {
		t.Fatalf("param label = %q, want the German translation", got.Params[0].Label)
	}
}

func TestLocalizeActionLeavesUserActionsUnchanged(t *testing.T) {
	a := actions.Action{ID: "my-custom-thing", Name: "My Custom Thing", Description: "Does something custom"}

	got := localizeAction(a, uiTextDE)

	if got.Name != a.Name || got.Description != a.Description || got.ID != a.ID {
		t.Fatalf("expected a user-created action to pass through unchanged even in German mode, got %+v", got)
	}
}

func TestLocalizeActionNoOpInEnglish(t *testing.T) {
	a := actions.Action{ID: "device-reboot", Name: "Reboot", Description: "Restarts the device normally"}

	got := localizeAction(a, uiTextEN)

	if got.Name != a.Name || got.Description != a.Description {
		t.Fatalf("expected English mode to leave even a known built-in action unchanged, got %+v", got)
	}
}

func TestCategoryDisplayLabelTranslatesBuiltinCategories(t *testing.T) {
	if got := categoryDisplayLabel("Device", uiTextDE); got != "Gerät" {
		t.Fatalf("categoryDisplayLabel(Device, de) = %q, want Gerät", got)
	}
	if got := categoryDisplayLabel("Device", uiTextEN); got != "Device" {
		t.Fatalf("categoryDisplayLabel(Device, en) = %q, want Device unchanged", got)
	}
	if got := categoryDisplayLabel("MyCustomCategory", uiTextDE); got != "MyCustomCategory" {
		t.Fatalf("expected an unrecognized (user-created) category to pass through unchanged, got %q", got)
	}
	if got := categoryDisplayLabel("", uiTextDE); got != uiTextDE.CategoryAll {
		t.Fatalf("categoryDisplayLabel(\"\", de) = %q, want the \"Alle\" sentinel", got)
	}
	if got := categoryDisplayLabel("General", uiTextDE); got != uiTextDE.CategoryFallback {
		t.Fatalf("categoryDisplayLabel(General, de) = %q, want the fallback label", got)
	}
}

// TestBuildActionItemsAppliesTranslationsEndToEnd proves the whole pipeline
// - ByCategory's grouping key, the pill label, and each item's Title/
// Description - actually reflects German translations for a built-in
// action when the dashboard is built in German mode.
func TestBuildActionItemsAppliesTranslationsEndToEnd(t *testing.T) {
	set := actions.ActionSet{Actions: []actions.Action{
		{ID: "device-reboot", Name: "Reboot", Description: "Restarts the device normally", Category: "Device", Tool: actions.ToolADB, Command: "reboot", Confirm: true},
	}}

	items := buildActionItems(set, "", uiTextDE)
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
	item := items[0].(actionItem)
	if item.Title() != "Neustart" {
		t.Fatalf("Title() = %q, want %q", item.Title(), "Neustart")
	}
	if !strings.Contains(item.Description(), "Gerät") {
		t.Fatalf("expected the translated category (Gerät) in the description, got %q", item.Description())
	}
}

// TestBuildActionItemsMarksOnlyUserCreatedActionsAsEditable is a regression
// test for the editable badge: a built-in ID (from actions.default.yaml)
// must never show it, even if the user's actions.yaml happens to redefine
// one under the same category - only IDs actually absent from the shipped
// defaults count as user-created.
func TestBuildActionItemsMarksOnlyUserCreatedActionsAsEditable(t *testing.T) {
	set := actions.ActionSet{Actions: []actions.Action{
		{ID: "device-reboot", Name: "Reboot", Category: "Device", Tool: actions.ToolADB, Command: "reboot"},
		{ID: "my-custom-action", Name: "My Custom Action", Category: "Device", Tool: actions.ToolShell, Command: "echo hi"},
	}}

	items := buildActionItems(set, "", uiTextEN)
	if len(items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(items))
	}

	builtin := items[0].(actionItem)
	if builtin.editable {
		t.Fatal("expected the built-in action to not be marked editable")
	}
	if strings.Contains(builtin.Title(), uiTextEN.ActionEditableBadge) {
		t.Fatalf("expected the built-in action's title to not carry the editable badge, got %q", builtin.Title())
	}

	custom := items[1].(actionItem)
	if !custom.editable {
		t.Fatal("expected the user-created action to be marked editable")
	}
	if !strings.Contains(custom.Title(), uiTextEN.ActionEditableBadge) {
		t.Fatalf("expected the user-created action's title to carry the editable badge, got %q", custom.Title())
	}
}
