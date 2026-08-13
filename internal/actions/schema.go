// Package actions implements the configuration-driven action system: a YAML
// file describes named operations ("Aktionen") that can be executed against
// a device via adb, scrcpy, or an arbitrary shell command.
package actions

// Tool identifies which underlying program an action's command is run with.
type Tool string

const (
	ToolADB    Tool = "adb"
	ToolScrcpy Tool = "scrcpy"
	ToolShell  Tool = "shell"
)

// Param describes a single user-supplied value an action's command can
// reference as "{name}".
type Param struct {
	Name    string `yaml:"name" validate:"required,alphanum"`
	Label   string `yaml:"label"`
	Default string `yaml:"default"`
}

// Action is one entry in actions.yaml.
type Action struct {
	ID          string `yaml:"id" validate:"required"`
	Name        string `yaml:"name" validate:"required"`
	Description string `yaml:"description"`
	Category    string `yaml:"category"`
	Tool        Tool   `yaml:"tool" validate:"required,oneof=adb scrcpy shell"`
	// Command may be empty for tool=scrcpy (use pure defaults) and tool=adb
	// (bare invocation), but is required for tool=shell - enforced in the
	// loader rather than via a struct tag since it depends on Tool's value.
	Command string  `yaml:"command"`
	Params  []Param `yaml:"params"`
	Confirm bool    `yaml:"confirm"`
	// Interactive marks actions that need a real, foreground terminal
	// (e.g. a bare "adb shell" session) instead of streamed/captured
	// output. The TUI hands these off via tea.ExecProcess.
	Interactive bool `yaml:"interactive"`
	// Format hints how the TUI should highlight streamed output instead of
	// showing it as flat, undifferentiated text: "" (default) shows raw
	// lines unchanged; "logcat" colors each line by its priority level;
	// "keyvalue" bolds the label in "key: value"/"key=value" lines (e.g.
	// dumpsys output); "packages" highlights `pm list packages` entries.
	// See internal/output for the actual line classification.
	Format string `yaml:"format" validate:"omitempty,oneof=logcat keyvalue packages"`
	// LivePreview marks quick, side-effect-free read actions (battery
	// status, device info, ...) that the TUI may run automatically just
	// because the user highlighted them in the list, showing the result
	// right away instead of requiring an explicit enter. See
	// LivePreviewEligible for the safety constraints this is subject to.
	LivePreview bool `yaml:"live_preview"`
}

// LivePreviewEligible reports whether this action may be auto-run for a
// live preview merely by being highlighted, with no explicit confirmation.
// That's only ever safe for simple, side-effect-free reads, so it's ignored
// - regardless of the LivePreview flag - for anything that needs user
// input, a destructive-action confirmation, a foreground terminal, or that
// launches scrcpy (which must never open just because the cursor moved
// past it).
func (a Action) LivePreviewEligible() bool {
	return a.LivePreview && len(a.Params) == 0 && !a.Confirm && !a.Interactive && a.Tool != ToolScrcpy
}

// InvalidAction records why a raw YAML entry was rejected.
type InvalidAction struct {
	ID     string
	Reason string
}

// ActionSet is the result of loading actions.yaml: the valid actions plus a
// record of anything that failed validation (so one bad entry never takes
// down the whole file).
type ActionSet struct {
	Actions []Action
	Invalid []InvalidAction
}

// ByCategory groups the valid actions by their Category field (actions with
// no category are grouped under "General"), preserving file order within
// each group.
func (s ActionSet) ByCategory() []CategoryGroup {
	order := []string{}
	groups := map[string][]Action{}
	for _, a := range s.Actions {
		cat := a.Category
		if cat == "" {
			cat = "General"
		}
		if _, ok := groups[cat]; !ok {
			order = append(order, cat)
		}
		groups[cat] = append(groups[cat], a)
	}
	result := make([]CategoryGroup, 0, len(order))
	for _, cat := range order {
		result = append(result, CategoryGroup{Category: cat, Actions: groups[cat]})
	}
	return result
}

// CategoryGroup is a set of actions sharing the same Category.
type CategoryGroup struct {
	Category string
	Actions  []Action
}

// Find returns the action with the given ID, if any.
func (s ActionSet) Find(id string) (Action, bool) {
	for _, a := range s.Actions {
		if a.ID == id {
			return a, true
		}
	}
	return Action{}, false
}
