package app

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
)

// fieldWizardStep is one step of a fieldWizard: a label (rendered as the
// step's title) plus a standalone huh.Field bound to a *string - the same
// "one huh.Field at a time" idiom settingsScreen/actionEditScreen already
// use for a single value, extended here to a short fixed sequence of them
// (e.g. latitude, then longitude) rather than a multi-field huh.Form, per
// actionEditScreen's stated reasoning: never make the user tab through a
// whole form when stepping through one thing at a time is clearer.
type fieldWizardStep struct {
	Label string
	Field huh.Field
	Value *string
	// Filterable is true for a select step that starts in "type to
	// search" mode (see newSelectStep) - the view renders a hint for it,
	// since huh's own filter-mode UI doesn't otherwise say what typing does.
	Filterable bool
}

// fieldWizard steps through a short, fixed sequence of standalone huh
// fields - reused by the emulator manager's simulation quick-actions/specs
// editor and its create wizard.
type fieldWizard struct {
	Steps []fieldWizardStep
	Index int
}

func newFieldWizard(steps []fieldWizardStep) fieldWizard {
	if len(steps) > 0 {
		steps[0].Field.Focus()
	}
	return fieldWizard{Steps: steps}
}

// Current returns the step currently being edited - only valid while !Done().
func (w fieldWizard) Current() fieldWizardStep { return w.Steps[w.Index] }

// Done reports whether every step has been completed.
func (w fieldWizard) Done() bool { return w.Index >= len(w.Steps) }

// Advance moves to the next step, focusing its field. Callers check Done()
// afterward to see whether the whole wizard just completed.
func (w *fieldWizard) Advance() {
	w.Index++
	if !w.Done() {
		w.Steps[w.Index].Field.Focus()
	}
}

// Update forwards msg to the current step's field. Callers still watch for
// "enter" themselves (same as confirmScreen/actionEditScreen) to decide
// whether to call Advance - a field's own Update return value doesn't
// generically say "the user is done with this one" across Input vs Select.
func (w fieldWizard) Update(msg tea.Msg) (fieldWizard, tea.Cmd) {
	if w.Done() {
		return w, nil
	}
	updated, cmd := w.Steps[w.Index].Field.Update(msg)
	if f, ok := updated.(huh.Field); ok {
		w.Steps[w.Index].Field = f
	}
	return w, cmd
}

// newInputStep builds a free-text fieldWizardStep, pre-filled with def.
func newInputStep(label, def string, validate func(string) error, theme *huh.Theme, width int) fieldWizardStep {
	value := new(string)
	*value = def
	input := huh.NewInput().Title(label).Value(value)
	if validate != nil {
		input = input.Validate(validate)
	}
	field := input.WithKeyMap(huh.NewDefaultKeyMap()).WithTheme(theme).WithWidth(width)
	return fieldWizardStep{Label: label, Field: field, Value: value}
}

// selectStepHeight bounds a select field's own internal viewport. Without
// an explicit Height, huh.Select sizes its viewport to fit *every* option
// unconditionally (see its Height doc comment: "If the number of options
// exceeds the height, the select field will become scrollable" - implying
// the reverse when unset) - for a long list (e.g. dozens of system images)
// that renders far taller than the terminal, and this app's own
// clampToTerminal then silently truncates the overflow with no way to
// scroll to what got cut off, since the field itself never turns on
// scrolling in the first place. A bounded height is what makes the
// viewport (and its arrow-key scrolling, and "/" filtering) actually kick
// in.
const selectStepHeight = 8

// newSelectStep builds a fieldWizardStep choosing among a fixed set of
// options, defaulting to the first one. Every select supports "/" to filter
// regardless (huh's own default keymap already binds it); filterable only
// controls whether the field starts in that filter-typing mode instead of
// plain arrow-key browsing - worth it for a long list (dozens of system
// images) where typing to narrow down beats scrolling, not worth the
// (minor) surprise on a short, already-scannable one (network profile,
// yes/no).
func newSelectStep(label string, options []huh.Option[string], filterable bool, theme *huh.Theme, width int) fieldWizardStep {
	value := new(string)
	if len(options) > 0 {
		*value = options[0].Value
	}
	sel := huh.NewSelect[string]().Title(label).Options(options...).Value(value).
		Height(selectStepHeight).
		Filtering(filterable).
		WithKeyMap(huh.NewDefaultKeyMap()).
		WithTheme(theme).
		WithWidth(width)
	return fieldWizardStep{Label: label, Field: sel, Value: value, Filterable: filterable}
}

// selectStepFiltering reports whether step's field is a *huh.Select
// currently in active filter-typing mode, returning the field itself (so
// callers can turn filtering back off) if so. Screens that bind "esc" to
// exit the whole wizard (see updateEmulatorCreateForm/updateEmulatorWizard)
// need this check first: without it, esc while searching would exit the
// wizard entirely instead of just clearing the search, since a select
// field's own esc handling only clears an *enabled* SetFilter/ClearFilter
// binding, which standalone fields built via newSelectStep never enable.
func selectStepFiltering(step fieldWizardStep) (*huh.Select[string], bool) {
	sel, ok := step.Field.(*huh.Select[string])
	if !ok {
		return nil, false
	}
	return sel, sel.GetFiltering()
}

// stringOptions turns a plain []string into huh.Select options with
// identical label/value.
func stringOptions(values []string) []huh.Option[string] {
	opts := make([]huh.Option[string], len(values))
	for i, v := range values {
		opts[i] = huh.NewOption(v, v)
	}
	return opts
}
