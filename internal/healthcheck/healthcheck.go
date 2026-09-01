// Package healthcheck runs startup diagnostics so problems (missing tools,
// broken config) surface with a clear remediation hint instead of a random
// failure deep inside the TUI.
package healthcheck

import (
	"context"
	"fmt"
	"os/exec"

	"android-toolbox/internal/actions"
	"android-toolbox/internal/config"
	"android-toolbox/internal/toolsmanager"
)

// Severity classifies a check result.
type Severity string

const (
	OK   Severity = "ok"
	Warn Severity = "warn"
	Fail Severity = "fail"
)

// Result is the outcome of a single check.
type Result struct {
	Name        string
	Severity    Severity
	Detail      string
	Remediation string
}

// Report is the full set of check results.
type Report struct {
	Results []Result
}

// HasFailures reports whether any check failed hard.
func (r Report) HasFailures() bool {
	for _, res := range r.Results {
		if res.Severity == Fail {
			return true
		}
	}
	return false
}

// Run executes every check and returns the aggregated report. It never
// returns an error itself - failures are represented as Fail results so the
// caller always gets a full picture.
func Run(ctx context.Context, paths config.Paths, settings config.Settings) Report {
	var report Report

	report.Results = append(report.Results, checkConfigDir(paths))
	report.Results = append(report.Results, checkActionsFile(paths))
	report.Results = append(report.Results, checkADB(paths))
	report.Results = append(report.Results, checkScrcpy(paths))
	report.Results = append(report.Results, checkAIProvider(settings))
	report.Results = append(report.Results, checkJava())
	report.Results = append(report.Results, checkAvdTools(paths))

	return report
}

func checkConfigDir(paths config.Paths) Result {
	if paths.ConfigDir == "" {
		return Result{Name: "Configuration directory", Severity: Fail, Detail: "Could not determine path"}
	}
	return Result{Name: "Configuration directory", Severity: OK, Detail: paths.ConfigDir}
}

func checkActionsFile(paths config.Paths) Result {
	set, err := actions.Load(paths.ActionsFile, actions.DefaultActionsYAML)
	if err != nil {
		return Result{
			Name:        "Actions (actions.yaml)",
			Severity:    Fail,
			Detail:      err.Error(),
			Remediation: "Check actions.yaml or restore a backup via 'android-toolbox recover'",
		}
	}
	if len(set.Invalid) > 0 {
		return Result{
			Name:        "Actions (actions.yaml)",
			Severity:    Warn,
			Detail:      formatInvalidActions(set.Invalid),
			Remediation: "Fix the invalid entries in actions.yaml",
		}
	}
	return Result{Name: "Actions (actions.yaml)", Severity: OK, Detail: formatActionCount(len(set.Actions))}
}

func formatActionCount(n int) string {
	if n == 1 {
		return "1 action loaded"
	}
	return itoa(n) + " actions loaded"
}

func formatInvalidActions(invalid []actions.InvalidAction) string {
	s := itoa(len(invalid)) + " invalid action(s): "
	for i, ia := range invalid {
		if i > 0 {
			s += "; "
		}
		s += ia.ID + " (" + ia.Reason + ")"
	}
	return s
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}

func checkADB(paths config.Paths) Result {
	mgr := toolsmanager.New(paths.ToolsDir)
	tool, err := mgr.ResolveADB()
	if err != nil {
		return Result{
			Name:        "adb",
			Severity:    Fail,
			Detail:      err.Error(),
			Remediation: "Run 'android-toolbox tools fetch'",
		}
	}
	if out, err := exec.Command(tool.Path, "start-server").CombinedOutput(); err != nil {
		return Result{
			Name:        "adb",
			Severity:    Fail,
			Detail:      "adb start-server failed: " + string(out),
			Remediation: "Check your USB driver/adb installation",
		}
	}
	return Result{Name: "adb", Severity: OK, Detail: tool.Path + " (" + tool.Source + ")"}
}

func checkScrcpy(paths config.Paths) Result {
	mgr := toolsmanager.New(paths.ToolsDir)
	tool, err := mgr.ResolveScrcpy()
	if err != nil {
		return Result{
			Name:        "scrcpy",
			Severity:    Warn,
			Detail:      err.Error(),
			Remediation: "Run 'android-toolbox tools fetch' (display actions are disabled until then)",
		}
	}
	if err := exec.Command(tool.Path, "--version").Run(); err != nil {
		return Result{
			Name:        "scrcpy",
			Severity:    Fail,
			Detail:      "scrcpy --version failed: " + err.Error(),
			Remediation: "Run 'android-toolbox tools fetch' again",
		}
	}
	return Result{Name: "scrcpy", Severity: OK, Detail: tool.Path + " (" + tool.Source + ")"}
}

func checkJava() Result {
	tool, err := toolsmanager.ResolveJava()
	if err != nil {
		return Result{
			Name:        "Java (for emulator manager)",
			Severity:    Warn,
			Detail:      err.Error(),
			Remediation: "Install a JRE/JDK (11+) if you want to create/manage emulators",
		}
	}
	if err := exec.Command(tool.Path, "-version").Run(); err != nil {
		return Result{
			Name:        "Java (for emulator manager)",
			Severity:    Warn,
			Detail:      "java -version failed: " + err.Error(),
			Remediation: "Check your Java installation",
		}
	}
	return Result{Name: "Java (for emulator manager)", Severity: OK, Detail: tool.Path}
}

func checkAvdTools(paths config.Paths) Result {
	mgr := toolsmanager.New(paths.ToolsDir)
	sdkManager, sdkErr := mgr.ResolveSdkManager()
	avdManager, avdErr := mgr.ResolveAvdManager()
	if sdkErr != nil || avdErr != nil {
		detail := "sdkmanager/avdmanager not found"
		if sdkErr != nil {
			detail = sdkErr.Error()
		} else if avdErr != nil {
			detail = avdErr.Error()
		}
		return Result{
			Name:        "Android SDK tools (emulator manager)",
			Severity:    Warn,
			Detail:      detail,
			Remediation: "Run 'android-toolbox emulator setup'",
		}
	}
	return Result{
		Name:     "Android SDK tools (emulator manager)",
		Severity: OK,
		Detail:   fmt.Sprintf("sdkmanager: %s (%s), avdmanager: %s (%s)", sdkManager.Path, sdkManager.Source, avdManager.Path, avdManager.Source),
	}
}

func checkAIProvider(settings config.Settings) Result {
	cmdName := settings.AI.Claude.Command
	if cmdName == "" {
		cmdName = "claude"
	}
	path, err := exec.LookPath(cmdName)
	if err != nil {
		return Result{
			Name:        "AI provider (" + settings.AI.Provider + ")",
			Severity:    Warn,
			Detail:      "CLI '" + cmdName + "' not found on PATH",
			Remediation: "Install the Claude Code CLI if you want to use AI mode",
		}
	}
	return Result{Name: "AI provider (" + settings.AI.Provider + ")", Severity: OK, Detail: path}
}
