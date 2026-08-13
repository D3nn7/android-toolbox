package app

import (
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/progress"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// splashLogo is a small hand-drawn block-letter rendering of the tool's
// short alias ("atbx") rather than its full name - a big-block "android-
// toolbox" banner would run well past 80 columns and wrap badly on a
// narrower terminal. The full name still appears as a normal subtitle line
// right underneath (see viewSplash), so the tool is clearly identified
// either way.
var splashLogo = []string{
	" ███   █████  ████    █   █ ",
	"█   █    █    █   █    █ █  ",
	"█████    █    ████      █   ",
	"█   █    █    █   █    █ █  ",
	"█   █    █    ████    █   █ ",
}

const (
	// splashProgressCap is how far the synthetic "still working" ramp is
	// allowed to climb on its own - it must never reach 100% before the
	// real healthcheck (running concurrently, see Model.Init) actually
	// finishes, or the bar would look done while the app is still loading.
	splashProgressCap = 0.92
	// splashRampFactor is the fraction of the remaining distance to
	// splashProgressCap covered on each tick - an ease-out curve that
	// slows down as it approaches the cap instead of arriving abruptly,
	// so the animation still reads as "working" during an unusually slow
	// startup instead of stalling dead at a fixed percentage.
	splashRampFactor   = 0.2
	splashTickInterval = 120 * time.Millisecond
	// splashMinDuration is the shortest amount of time the splash stays on
	// screen, regardless of how fast the real healthcheck finishes (which
	// is normally near-instant) - without this the animation would often
	// flash by for a single frame, defeating the point of having one.
	splashMinDuration = 1400 * time.Millisecond
	// splashSettleDelay keeps the completed (100%) bar on screen just long
	// enough to actually register as "done" before handing off to whatever
	// comes next - snapping away the instant completion is decided would
	// make that moment invisible.
	splashSettleDelay = 350 * time.Millisecond

	splashProgressWidth = 40
)

// splashTickMsg drives the synthetic progress ramp described above.
type splashTickMsg struct{}

// splashMinDurationElapsedMsg fires once splashMinDuration has passed since
// the splash screen appeared.
type splashMinDurationElapsedMsg struct{}

// splashDoneMsg fires once the post-completion settle delay has elapsed.
type splashDoneMsg struct{}

type splashScreen struct {
	progress progress.Model
	// reportReady is set once the real healthDoneMsg has arrived.
	reportReady bool
	// minDurationElapsed is set once splashMinDuration has passed.
	minDurationElapsed bool
	// completing is set once both of the above are true and the bar has
	// been told to animate to 100% - the ramp (splashTickMsg) stops once
	// this is set, since the bar is now animating toward its real target.
	completing bool
}

// splashProgressBarWidth clamps the bar to a sensible range: full
// splashProgressWidth on a normally sized terminal, narrower on a small one,
// but never so wide it risks colliding with the terminal edge once the
// percentage text is added.
func splashProgressBarWidth(terminalWidth int) int {
	w := terminalWidth - 10
	if w > splashProgressWidth {
		return splashProgressWidth
	}
	if w < 10 {
		return 10
	}
	return w
}

func newSplashScreen() splashScreen {
	p := progress.New(
		progress.WithGradient(colorAndroidDark.Dark, colorAndroidGreen.Dark),
		progress.WithWidth(splashProgressWidth),
	)
	return splashScreen{progress: p}
}

// Init only starts the decorative ramp and the minimum-duration timer - the
// actual healthcheck is kicked off separately (see Model.Init) since it
// must run identically whether or not the splash animation is enabled.
func (s splashScreen) Init() tea.Cmd {
	return tea.Batch(splashTickCmd(), splashMinDurationCmd())
}

func splashTickCmd() tea.Cmd {
	return tea.Tick(splashTickInterval, func(time.Time) tea.Msg { return splashTickMsg{} })
}

func splashMinDurationCmd() tea.Cmd {
	return tea.Tick(splashMinDuration, func(time.Time) tea.Msg { return splashMinDurationElapsedMsg{} })
}

func splashSettleCmd() tea.Cmd {
	return tea.Tick(splashSettleDelay, func(time.Time) tea.Msg { return splashDoneMsg{} })
}

// beginSplashCompletion animates the bar the rest of the way to 100% and
// schedules the handoff to finalizeHealthcheck - only called once both the
// real result is in AND the minimum display duration has passed.
func (m Model) beginSplashCompletion() (Model, tea.Cmd) {
	m.splash.completing = true
	cmd := m.splash.progress.SetPercent(1.0)
	return m, tea.Batch(cmd, splashSettleCmd())
}

func (m Model) updateSplash(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case healthDoneMsg:
		// Store the result exactly like updateHealthcheck does - by the
		// time finalizeHealthcheck runs (after the settle delay below) it
		// reads m.health, not this message.
		m.health.report = msg.report
		m.health.err = msg.err
		m.health.done = true
		m.splash.reportReady = true
		if m.splash.minDurationElapsed {
			return m.beginSplashCompletion()
		}
		return m, nil // keep ramping until the minimum duration is also met

	case splashMinDurationElapsedMsg:
		m.splash.minDurationElapsed = true
		if m.splash.reportReady {
			return m.beginSplashCompletion()
		}
		return m, nil

	case splashTickMsg:
		if m.splash.completing {
			return m, nil // already animating to its final target
		}
		current := m.splash.progress.Percent()
		next := current + (splashProgressCap-current)*splashRampFactor
		cmd := m.splash.progress.SetPercent(next)
		return m, tea.Batch(cmd, splashTickCmd())

	case progress.FrameMsg:
		updated, cmd := m.splash.progress.Update(msg)
		if p, ok := updated.(progress.Model); ok {
			m.splash.progress = p
		}
		return m, cmd

	case splashDoneMsg:
		return m.finalizeHealthcheck()
	}
	return m, nil
}

func (m Model) viewSplash() string {
	var b strings.Builder
	for _, line := range splashLogo {
		b.WriteString(m.styles.Highlight.Render(line))
		b.WriteString("\n")
	}
	b.WriteString("\n")
	b.WriteString(m.styles.Subtle.Render(m.text.AppTitle))
	b.WriteString("\n\n")
	b.WriteString(m.splash.progress.View())
	content := b.String()

	// m.width/m.height aren't known yet on the very first frame (before
	// bubbletea's initial tea.WindowSizeMsg arrives) - fall back to
	// unpositioned content rather than centering into a bogus 0x0 box.
	if m.width < 1 || m.height < 1 {
		return content
	}
	// -1 matches View()'s own terminalBottomMargin: never place content
	// into the very last row, which some terminals auto-scroll on.
	return lipgloss.Place(m.width, m.height-1, lipgloss.Center, lipgloss.Center, content)
}
