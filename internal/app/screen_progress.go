package app

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/progress"
	tea "github.com/charmbracelet/bubbletea"
)

// progressEvent is one update from a background long-running task (a
// download, an "sdkmanager --install", ...): either one line of output
// (with an optional parsed percentage) or a final done signal. Mirrors
// screen_runner.go's streamEvent, extended with pct since plain action
// output streaming has no use for a percentage.
type progressEvent struct {
	runID int
	line  string
	pct   int // -1 if this line carried no parsable percentage
	done  bool
	err   error
}

// progressRunner is a reusable "run this task, show its progress" component
// (a real, byte/percentage-driven bar plus a scrollback of recent lines) -
// embedded by whichever screen needs to show a download or install running,
// rather than a top-level screen of its own, since every current use (the
// emulator create wizard) is a stage of a larger flow rather than a
// destination in its own right.
type progressRunner struct {
	runID    int
	bar      progress.Model
	lines    []string
	events   chan progressEvent
	finished bool
	err      error
}

const progressRunnerMaxLines = 8

var percentRe = regexp.MustCompile(`(\d{1,3})\s?%`)

// parsePercent extracts a 0-100 percentage from a progress line if present,
// or -1 if the line carries no such marker - both toolsmanager's HTTP
// download progress ("42% - foo.zip") and sdkmanager's own install output
// ("Fetching ... 42%") use this format.
func parsePercent(line string) int {
	m := percentRe.FindStringSubmatch(line)
	if m == nil {
		return -1
	}
	n, err := strconv.Atoi(m[1])
	if err != nil {
		return -1
	}
	if n < 0 {
		n = 0
	}
	if n > 100 {
		n = 100
	}
	return n
}

func newProgressRunner(width int) progressRunner {
	return progressRunner{bar: progress.New(progress.WithGradient(colorAndroidDark.Dark, colorAndroidGreen.Dark), progress.WithWidth(width))}
}

// startProgressRunner runs task in a goroutine, feeding every line it
// reports through progress (a plain string callback, the same signature
// toolsmanager.ProgressFunc already uses) into the runner as a
// progressEvent. runID lets a stale event from a previously abandoned run be
// told apart from the current one, same reasoning as runnerScreen.runID.
func startProgressRunner(runID, width int, task func(report func(string)) error) (progressRunner, tea.Cmd) {
	events := make(chan progressEvent, 256)
	pr := progressRunner{runID: runID, bar: newProgressRunner(width).bar, events: events}

	report := func(line string) {
		events <- progressEvent{runID: runID, line: line, pct: parsePercent(line)}
	}

	startCmd := func() tea.Msg {
		go func() {
			err := task(report)
			events <- progressEvent{runID: runID, done: true, err: err}
			close(events)
		}()
		return waitForProgressEvent(runID, events)()
	}
	return pr, startCmd
}

func waitForProgressEvent(runID int, ch chan progressEvent) tea.Cmd {
	return func() tea.Msg {
		ev, ok := <-ch
		if !ok {
			return progressEvent{runID: runID, done: true}
		}
		return ev
	}
}

// updateProgressRunner applies msg to pr. justFinished is true exactly once,
// the update in which the task's completion (success or error) is first
// observed - callers use it to trigger whatever comes next (e.g. the
// emulator create wizard moving on to "avdmanager create avd" once an
// image's download finishes).
func updateProgressRunner(pr progressRunner, msg tea.Msg) (progressRunner, tea.Cmd, bool) {
	switch msg := msg.(type) {
	case progressEvent:
		if msg.runID != pr.runID {
			return pr, nil, false // stale event from an abandoned run
		}
		if msg.done {
			pr.finished = true
			pr.err = msg.err
			cmd := pr.bar.SetPercent(1.0)
			return pr, cmd, true
		}
		if msg.line != "" {
			pr.lines = append(pr.lines, msg.line)
			if len(pr.lines) > progressRunnerMaxLines {
				pr.lines = pr.lines[len(pr.lines)-progressRunnerMaxLines:]
			}
		}
		var cmd tea.Cmd
		if msg.pct >= 0 {
			cmd = pr.bar.SetPercent(float64(msg.pct) / 100)
		}
		return pr, tea.Batch(cmd, waitForProgressEvent(pr.runID, pr.events)), false

	case progress.FrameMsg:
		updated, cmd := pr.bar.Update(msg)
		if b, ok := updated.(progress.Model); ok {
			pr.bar = b
		}
		return pr, cmd, false
	}
	return pr, nil, false
}

func viewProgressRunner(pr progressRunner, s styles) string {
	var b strings.Builder
	b.WriteString(pr.bar.View())
	b.WriteString("\n\n")
	b.WriteString(s.Subtle.Render(strings.Join(pr.lines, "\n")))
	if pr.finished && pr.err != nil {
		b.WriteString("\n\n")
		b.WriteString(s.Error.Render(pr.err.Error()))
	}
	return b.String()
}
