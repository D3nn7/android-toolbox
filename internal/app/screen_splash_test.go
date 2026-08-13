package app

import (
	"context"
	"testing"

	"github.com/charmbracelet/bubbles/progress"

	"android-toolbox/internal/config"
	"android-toolbox/internal/healthcheck"
)

// TestNewRespectsShowStartupAnimationSetting proves the splash screen is
// only entered when the setting is actually on - a zero-value/off setting
// must fall back to the pre-existing behavior of starting directly on the
// healthcheck screen.
func TestNewRespectsShowStartupAnimationSetting(t *testing.T) {
	on := config.Settings{}
	on.UI.ShowStartupAnimation = true
	m := New(context.Background(), config.Paths{}, on, config.State{}, nil)
	if m.current != screenSplash {
		t.Fatalf("expected ShowStartupAnimation=true to start on the splash screen, got %v", m.current)
	}

	off := config.Settings{}
	m = New(context.Background(), config.Paths{}, off, config.State{}, nil)
	if m.current != screenHealthcheck {
		t.Fatalf("expected ShowStartupAnimation=false to start on the healthcheck screen directly, got %v", m.current)
	}
}

// TestSplashTickRampsTowardCapWithoutReachingOrExceedingIt is the safety net
// for the synthetic "still working" animation: it must keep climbing for an
// arbitrarily long real healthcheck, but never actually reach (let alone
// exceed) 100% before the real result is in - that would make the bar look
// finished while the app is still loading.
func TestSplashTickRampsTowardCapWithoutReachingOrExceedingIt(t *testing.T) {
	m := Model{splash: newSplashScreen()}

	var last float64
	for i := 0; i < 200; i++ {
		updated, _ := m.updateSplash(splashTickMsg{})
		m = updated.(Model)
		p := m.splash.progress.Percent()
		if p >= splashProgressCap {
			t.Fatalf("tick %d: percent %.4f reached/exceeded the cap %.4f before the real result arrived", i, p, splashProgressCap)
		}
		if p < last {
			t.Fatalf("tick %d: percent went backwards, %.4f -> %.4f", i, last, p)
		}
		last = p
	}
	if last < splashProgressCap*0.9 {
		t.Fatalf("expected the ramp to get reasonably close to the cap after 200 ticks, only reached %.4f", last)
	}
}

// TestSplashHealthDoneMsgAloneDoesNotCompleteBeforeMinDuration proves the
// splash keeps running (rather than instantly jumping to 100%) when the
// real healthcheck finishes before the minimum display duration has - the
// whole point of splashMinDuration is that a near-instant check must not
// make the animation flash by for a single frame.
func TestSplashHealthDoneMsgAloneDoesNotCompleteBeforeMinDuration(t *testing.T) {
	m := Model{splash: newSplashScreen()}
	report := healthcheck.Report{Results: []healthcheck.Result{{Name: "x", Severity: healthcheck.OK}}}

	updated, cmd := m.updateSplash(healthDoneMsg{report: report})
	m = updated.(Model)

	if !m.splash.reportReady {
		t.Fatal("expected reportReady to be set once the real result arrives")
	}
	if !m.health.done {
		t.Fatal("expected m.health.done to be set so finalizeHealthcheck has a complete report to read")
	}
	if m.splash.completing {
		t.Fatal("expected completion to wait for the minimum duration, not start immediately")
	}
	if m.splash.progress.Percent() == 1.0 {
		t.Fatal("expected the bar to NOT yet be targeted at 100% before the minimum duration elapses")
	}
	if cmd != nil {
		t.Fatal("expected no follow-up command yet - still waiting on the minimum duration timer")
	}

	// The ramp must keep running while only waiting on the timer.
	beforePercent := m.splash.progress.Percent()
	updated, _ = m.updateSplash(splashTickMsg{})
	m = updated.(Model)
	if m.splash.progress.Percent() <= beforePercent {
		t.Fatal("expected the ramp to keep climbing while only the minimum-duration timer is still pending")
	}
}

