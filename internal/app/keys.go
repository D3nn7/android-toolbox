package app

import "github.com/charmbracelet/bubbles/key"

// keyMap centralises every keybinding used across screens so the footer
// help text and the actual handling never drift apart.
type keyMap struct {
	Up           key.Binding
	Down         key.Binding
	Select       key.Binding
	Run          key.Binding
	Back         key.Binding
	Quit         key.Binding
	Refresh      key.Binding
	SwitchDevice key.Binding
	Filter       key.Binding
	Confirm      key.Binding
	Cancel       key.Binding
	Tab          key.Binding
	Help         key.Binding
	AIAction     key.Binding
	Backups      key.Binding
	NextCategory key.Binding
	PrevCategory key.Binding
	Settings     key.Binding
	EditAction   key.Binding
	SwitchTool   key.Binding
}

func newKeyMap(t uiText) keyMap {
	return keyMap{
		Up:           key.NewBinding(key.WithKeys("up", "k"), key.WithHelp("↑/k", t.KeyUp)),
		Down:         key.NewBinding(key.WithKeys("down", "j"), key.WithHelp("↓/j", t.KeyDown)),
		Select:       key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", t.KeySelect)),
		Run:          key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", t.KeyRun)),
		Back:         key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", t.KeyBack)),
		Quit:         key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q", t.KeyQuit)),
		Refresh:      key.NewBinding(key.WithKeys("r"), key.WithHelp("r", t.KeyRefresh)),
		SwitchDevice: key.NewBinding(key.WithKeys("ctrl+g"), key.WithHelp("ctrl+g", t.KeySwitchDevice)),
		Filter:       key.NewBinding(key.WithKeys("/"), key.WithHelp("/", t.KeyFilter)),
		Confirm:      key.NewBinding(key.WithKeys("y"), key.WithHelp("y", t.KeyConfirm)),
		Cancel:       key.NewBinding(key.WithKeys("n", "esc"), key.WithHelp("n/esc", t.KeyCancel)),
		Tab:          key.NewBinding(key.WithKeys("tab", "shift+tab"), key.WithHelp("tab", t.KeyNextField)),
		Help:         key.NewBinding(key.WithKeys("?"), key.WithHelp("?", t.KeyHelp)),
		AIAction:     key.NewBinding(key.WithKeys("a"), key.WithHelp("a", t.KeyAIAction)),
		Backups:      key.NewBinding(key.WithKeys("b"), key.WithHelp("b", t.KeyBackups)),
		NextCategory: key.NewBinding(key.WithKeys("tab"), key.WithHelp("tab", t.KeyNextCategory)),
		PrevCategory: key.NewBinding(key.WithKeys("shift+tab"), key.WithHelp("shift+tab", t.KeyPrevCategory)),
		Settings:     key.NewBinding(key.WithKeys("s"), key.WithHelp("s", t.KeySettings)),
		EditAction:   key.NewBinding(key.WithKeys("e"), key.WithHelp("e", t.KeyEditAction)),
		SwitchTool:   key.NewBinding(key.WithKeys("ctrl+t"), key.WithHelp("ctrl+t", t.KeySwitchTool)),
	}
}

// helpBindings implements help.KeyMap over an arbitrary, screen-supplied
// subset of bindings, so every screen can show just what applies to it.
type helpBindings struct {
	bindings []key.Binding
}

func (h helpBindings) ShortHelp() []key.Binding { return h.bindings }
func (h helpBindings) FullHelp() [][]key.Binding {
	return [][]key.Binding{h.bindings}
}
