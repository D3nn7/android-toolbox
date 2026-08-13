package app

import (
	"time"

	"android-toolbox/internal/actions"
	"android-toolbox/internal/adb"
	"android-toolbox/internal/device"
	"android-toolbox/internal/healthcheck"
)

// healthDoneMsg carries the result of the startup healthcheck.
type healthDoneMsg struct {
	report healthcheck.Report
	err    error
}

// devicesRefreshedMsg carries a fresh "adb devices" result.
type devicesRefreshedMsg struct {
	devices []adb.Device
	err     error
}

// deviceTickMsg fires the periodic device-list refresh on the select screen.
type deviceTickMsg struct{ at time.Time }

// deviceInfoMsg carries a fresh device info snapshot for the dashboard.
type deviceInfoMsg struct {
	info device.Info
	err  error
}

// infoTickMsg fires the periodic device-info refresh on the dashboard.
type infoTickMsg struct{ at time.Time }

// actionOutputLineMsg carries one line of streamed action output.
type actionOutputLineMsg struct {
	line string
}

// actionDoneMsg signals a streamed action has finished.
type actionDoneMsg struct {
	err      error
	exitCode int
}

// scrcpyStartedMsg reports the outcome of launching a detached scrcpy action.
type scrcpyStartedMsg struct {
	pid int
	err error
}

// actionSavedMsg reports the outcome of persisting an AI-generated action.
type actionSavedMsg struct {
	err error
}

// aiDraftMsg carries the result of an AI action-generation request.
type aiDraftMsg struct {
	action actions.Action
	err    error
}

// statusMsg pushes a transient one-line status message onto the dashboard.
type statusMsg struct {
	text  string
	isErr bool
}

// livePreviewMsg carries the result of an auto-run "live preview" action
// (see actions.Action.LivePreviewEligible), tagged with the action ID it
// belongs to and a runID so a result arriving after the user already moved
// on to a different item can be told apart from the current one and
// dropped instead of overwriting unrelated state.
type livePreviewMsg struct {
	runID    int
	actionID string
	output   string
	err      error
}