// TestSplashCompletesOnceBothReportAndMinDurationAreReady proves completion
// only starts once BOTH conditions are met, regardless of which one arrives
// second - covers both orderings.
func TestSplashCompletesOnceBothReportAndMinDurationAreReady(t *testing.T) {
	report := healthcheck.Report{Results: []healthcheck.Result{{Name: "x", Severity: healthcheck.OK}}}

	t.Run("report arrives first", func(t *testing.T) {
		m := Model{splash: newSplashScreen()}
		updated, _ := m.updateSplash(healthDoneMsg{report: report})
		m = updated.(Model)
		updated, cmd := m.updateSplash(splashMinDurationElapsedMsg{})
		m = updated.(Model)

		if !m.splash.completing || m.splash.progress.Percent() != 1.0 || cmd == nil {
			t.Fatalf("expected completion once the timer catches up, got completing=%v percent=%.2f cmd=%v",
				m.splash.completing, m.splash.progress.Percent(), cmd)
		}
	})

	t.Run("min duration elapses first", func(t *testing.T) {
		m := Model{splash: newSplashScreen()}
		updated, cmd := m.updateSplash(splashMinDurationElapsedMsg{})
		m = updated.(Model)
		if m.splash.completing || cmd != nil {
			t.Fatal("expected the timer alone to not complete anything without the report")
		}

		updated, cmd = m.updateSplash(healthDoneMsg{report: report})
		m = updated.(Model)
		if !m.splash.completing || m.splash.progress.Percent() != 1.0 || cmd == nil {
			t.Fatalf("expected completion once the report catches up, got completing=%v percent=%.2f cmd=%v",
				m.splash.completing, m.splash.progress.Percent(), cmd)
		}
	})
}

// TestSplashTickIsNoOpOnceCompleting proves the synthetic ramp stops for
// good once completion has actually begun.
func TestSplashTickIsNoOpOnceCompleting(t *testing.T) {
	m := Model{splash: newSplashScreen()}
	m.splash.completing = true
	m.splash.progress.SetPercent(1.0)

	beforePercent := m.splash.progress.Percent()
	updated, _ := m.updateSplash(splashTickMsg{})
	m = updated.(Model)
	if m.splash.progress.Percent() != beforePercent {
		t.Fatal("expected splashTickMsg to be a no-op once completing")
	}
}

// TestSplashDoneMsgFinalizesUsingTheStoredReport proves the handoff from the
// splash screen to whatever comes next goes through the same
// finalizeHealthcheck logic the non-animated flow uses - checked here via
// the one branch that's deterministic without touching real adb/scrcpy
// resolution: a failed check must always land on screenHealthFailed.
func TestSplashDoneMsgFinalizesUsingTheStoredReport(t *testing.T) {
	m := Model{current: screenSplash}
	m.health.report = healthcheck.Report{Results: []healthcheck.Result{{Name: "x", Severity: healthcheck.Fail}}}
	m.health.done = true

	updated, _ := m.updateSplash(splashDoneMsg{})
	m = updated.(Model)

	if m.current != screenHealthFailed {
		t.Fatalf("expected a failed report to land on screenHealthFailed after the splash settle delay, got %v", m.current)
	}
}

// TestSplashFrameMsgUpdatesTheProgressBarWithoutPanicking exercises the
// spring-animation plumbing bubbles/progress relies on (SetPercent's
// returned command feeds FrameMsg back into Update) end to end, rather than
// just trusting the library.
func TestSplashFrameMsgUpdatesTheProgressBarWithoutPanicking(t *testing.T) {
	m := Model{splash: newSplashScreen()}
	setCmd := m.splash.progress.SetPercent(0.5)
	if setCmd == nil {
		t.Fatal("expected SetPercent to return an animation-frame command")
	}
	frameMsg := setCmd()
	if _, ok := frameMsg.(progress.FrameMsg); !ok {
		t.Fatalf("expected SetPercent's command to produce a progress.FrameMsg, got %T", frameMsg)
	}

	updated, cmd := m.updateSplash(frameMsg)
	m = updated.(Model)
	_ = cmd // may or may not be nil depending on spring equilibrium; just must not panic
	if m.splash.progress.Percent() != 0.5 {
		t.Fatalf("expected the target percent to remain 0.5 after a frame update, got %.4f", m.splash.progress.Percent())
	}
}

// TestFinalizeHealthcheckFailureAlwaysShowsFailedScreenRegardlessOfSetting
// proves UISettings.ShowHealthcheck can only ever suppress the *passing*
// results screen - a failure must never be hidden from the user no matter
// how that setting is configured, since they need to see what's wrong.
func TestFinalizeHealthcheckFailureAlwaysShowsFailedScreenRegardlessOfSetting(t *testing.T) {
	for _, showHealthcheck := range []bool{true, false} {
		m := Model{}
		m.settings.UI.ShowHealthcheck = showHealthcheck
		m.health.report = healthcheck.Report{Results: []healthcheck.Result{{Name: "x", Severity: healthcheck.Fail}}}

		got, cmd := m.finalizeHealthcheck()
		if got.current != screenHealthFailed {
			t.Fatalf("ShowHealthcheck=%v: expected screenHealthFailed, got %v", showHealthcheck, got.current)
		}
		if cmd != nil {
			t.Fatalf("ShowHealthcheck=%v: expected no follow-up command for a failed check", showHealthcheck)
		}
	}
}
