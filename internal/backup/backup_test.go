package backup

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestSnapshotNoopWhenSourceMissing(t *testing.T) {
	dir := t.TempDir()
	backupDir := filepath.Join(dir, ".backup")
	if err := Snapshot(backupDir, filepath.Join(dir, "does-not-exist.yaml")); err != nil {
		t.Fatalf("expected no error for missing source, got %v", err)
	}
	if _, err := os.Stat(backupDir); !os.IsNotExist(err) {
		t.Fatal("expected backup dir to not be created when source is missing")
	}
}

func TestSnapshotAndList(t *testing.T) {
	dir := t.TempDir()
	backupDir := filepath.Join(dir, ".backup")
	target := filepath.Join(dir, "actions.yaml")
	if err := os.WriteFile(target, []byte("v1"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := Snapshot(backupDir, target); err != nil {
		t.Fatalf("Snapshot error: %v", err)
	}

	entries, err := List(backupDir)
	if err != nil {
		t.Fatalf("List error: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 backup entry, got %d", len(entries))
	}
	if entries[0].OriginalName != "actions.yaml" {
		t.Fatalf("unexpected original name: %q", entries[0].OriginalName)
	}
	data, err := os.ReadFile(entries[0].Path)
	if err != nil || string(data) != "v1" {
		t.Fatalf("unexpected backup content: %q, err=%v", data, err)
	}
}

func TestListOrdersNewestFirst(t *testing.T) {
	dir := t.TempDir()
	older := Entry{Path: filepath.Join(dir, "actions.yaml.20250101-000000.bak"), OriginalName: "actions.yaml"}
	newer := Entry{Path: filepath.Join(dir, "actions.yaml.20260101-000000.bak"), OriginalName: "actions.yaml"}
	for _, e := range []Entry{older, newer} {
		if err := os.WriteFile(e.Path, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	entries, err := List(dir)
	if err != nil {
		t.Fatalf("List error: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}
	if !entries[0].Timestamp.After(entries[1].Timestamp) {
		t.Fatalf("expected newest-first order, got %+v", entries)
	}
}

func TestBeforeWriteSkipsWriteOnBackupFailure(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "actions.yaml")
	if err := os.WriteFile(target, []byte("v1"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Make the backup dir a path that collides with an existing file so
	// MkdirAll inside Snapshot fails deterministically.
	blockedBackupDir := filepath.Join(dir, "blocked")
	if err := os.WriteFile(blockedBackupDir, []byte("not a dir"), 0o644); err != nil {
		t.Fatal(err)
	}

	writeCalled := false
	err := BeforeWrite(blockedBackupDir, target, func() error {
		writeCalled = true
		return nil
	})
	if err == nil {
		t.Fatal("expected error when backup fails")
	}
	if writeCalled {
		t.Fatal("writeFn must not run when the backup step fails")
	}
}

func TestRestoreWritesBackContentAndSnapshotsCurrentFirst(t *testing.T) {
	dir := t.TempDir()
	backupDir := filepath.Join(dir, ".backup")
	target := filepath.Join(dir, "actions.yaml")

	if err := os.WriteFile(target, []byte("current"), 0o644); err != nil {
		t.Fatal(err)
	}

	oldEntry := Entry{
		Path:         filepath.Join(backupDir, "actions.yaml.20200101-000000.bak"),
		OriginalName: "actions.yaml",
		Timestamp:    time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC),
	}
	if err := os.MkdirAll(backupDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(oldEntry.Path, []byte("restored-content"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := Restore(oldEntry, dir, backupDir); err != nil {
		t.Fatalf("Restore error: %v", err)
	}

	data, err := os.ReadFile(target)
	if err != nil || string(data) != "restored-content" {
		t.Fatalf("expected restored content, got %q, err=%v", data, err)
	}

	entries, err := List(backupDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected the pre-restore state to have been snapshotted too, got %d entries", len(entries))
	}
}
