package app

import (
	"context"
	"runtime"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"android-toolbox/internal/config"
	"android-toolbox/internal/toolsmanager"
)

// toolsUpdateCheckInterval/Timeout mirror selfUpdateCheckInterval/Timeout -
// same reasoning (stay well under API rate limits, never delay startup).
const (
	toolsUpdateCheckInterval = 24 * time.Hour
	toolsUpdateCheckTimeout  = 5 * time.Second
)

// toolsUpdateCheckMsg carries the latest known adb/scrcpy versions - either
// value is empty if that particular check failed or hasn't happened yet.
type toolsUpdateCheckMsg struct {
	adbLatest    string
	scrcpyLatest string
}

// checkForToolUpdatesCmd is checkForSelfUpdateCmd's counterpart for the
// third-party tools: same cache-then-live-query shape, gated by
// UISettings.AutoCheckToolUpdates (nil when disabled, so Init() has nothing
// to batch in at all - this only ever checks, never installs).
func checkForToolUpdatesCmd(ctx context.Context, paths config.Paths, state config.State, enabled bool) tea.Cmd {
	if !enabled {
		return nil
	}
	return func() tea.Msg {
		if lastChecked, err := time.Parse(time.RFC3339, state.LastToolsUpdateCheckAt); err == nil {
			if time.Since(lastChecked) < toolsUpdateCheckInterval {
				return toolsUpdateCheckMsg{adbLatest: state.ADBLatestKnown, scrcpyLatest: state.ScrcpyLatestKnown}
			}
		}

		checkCtx, cancel := context.WithTimeout(ctx, toolsUpdateCheckTimeout)
		defer cancel()
		adbLatest, scrcpyLatest := liveCheckToolVersions(checkCtx, paths)

		state.LastToolsUpdateCheckAt = time.Now().UTC().Format(time.RFC3339)
		state.ADBLatestKnown = adbLatest
		state.ScrcpyLatestKnown = scrcpyLatest
		_ = config.SaveState(paths, state) // best-effort, same reasoning as checkForSelfUpdateCmd

		return toolsUpdateCheckMsg{adbLatest: adbLatest, scrcpyLatest: scrcpyLatest}
	}
}

// liveCheckToolVersions queries the actual latest adb/scrcpy versions,
// swallowing individual failures (e.g. offline) as "" rather than failing
// the whole check - shared by the background check above and the Settings
// screen's manual "check now" action (see screen_settings.go).
func liveCheckToolVersions(ctx context.Context, paths config.Paths) (adbLatest, scrcpyLatest string) {
	mgr := toolsmanager.New(paths.ToolsDir)
	if status, err := mgr.CheckADBUpdate(ctx, runtime.GOOS); err == nil {
		adbLatest = status.Latest
	}
	scrcpyLatest = mgr.CheckScrcpyUpdate(ctx, runtime.GOOS, runtime.GOARCH).Latest
	return adbLatest, scrcpyLatest
}

// outdatedTools reports which of adb/scrcpy have a locally-installed
// version older than the latest known one - purely local comparisons (no
// network), safe to call on every render.
func outdatedTools(paths config.Paths, latestKnownADB, latestKnownScrcpy string) (adbOutdated, scrcpyOutdated bool) {
	mgr := toolsmanager.New(paths.ToolsDir)
	adbOutdated = latestKnownADB != "" && mgr.InstalledADBVersion(runtime.GOOS) != latestKnownADB
	scrcpyOutdated = latestKnownScrcpy != "" && mgr.InstalledScrcpyVersion(runtime.GOOS, runtime.GOARCH) != latestKnownScrcpy
	return adbOutdated, scrcpyOutdated
}
