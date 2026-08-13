package actions

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"regexp"

	"github.com/google/shlex"

	"android-toolbox/internal/scrcpy"
)

// Executor turns a validated Action plus a target serial/param values into a
// runnable command, using pre-resolved tool paths so it never has to know
// where adb/scrcpy actually live.
type Executor struct {
	ADBPath string
	Scrcpy  *scrcpy.Launcher
}

// NewExecutor creates an Executor. scrcpyLauncher may be nil if scrcpy is
// unavailable; scrcpy actions will then fail with a clear error instead of
// panicking.
func NewExecutor(adbPath string, scrcpyLauncher *scrcpy.Launcher) *Executor {
	return &Executor{ADBPath: adbPath, Scrcpy: scrcpyLauncher}
}

var placeholderRe = regexp.MustCompile(`\{(\w+)\}`)

// substitutePlaceholders replaces every "{key}" in s with values[key],
// leaving unknown placeholders untouched so typos surface in the output
// instead of silently vanishing.
func substitutePlaceholders(s string, values map[string]string) string {
	return placeholderRe.ReplaceAllStringFunc(s, func(m string) string {
		key := m[1 : len(m)-1]
		if v, ok := values[key]; ok {
			return v
		}
		return m
	})
}

// resolveParamValues merges user-supplied values with each param's default.
func resolveParamValues(a Action, serial string, supplied map[string]string) map[string]string {
	values := map[string]string{"serial": serial}
	for _, p := range a.Params {
		v := supplied[p.Name]
		if v == "" {
			v = p.Default
		}
		values[p.Name] = v
	}
	return values
}

func quoteForShell(path string) string {
	if path == "" {
		return ""
	}
	return `"` + path + `"`
}

// prepareTokens shlex-splits the action's raw command template (so quoted
// groups like `"dumpsys x | grep y"` stay one token) and only then
// substitutes placeholders into each resulting token. Substituting *after*
// tokenizing - rather than substituting into the raw string and re-parsing -
// means a value containing backslashes (e.g. a Windows path) or spaces is
// inserted verbatim and can never be mangled by shell-quoting rules.
func prepareTokens(command string, values map[string]string) ([]string, error) {
	raw, err := shlex.Split(command)
	if err != nil {
		return nil, fmt.Errorf("could not parse command: %w", err)
	}
	tokens := make([]string, len(raw))
	for i, t := range raw {
		tokens[i] = substitutePlaceholders(t, values)
	}
	return tokens, nil
}

// Prepare builds the *exec.Cmd for an adb or shell action. It must not be
// used for tool=scrcpy - use StartScrcpy instead, since scrcpy is launched
// detached rather than streamed.
func (e *Executor) Prepare(ctx context.Context, a Action, serial string, supplied map[string]string) (*exec.Cmd, error) {
	values := resolveParamValues(a, serial, supplied)

	switch a.Tool {
	case ToolADB:
		if e.ADBPath == "" {
			return nil, fmt.Errorf("adb is not available (see 'android-toolbox tools fetch')")
		}
		tokens, err := prepareTokens(a.Command, values)
		if err != nil {
			return nil, err
		}
		args := append([]string{"-s", serial}, tokens...)
		return exec.CommandContext(ctx, e.ADBPath, args...), nil

	case ToolShell:
		values["adb"] = quoteForShell(e.ADBPath)
		if e.Scrcpy != nil {
			values["scrcpy"] = quoteForShell(e.Scrcpy.BinPath)
		}
		command := substitutePlaceholders(a.Command, values)
		return shellCommand(ctx, command), nil

	case ToolScrcpy:
		return nil, fmt.Errorf("tool 'scrcpy' is started via StartScrcpy, not Prepare")

	default:
		return nil, fmt.Errorf("unknown tool: %q", a.Tool)
	}
}

