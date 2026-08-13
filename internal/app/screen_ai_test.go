package app

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// newTestAIModel mirrors what the dashboard's "a" key handler does when
// entering the AI screen: construct it, then explicitly focus its textarea
// (bubbles/textarea ignores all key input, typing included, while
// unfocused - see textarea.Model.Update's `if !m.focus` guard).
func newTestAIModel() Model {
	m := Model{styles: newStyles(), text: uiTextEN, current: screenAI}
	m.ai = newAIScreen(m.text)
	m.ai.textarea.Focus()
	return m
}

func typeRune(m Model, r rune) Model {
	updated, _ := m.updateAI(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	return updated.(Model)
}

// TestAIInputStageForwardsTypingToTextarea is a regression test for a real
// bug: updateAI's tea.KeyMsg case had an unconditional `return m, nil`
// right after its per-stage switch, which fired for every key press
// regardless of whether the nested switch actually matched anything. Since
// aiStageInput only explicitly handles "esc" and "ctrl+d", every other key
// - i.e. all normal typing - was swallowed before it could ever reach the
// textarea's own Update() call, so the input box looked dead: you could
// open "create AI action" but literally could not type into it.
func TestAIInputStageForwardsTypingToTextarea(t *testing.T) {
	m := newTestAIModel()

	for _, r := range "hi" {
		m = typeRune(m, r)
	}

	if got := m.ai.textarea.Value(); got != "hi" {
		t.Fatalf("expected typed text to reach the textarea, got %q", got)
	}
}

func TestAIInputStageEscStillReturnsToDashboardWithoutTyping(t *testing.T) {
	m := newTestAIModel()

	updated, _ := m.updateAI(tea.KeyMsg{Type: tea.KeyEsc})
	m = updated.(Model)

	if m.current != screenDashboard {
		t.Fatalf("expected esc to return to the dashboard, current = %v", m.current)
	}
	if m.ai.textarea.Value() != "" {
		t.Fatalf("expected esc to not be forwarded as text, got %q", m.ai.textarea.Value())
	}
}

func TestAIPreviewStageDoesNotForwardKeysToTextarea(t *testing.T) {
	m := newTestAIModel()
	m.ai.stage = aiStagePreview
	m.ai.saveDialog, m.ai.saveAnswer = newSaveActionDialog(m.text, androidHuhTheme(), 60, false)

	m = typeRune(m, 'x')

	// In the preview stage there is no textarea to type into - "x" isn't a
	// recognized preview key (y/n/r), so this must be a no-op rather than
	// somehow reaching the (stage-input-only) textarea.
	if m.ai.textarea.Value() != "" {
		t.Fatalf("expected preview-stage keys to never reach the textarea, got %q", m.ai.textarea.Value())
	}
	if m.ai.stage != aiStagePreview {
		t.Fatalf("expected stage to remain aiStagePreview, got %v", m.ai.stage)
	}
}
