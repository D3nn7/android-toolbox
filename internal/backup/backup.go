// Package backup snapshots config files with a timestamp before they get
// overwritten, and can restore any prior snapshot back into place.
package backup

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// timeFormat avoids ":" (invalid in Windows filenames) while still sorting
// correctly as a plain string.
const timeFormat = "20060102-150405"

// Entry is one backup file found in a backup directory.
type Entry struct {
	Path         string
	OriginalName string
	Timestamp    time.Time
}

// Snapshot copies filePath into backupDir, suffixed with the current
// timestamp. It is a no-op (not an error) if filePath does not exist yet -
// there is nothing to protect before the very first write.
func Snapshot(backupDir, filePath string) error {
	data, err := os.ReadFile(filePath)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("backup of %s failed: %w", filePath, err)
	}
	if err := os.MkdirAll(backupDir, 0o755); err != nil {
		return err
	}
	name := fmt.Sprintf("%s.%s.bak", filepath.Base(filePath), time.Now().Format(timeFormat))
	return os.WriteFile(filepath.Join(backupDir, name), data, 0o644)
}

// BeforeWrite snapshots filePath and then, only if that succeeded, runs
// writeFn. Use this to wrap any place actions.yaml/settings.yaml gets
// overwritten so a bad write is always recoverable via `recover`.
func BeforeWrite(backupDir, filePath string, writeFn func() error) error {
	if err := Snapshot(backupDir, filePath); err != nil {
		return fmt.Errorf("backup failed, write aborted: %w", err)
	}
	return writeFn()
}

// List returns every backup entry in backupDir, newest first.
func List(backupDir string) ([]Entry, error) {
	files, err := os.ReadDir(backupDir)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	var entries []Entry
	for _, f := range files {
		if f.IsDir() {
			continue
		}
		name := f.Name()
		if !strings.HasSuffix(name, ".bak") {
			continue
		}
		trimmed := strings.TrimSuffix(name, ".bak")
		idx := strings.LastIndex(trimmed, ".")
		if idx < 0 {
			continue
		}
		original, tsStr := trimmed[:idx], trimmed[idx+1:]
		ts, err := time.Parse(timeFormat, tsStr)
		if err != nil {
			continue
		}
		entries = append(entries, Entry{
			Path:         filepath.Join(backupDir, name),
			OriginalName: original,
			Timestamp:    ts,
		})
	}

	sort.Slice(entries, func(i, j int) bool { return entries[i].Timestamp.After(entries[j].Timestamp) })
	return entries, nil
}

// Restore writes entry's content back to <configDir>/<entry.OriginalName>,
// first snapshotting whatever is currently there (so restoring is itself
// undoable).
func Restore(entry Entry, configDir, backupDir string) error {
	data, err := os.ReadFile(entry.Path)
	if err != nil {
		return fmt.Errorf("could not read backup file: %w", err)
	}
	target := filepath.Join(configDir, entry.OriginalName)
	if err := Snapshot(backupDir, target); err != nil {
		return err
	}
	return os.WriteFile(target, data, 0o644)
}
