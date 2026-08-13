package config

import (
	"path/filepath"
	"testing"
)

func statePaths(t *testing.T) Paths {
	t.Helper()
	dir := t.TempDir()
	return Paths{StateFile: filepath.Join(dir, "state.json")}
}

func TestLoadStateReturnsZeroValueWhenMissing(t *testing.T) {
	p := statePaths(t)

	s, err := LoadState(p)
	if err != nil {
		t.Fatalf("LoadState returned an error: %v", err)
	}
	if !s.IsFirstRun() {
		t.Errorf("expected a missing state file to report IsFirstRun() == true")
	}
}

func TestSaveAndLoadStateRoundTrip(t *testing.T) {
	p := statePaths(t)

	want := State{
		PathInstallAsked:   true,
		PathInstalled:      true,
		LatestKnownVersion: "1.2.3",
	}
	if err := SaveState(p, want); err != nil {
		t.Fatalf("SaveState returned an error: %v", err)
	}

	got, err := LoadState(p)
	if err != nil {
		t.Fatalf("LoadState returned an error: %v", err)
	}
	if got != want {
		t.Fatalf("round trip mismatch: got %+v, want %+v", got, want)
	}
}

func TestMarkFirstRunCompleteEndsFirstRun(t *testing.T) {
	p := statePaths(t)

	s, err := MarkFirstRunComplete(p, State{})
	if err != nil {
		t.Fatalf("MarkFirstRunComplete returned an error: %v", err)
	}
	if s.IsFirstRun() {
		t.Fatal("expected IsFirstRun() to be false after MarkFirstRunComplete")
	}
	if s.FirstRunCompletedAt == "" {
		t.Fatal("expected FirstRunCompletedAt to be set")
	}

	persisted, err := LoadState(p)
	if err != nil {
		t.Fatalf("LoadState returned an error: %v", err)
	}
	if persisted != s {
		t.Fatalf("expected the persisted state to match the returned state, got %+v vs %+v", persisted, s)
	}
}
