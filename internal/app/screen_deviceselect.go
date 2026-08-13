package app

import (
	"context"
	"fmt"
	"time"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"

	"android-toolbox/internal/adb"
	"android-toolbox/internal/config"
)

// deviceItem carries its state's already-translated label as stateLabel
// rather than translating lazily in Description(): that method comes from
// bubbles/list's Item interface, so it can't take a uiText parameter -
// resolving the label once, when the item is built (see
// updateDeviceSelect's devicesRefreshedMsg case), is what lets it stay
// language-aware without a package-level "current language" global.
type deviceItem struct {
	d          adb.Device
	stateLabel string
}

func (i deviceItem) Title() string {
	label := i.d.Model
	if label == "" {
		label = i.d.Serial
	}
	return fmt.Sprintf("%s (%s)", label, i.d.Serial)
}
func (i deviceItem) Description() string { return i.stateLabel }
func (i deviceItem) FilterValue() string { return i.d.Serial + " " + i.d.Model }

// describeDeviceState translates an adb device state into its display
// label.
func describeDeviceState(state string, t uiText) string {
	switch state {
	case "device":
		return t.DeviceStateConnected
	case "unauthorized":
		return t.DeviceStateUnauthAuth
	case "offline":
		return t.DeviceStateOffline
	default:
		return state
	}
}

type deviceSelectScreen struct {
	list     list.Model
	err      error
	interval time.Duration
}

func newDeviceSelectScreen(ctx context.Context, client *adb.Client, settings config.Settings) (deviceSelectScreen, tea.Cmd) {
	l := list.New(nil, newWrappingDelegate(2), 0, 0)
	l.Styles = androidListStyles()
	// The page title just above already names this screen - the list's
	// own title bar would just restate that a second time.
	l.SetShowTitle(false)
	l.SetShowStatusBar(false)
	l.SetShowHelp(false) // this screen renders its own footer text instead
	l.FilterInput.Placeholder = resolveUIText(settings.Language()).DeviceSelectFilterPlaceholder

	interval := time.Duration(settings.Devices.RefreshIntervalSeconds) * time.Second
	s := deviceSelectScreen{list: l, interval: interval}

	if client == nil {
		return s, nil
	}
	// Note: the recurring tick chain is kicked off by the caller (see
	// app.go/screen_health.go) at most once per app lifetime, not here -
	// re-entering this screen must not spawn a second, redundant chain.
	return s, refreshDevicesCmd(ctx, client)
}

func (m Model) updateDeviceSelect(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case devicesRefreshedMsg:
		m.deviceSelect.err = msg.err
		if msg.err == nil {
			items := make([]list.Item, len(msg.devices))
			for i, d := range msg.devices {
				items[i] = deviceItem{d: d, stateLabel: describeDeviceState(d.State, m.text)}
			}
			m.deviceSelect.list.SetItems(items)
		}
		return m, nil

	case tea.KeyMsg:
		// Guard against intercepting keys meant for the filter text box -
		// the same pattern screen_dashboard.go and screen_recover.go use
		// for their own lists.
		if m.deviceSelect.list.FilterState() != list.Filtering {
			switch msg.String() {
			case "q":
				return m, tea.Quit
			case "r":
				return m, refreshDevicesCmd(m.ctx, m.adbClient)
			case "s":
				m.settingsScreen = newSettingsScreen(m)
				m.settingsReturnTo = screenDeviceSelect
				m.current = screenSettings
				return m, nil
			case "enter":
				if item, ok := m.deviceSelect.list.SelectedItem().(deviceItem); ok {
					if !item.d.Connected() {
						m.deviceSelect.err = fmt.Errorf(m.text.DeviceNotReadyErrorFmt, item.d.Serial, item.d.State)
						return m, nil
					}
					return m.enterDashboard(item.d.Serial)
				}
			}
		}
	}

	var cmd tea.Cmd
	m.deviceSelect.list, cmd = m.deviceSelect.list.Update(msg)
	return m, cmd
}

func (m Model) viewDeviceSelect() string {
	body := m.deviceSelect.list.View()
	if len(m.deviceSelect.list.Items()) == 0 {
		body = m.styles.Subtle.Render(m.text.DeviceNoneConnected)
	}
	footer := m.styles.Subtle.Render(m.text.DeviceSelectFooter)
	if m.deviceSelect.err != nil {
		footer = m.styles.Error.Render(m.deviceSelect.err.Error()) + "\n" + footer
	}
	return m.styles.Title.Render(m.text.DeviceSelectTitle) + "\n\n" + body + "\n" + footer
}
