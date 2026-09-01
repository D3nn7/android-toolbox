package app

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// toolKind identifies one of the top-level tools the user can pick between
// on the tool-select screen - not to be confused with the adb/scrcpy
// binaries managed by internal/toolsmanager.
type toolKind int

const (
	toolDevices toolKind = iota
	toolAPKInfo
	toolEmulators
)

var toolOrder = [...]toolKind{toolDevices, toolAPKInfo, toolEmulators}

type toolSelectScreen struct {
	cursor int
}

func (m Model) toolLabel(k toolKind) string {
	switch k {
	case toolAPKInfo:
		return m.text.ToolAPKInfoLabel
	case toolEmulators:
		return m.text.ToolEmulatorsLabel
	default:
		return m.text.ToolDevicesLabel
	}
}

func (m Model) toolDescription(k toolKind) string {
	switch k {
	case toolAPKInfo:
		return m.text.ToolAPKInfoDesc
	case toolEmulators:
		return m.text.ToolEmulatorsDesc
	default:
		return m.text.ToolDevicesDesc
	}
}

// canSwitchTool reports whether ctrl+t should act on the given screen -
// excluded are the startup-only screens (nothing has loaded yet to switch
// away from) and the tool-select screen itself (already there).
func canSwitchTool(current screen) bool {
	switch current {
	case screenSplash, screenHealthcheck, screenHealthFailed, screenToolSelect:
		return false
	default:
		return true
	}
}

// enterToolSelect switches to the tool-select screen. Reachable right after
// the healthcheck passes, and at any time afterward via ctrl+t (see
// Model.Update's global key handling) - so the user can freely change their
// mind about which tool they're using without restarting the app.
func (m Model) enterToolSelect() (Model, tea.Cmd) {
	m.toolSelect = toolSelectScreen{}
	m.current = screenToolSelect
	return m, nil
}

// enterTool switches into whichever tool was picked (or switched to via
// ctrl+t).
func (m Model) enterTool(tool toolKind) (Model, tea.Cmd) {
	switch tool {
	case toolAPKInfo:
		m.apkInfo = newAPKInfoScreen(m)
		m.current = screenAPKInfo
		return m, m.apkInfo.picker.Init()
	case toolEmulators:
		return m.enterEmulatorList()
	default:
		return m.enterDeviceSelect()
	}
}

func (m Model) updateToolSelect(msg tea.Msg) (tea.Model, tea.Cmd) {
	key, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	switch key.String() {
	case "q":
		return m, tea.Quit
	case "up", "k":
		if m.toolSelect.cursor > 0 {
			m.toolSelect.cursor--
		}
	case "down", "j":
		if m.toolSelect.cursor < len(toolOrder)-1 {
			m.toolSelect.cursor++
		}
	case "enter":
		return m.enterTool(toolOrder[m.toolSelect.cursor])
	}
	return m, nil
}

func (m Model) viewToolSelect() string {
	title := m.styles.Title.Render(m.text.ToolSelectTitle)

	var rows strings.Builder
	for i, tool := range toolOrder {
		marker := "  "
		if i == m.toolSelect.cursor {
			marker = m.styles.Highlight.Render("> ")
		}
		fmt.Fprintf(&rows, "%s%s\n", marker, m.toolLabel(tool))
		fmt.Fprintf(&rows, "    %s\n", m.styles.Subtle.Render(m.toolDescription(tool)))
	}

	return title + "\n\n" + rows.String() + "\n" + m.styles.Subtle.Render(m.text.ToolSelectFooter)
}
