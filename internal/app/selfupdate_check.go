package app

import (
	"context"
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"android-toolbox/internal/buildinfo"
	"android-toolbox/internal/config"
	"android-toolbox/internal/selfupdate"
)

// selfUpdateCheckInterval bounds how often the background version check
// actually queries GitHub - frequent enough to notice a new release within
// about a day, infrequent enough to stay well clear of GitHub's
// unauthenticated API rate limit even for users who restart the app often.
const selfUpdateCheckInterval = 24 * time.Hour

// selfUpdateCheckTimeout keeps a stalled/unreachable GitHub from ever
// delaying startup - the check runs as a background tea.Cmd regardless, but
// an unbounded network call could otherwise hang around indefinitely.
const selfUpdateCheckTimeout = 5 * time.Second

// selfUpdateCheckMsg carries the result of the periodic self-update check:
// version is the latest known release (from a fresh query or the cache),
// empty if the check failed or none is known yet.
type selfUpdateCheckMsg struct {
	version string
}

// checkForSelfUpdateCmd returns a command that reports the latest known
// android-toolbox release - either straight from the state.json cache (if
// checked within selfUpdateCheckInterval) or from a fresh, time-boxed
// GitHub query, whose result is then cached for next time. A failed query
// is silently swallowed (empty version): a missed update notice is a
// cosmetic problem, not one worth surfacing as an error on every launch.
func checkForSelfUpdateCmd(ctx context.Context, paths config.Paths, state config.State) tea.Cmd {
	return func() tea.Msg {
		if lastChecked, err := time.Parse(time.RFC3339, state.LastUpdateCheckAt); err == nil {
			if time.Since(lastChecked) < selfUpdateCheckInterval {
				return selfUpdateCheckMsg{version: state.LatestKnownVersion}
			}
		}

		checkCtx, cancel := context.WithTimeout(ctx, selfUpdateCheckTimeout)
		defer cancel()
		rel, err := selfupdate.LatestRelease(checkCtx)
		if err != nil {
			return selfUpdateCheckMsg{}
		}

		state.LastUpdateCheckAt = time.Now().UTC().Format(time.RFC3339)
		state.LatestKnownVersion = rel.Version
		_ = config.SaveState(paths, state) // best-effort: a failed cache write just means we ask again next launch

		return selfUpdateCheckMsg{version: rel.Version}
	}
}

// updateNoticeText renders the "a new version is available" line shown on
// the healthcheck screen and the dashboard header, or "" if none is due
// (nothing known yet, or the known latest isn't actually newer).
func updateNoticeText(t uiText, latestVersion string) string {
	if latestVersion == "" || !selfupdate.IsNewer(buildinfo.Version, latestVersion) {
		return ""
	}
	return fmt.Sprintf(t.UpdateAvailableFmt, latestVersion, buildinfo.Version)
}