// RunningAction represents a streamed adb/shell action: Output yields the
// combined stdout+stderr as the process runs. Wait blocks until it exits.
//
// cmd.Wait() is called exactly once, internally, by the goroutine Start
// spawns to close the output pipe once the process exits - exec.Cmd forbids
// calling Wait more than once (the second call fails with "Wait was already
// called", and worse, racing two callers over the same *os.ProcessState is
// undefined). Wait/ExitCode therefore synchronize on the `done` channel
// instead of ever touching cmd.Wait() themselves.
type RunningAction struct {
	cmd     *exec.Cmd
	Output  io.Reader
	done    chan struct{}
	waitErr error
}

// Wait blocks until the process has exited and returns its error, if any.
func (r *RunningAction) Wait() error {
	<-r.done
	return r.waitErr
}

// ExitCode returns the process's exit code once Wait has returned/would
// return immediately. It is -1 if the process has not exited yet or exited
// abnormally without a code.
func (r *RunningAction) ExitCode() int {
	select {
	case <-r.done:
	default:
		return -1
	}
	if r.cmd.ProcessState == nil {
		return -1
	}
	return r.cmd.ProcessState.ExitCode()
}

// Start begins a streamed adb or shell action and returns immediately; the
// caller reads RunningAction.Output until EOF and then calls Wait for the
// final error/exit code. Intended for the TUI's live output viewport.
func (e *Executor) Start(ctx context.Context, a Action, serial string, supplied map[string]string) (*RunningAction, error) {
	if a.Tool == ToolScrcpy {
		return nil, fmt.Errorf("tool 'scrcpy' is started via StartScrcpy, not Start")
	}
	cmd, err := e.Prepare(ctx, a, serial, supplied)
	if err != nil {
		return nil, err
	}

	pr, pw := io.Pipe()
	cmd.Stdout = pw
	cmd.Stderr = pw

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start failed: %w", err)
	}

	ra := &RunningAction{cmd: cmd, Output: pr, done: make(chan struct{})}
	go func() {
		ra.waitErr = cmd.Wait()
		pw.Close()
		close(ra.done)
	}()

	return ra, nil
}

// RunSync runs an adb or shell action to completion, writing its combined
// output directly to stdout/stderr. Used by the non-interactive `run` CLI
// command.
func (e *Executor) RunSync(ctx context.Context, a Action, serial string, supplied map[string]string, stdout, stderr io.Writer) error {
	if a.Tool == ToolScrcpy {
		return fmt.Errorf("tool 'scrcpy' is started via StartScrcpy, not RunSync")
	}
	cmd, err := e.Prepare(ctx, a, serial, supplied)
	if err != nil {
		return err
	}
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	return cmd.Run()
}

// RunInteractive runs an action with the process's own stdin/stdout/stderr
// wired directly to the terminal - for actions like a bare "adb shell" that
// need a real interactive session. Callers running inside the Bubbletea TUI
// must release the terminal first.
func (e *Executor) RunInteractive(ctx context.Context, a Action, serial string, supplied map[string]string) error {
	if a.Tool == ToolScrcpy {
		return fmt.Errorf("tool 'scrcpy' is started via StartScrcpy, not RunInteractive")
	}
	cmd, err := e.Prepare(ctx, a, serial, supplied)
	if err != nil {
		return err
	}
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// StartScrcpy launches a tool=scrcpy action via the detached scrcpy
// launcher. It returns as soon as the process has started; scrcpy runs
// independently of the TUI from then on.
func (e *Executor) StartScrcpy(a Action, serial string, supplied map[string]string) (*exec.Cmd, error) {
	if a.Tool != ToolScrcpy {
		return nil, fmt.Errorf("StartScrcpy expects tool 'scrcpy', got %q", a.Tool)
	}
	if e.Scrcpy == nil {
		return nil, fmt.Errorf("scrcpy is not available (see 'android-toolbox tools fetch')")
	}
	values := resolveParamValues(a, serial, supplied)
	tokens, err := prepareTokens(a.Command, values)
	if err != nil {
		return nil, err
	}
	return e.Scrcpy.Launch(serial, tokens)
}
