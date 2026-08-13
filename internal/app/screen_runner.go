package app

import (
	"bufio"
	"context"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"

	"android-toolbox/internal/actions"
)

// streamEvent is the single message type the background reader goroutine
// sends: either one output line, or a final done signal carrying the
// process's exit error (nil on success). runID ties every event to the
// specific run that produced it (see runnerScreen.runID).
type streamEvent struct {
	runID int
	line  string
	done  bool
	err   error
}

type runnerScreen struct {
	runID    int
	action   actions.Action
	viewport viewport.Model
	lines    []string
	events   chan streamEvent
	cancel   context.CancelFunc
	finished bool
	exitErr  error
}

// newRunnerScreen starts action as a streamed run tagged with runID. runID
// must be unique per run (see Model.runnerSeq): if the user cancels and
// immediately starts a new action, Bubbletea may still have the previous
// run's "wait for next event" command in flight, and its eventual message
// must be recognizable as stale rather than mistakenly applied to the new
// run's screen.
func newRunnerScreen(ctx context.Context, executor *actions.Executor, a actions.Action, serial string, params map[string]string, width, height, runID int) (runnerScreen, tea.Cmd) {
	runCtx, cancel := context.WithCancel(ctx)
	events := make(chan streamEvent, 256)

	// Sized from the already-known terminal dimensions rather than a fixed
	// default: WindowSizeMsg only fires on an actual resize, so a hardcoded
	// size here would stick for the whole action if the user never resizes.
	vp := viewport.New(width, height)

	r := runnerScreen{runID: runID, action: a, viewport: vp, events: events, cancel: cancel}

	startCmd := func() tea.Msg {
		ra, err := executor.Start(runCtx, a, serial, params)
		if err != nil {
			return streamEvent{runID: runID, done: true, err: err}
		}
		go func() {
			scanner := bufio.NewScanner(ra.Output)
			scanner.Buffer(make([]byte, 64*1024), 1024*1024)
			for scanner.Scan() {
				events <- streamEvent{runID: runID, line: scanner.Text()}
			}
			waitErr := ra.Wait()
			events <- streamEvent{runID: runID, done: true, err: waitErr}
			close(events)
		}()
		return waitForStreamEvent(runID, events)()
	}

	return r, startCmd
}

func waitForStreamEvent(runID int, ch chan streamEvent) tea.Cmd {
	return func() tea.Msg {
		ev, ok := <-ch
		if !ok {
			return streamEvent{runID: runID, done: true}
		}
		return ev
	}
}

func (m Model) updateRunner(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case streamEvent:
		if msg.runID != m.runner.runID {
			// Leftover message from a run the user already canceled/left -
			// e.g. its final "process killed" event arriving just after a
			// new action was started on this same screen. Drop it instead
			// of overwriting the new run's (unrelated) state with it.
			return m, nil
		}
		if msg.done {
			m.runner.finished = true
			m.runner.exitErr = msg.err
			m.runner.viewport.SetContent(strings.Join(m.runner.lines, "\n"))
			m.runner.viewport.GotoBottom()
			return m, nil
		}
		m.runner.lines = append(m.runner.lines, m.renderOutputLine(m.runner.action.Format, msg.line))
		m.runner.viewport.SetContent(strings.Join(m.runner.lines, "\n"))
		m.runner.viewport.GotoBottom()
		return m, waitForStreamEvent(m.runner.runID, m.runner.events)

	case tea.KeyMsg:
		switch msg.String() {
		case "q", "esc":
			// Always cancel, even if the run already finished on its own -
			// context.CancelFunc is a no-op in that case, but skipping it
			// here would leak the run's context under the parent's
			// cancel-tree for the rest of the app's lifetime.
			if m.runner.cancel != nil {
				m.runner.cancel()
			}
			m.current = screenDashboard
			return m, nil
		}
	}

	var cmd tea.Cmd
	m.runner.viewport, cmd = m.runner.viewport.Update(msg)
	return m, cmd
}

// viewRunner renders the RIGHT pane's content while an action streams
// output (or has just finished). The left pane and header/footer are
// composed separately in app.go (see isDashboardCluster).
func (m Model) viewRunner() string {
	status := m.styles.Info.Render(m.text.RunnerRunning)
	if m.runner.finished {
		lineWord := m.text.RunnerLines
		if len(m.runner.lines) == 1 {
			lineWord = m.text.RunnerLine
		}
		if m.runner.exitErr != nil {
			status = m.styles.Error.Render(fmt.Sprintf(m.text.RunnerErrorFmt, m.runner.exitErr.Error(), len(m.runner.lines), lineWord))
		} else {
			status = m.styles.OK.Render(fmt.Sprintf(m.text.RunnerCompletedFmt, len(m.runner.lines), lineWord))
		}
	}

	return m.styles.Highlight.Render(m.runner.action.Name) + "\n" +
		m.runner.viewport.View() + "\n" + status
}
