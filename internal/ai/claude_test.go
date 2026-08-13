package ai

import (
	"strings"
	"testing"

	"android-toolbox/internal/actions"
)

func TestExtractJSONObjectPlain(t *testing.T) {
	in := `{"id":"x","name":"y"}`
	if got := extractJSONObject(in); got != in {
		t.Fatalf("got %q, want %q", got, in)
	}
}

func TestExtractJSONObjectStripsMarkdownFence(t *testing.T) {
	in := "Here you go:\n```json\n{\"id\":\"x\",\"name\":\"y\"}\n```\nHope that helps!"
	want := `{"id":"x","name":"y"}`
	if got := extractJSONObject(in); got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestParseActionDraftValid(t *testing.T) {
	text := `{"id":"battery-info","name":"Akku","description":"desc","category":"Geraet","tool":"adb","command":"shell dumpsys battery","params":[],"confirm":false,"interactive":false}`
	d, err := parseActionDraft(text)
	if err != nil {
		t.Fatalf("parseActionDraft error: %v", err)
	}
	if d.ID != "battery-info" || d.Tool != "adb" {
		t.Fatalf("unexpected draft: %+v", d)
	}
}

func TestParseActionDraftInvalidJSON(t *testing.T) {
	_, err := parseActionDraft("this is not json at all")
	if err == nil {
		t.Fatal("expected error for non-JSON response")
	}
}

func TestActionDraftToAction(t *testing.T) {
	d := ActionDraft{
		ID: "x", Name: "X", Tool: "shell", Command: "echo hi",
		Params: []Param{{Name: "p", Label: "P", Default: "d"}},
	}
	a := d.ToAction()
	if string(a.Tool) != "shell" || len(a.Params) != 1 || a.Params[0].Name != "p" {
		t.Fatalf("unexpected conversion: %+v", a)
	}
}

func TestActionDraftFromActionRoundTrips(t *testing.T) {
	a := actions.Action{
		ID: "x", Name: "X", Description: "desc", Category: "Cat",
		Tool: actions.ToolShell, Command: "echo hi", Confirm: true, Interactive: true,
		Params: []actions.Param{{Name: "p", Label: "P", Default: "d"}},
	}
	d := ActionDraftFromAction(a)
	back := d.ToAction()
	if back.ID != a.ID || back.Name != a.Name || back.Description != a.Description ||
		back.Category != a.Category || back.Tool != a.Tool || back.Command != a.Command ||
		back.Confirm != a.Confirm || back.Interactive != a.Interactive {
		t.Fatalf("round trip lost data: got %+v, want %+v", back, a)
	}
	if len(back.Params) != 1 || back.Params[0].Name != "p" || back.Params[0].Label != "P" || back.Params[0].Default != "d" {
		t.Fatalf("unexpected params after round trip: %+v", back.Params)
	}
}

func TestBuildUserPromptIncludesExistingActionAsEditContext(t *testing.T) {
	d := ActionDraftFromAction(actions.Action{ID: "keep-this-id", Name: "Old Name", Tool: actions.ToolShell, Command: "echo old"})
	prompt := buildUserPrompt(GenerateRequest{UserPrompt: "rename it", ExistingAction: &d})

	if !strings.Contains(prompt, `"id":"keep-this-id"`) {
		t.Fatalf("expected the prompt to embed the existing action's id, got: %s", prompt)
	}
	if !strings.Contains(prompt, "rename it") {
		t.Fatalf("expected the prompt to still include the user's request, got: %s", prompt)
	}
}

func TestBuildUserPromptOmitsExistingIDsWhenEditingAnAction(t *testing.T) {
	d := ActionDraftFromAction(actions.Action{ID: "keep-this-id", Tool: actions.ToolShell, Command: "echo old"})
	prompt := buildUserPrompt(GenerateRequest{UserPrompt: "rename it", ExistingIDs: []string{"other-1", "other-2"}, ExistingAction: &d})

	if strings.Contains(prompt, "other-1") {
		t.Fatalf("expected the unrelated existing-IDs list to be skipped once an existing action is being edited, got: %s", prompt)
	}
}
