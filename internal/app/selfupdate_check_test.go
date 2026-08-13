package app

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"android-toolbox/internal/buildinfo"
	"android-toolbox/internal/config"
)

func TestUpdateNoticeTextEmptyWhenNoVersionKnown(t *testing.T) {
	if got := updateNoticeText(uiTextEN, ""); got != "" {
		t.Fatalf("expected no notice when no version is known, got %q", got)
	}
}

func TestUpdateNoticeTextEmptyWhenNotNewer(t *testing.T) {
	if got := updateNoticeText(uiTextEN, buildinfo.Version); got != "" {
		t.Fatalf("expected no notice when the known version isn't newer than the running one, got %q", got)
	}
}

func TestUpdateNoticeTextShowsBothVersionsWhenNewer(t *testing.T) {
	got := updateNoticeText(uiTextEN, "999.0.0")
	if got == "" {
		t.Fatal("expected a notice for a genuinely newer version")
	}
	if !strings.Contains(got, "999.0.0") || !strings.Contains(got, buildinfo.Version) {
		t.Fatalf("expected the notice to mention both the new and current version, got %q", got)
	}
}

// TestCheckForSelfUpdateCmdUsesCacheWithinInterval proves a recent check
// short-circuits to the cached version without ever attempting a network
// call - the only branch of checkForSelfUpdateCmd that's deterministically
// testable without a live GitHub connection (the "cache is stale, actually
// query GitHub" branch, like this project's other network-calling code,
// isn't unit-tested).
func TestCheckForSelfUpdateCmdUsesCacheWithinInterval(t *testing.T) {
	paths := config.Paths{StateFile: filepath.Join(t.TempDir(), "state.json")}
	state := config.State{
		LastUpdateCheckAt:  time.Now().UTC().Add(-1 * time.Hour).Format(time.RFC3339),
		LatestKnownVersion: "9.9.9",
	}

	cmd := checkForSelfUpdateCmd(context.Background(), paths, state)
	msg := cmd()

	got, ok := msg.(selfUpdateCheckMsg)
	if !ok {
		t.Fatalf("expected a selfUpdateCheckMsg, got %T", msg)
	}
	if got.version != "9.9.9" {
		t.Fatalf("expected the cached version to be returned as-is, got %q", got.version)
	}
}

func TestCheckForSelfUpdateCmdCacheHitDoesNotRewriteState(t *testing.T) {
	stateFile := filepath.Join(t.TempDir(), "state.json")
	paths := config.Paths{StateFile: stateFile}
	state := config.State{
		LastUpdateCheckAt:  time.Now().UTC().Add(-1 * time.Minute).Format(time.RFC3339),
		LatestKnownVersion: "9.9.9",
	}

	checkForSelfUpdateCmd(context.Background(), paths, state)()

	if _, err := os.Stat(stateFile); err == nil {
		t.Fatal("expected a cache hit to never write state.json - nothing changed, nothing to persist")
	}
}
