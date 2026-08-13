package actions

import (
	_ "embed"
	"fmt"
	"os"
	"strings"

	"github.com/go-playground/validator/v10"
	"gopkg.in/yaml.v3"
)

//go:embed actions.default.yaml
var DefaultActionsYAML []byte

var validate = validator.New(validator.WithRequiredStructEnabled())

// validateAction runs struct-tag validation plus the one rule that depends
// on cross-field state: a shell action needs an actual command to run.
func validateAction(a Action) error {
	if err := validate.Struct(a); err != nil {
		return err
	}
	if a.Tool == ToolShell && strings.TrimSpace(a.Command) == "" {
		return fmt.Errorf("tool 'shell' requires a non-empty command")
	}
	return nil
}

// Load reads and validates the action set at path, seeding it with seedYAML
// if the file does not exist yet. Individual invalid entries are collected
// in ActionSet.Invalid rather than failing the whole load; only a malformed
// YAML document or an unreadable/unwritable file returns an error.
func Load(path string, seedYAML []byte) (ActionSet, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		if err := os.WriteFile(path, seedYAML, 0o644); err != nil {
			return ActionSet{}, fmt.Errorf("could not create actions.yaml: %w", err)
		}
		data = seedYAML
	} else if err != nil {
		return ActionSet{}, fmt.Errorf("could not read actions.yaml: %w", err)
	}

	var raw []Action
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return ActionSet{}, fmt.Errorf("actions.yaml is not valid YAML: %w", err)
	}

	var set ActionSet
	seenIDs := map[string]bool{}
	for i, a := range raw {
		label := a.ID
		if label == "" {
			label = fmt.Sprintf("#%d (%s)", i+1, a.Name)
		}
		if err := validateAction(a); err != nil {
			set.Invalid = append(set.Invalid, InvalidAction{ID: label, Reason: err.Error()})
			continue
		}
		if seenIDs[a.ID] {
			set.Invalid = append(set.Invalid, InvalidAction{ID: label, Reason: "duplicate ID"})
			continue
		}
		seenIDs[a.ID] = true
		set.Actions = append(set.Actions, a)
	}

	return set, nil
}

// Save writes the given actions back to path as YAML. Callers that want a
// pre-write backup should snapshot the file themselves (see internal/backup)
// before calling Save.
func Save(path string, actions []Action) error {
	data, err := yaml.Marshal(actions)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

// Append adds a new action to the file at path, rejecting duplicate IDs.
func Append(path string, a Action) error {
	set, err := Load(path, DefaultActionsYAML)
	if err != nil {
		return err
	}
	if _, exists := set.Find(a.ID); exists {
		return fmt.Errorf("action with ID %q already exists", a.ID)
	}
	if err := validateAction(a); err != nil {
		return fmt.Errorf("invalid action: %w", err)
	}
	return Save(path, append(set.Actions, a))
}

// Update replaces the action with updated.ID in the file at path, keeping
// every other action and their relative order unchanged. Returns an error
// if no action with that ID exists yet (use Append for a genuinely new
// one) or if updated itself doesn't validate.
func Update(path string, updated Action) error {
	set, err := Load(path, DefaultActionsYAML)
	if err != nil {
		return err
	}
	if err := validateAction(updated); err != nil {
		return fmt.Errorf("invalid action: %w", err)
	}
	for i, a := range set.Actions {
		if a.ID == updated.ID {
			set.Actions[i] = updated
			return Save(path, set.Actions)
		}
	}
	return fmt.Errorf("action with ID %q does not exist", updated.ID)
}

// defaultActionIDs parses DefaultActionsYAML directly (bypassing Load's
// file-path handling, since there is no path here - this is the embedded
// seed itself) into just the set of IDs it ships.
func defaultActionIDs() map[string]bool {
	var raw []Action
	// DefaultActionsYAML is embedded and validated by this package's own
	// tests, so a parse failure here would mean a broken build, not a
	// runtime condition worth surfacing as an error.
	_ = yaml.Unmarshal(DefaultActionsYAML, &raw)
	ids := make(map[string]bool, len(raw))
	for _, a := range raw {
		ids[a.ID] = true
	}
	return ids
}

// IsBuiltinID reports whether id belongs to one of the actions shipped in
// DefaultActionsYAML, as opposed to one the user added later (by hand, or
// via the AI feature). Used to decide which actions are safe to consider
// "editable" in the TUI - built-in ones are meant to stay as shipped.
func IsBuiltinID(id string) bool {
	return defaultActionIDs()[id]
}
