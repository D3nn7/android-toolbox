//go:build windows

package actions

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

// TestShellCommandHandlesChainedQuotedPaths is a regression test for a real
// bug found via live testing: cmd.exe's /C quote-stripping rule only
// preserves inner quotes verbatim in the narrow "exactly two quotes, no
// special chars" case. A composite command chaining multiple quoted paths
// with "&&" has more than two quotes plus a special character, so cmd falls
// back to stripping the first quote and the last quote *anywhere* in the
// line - silently unbalancing everything in between - unless the whole
// command is wrapped in one extra pair of outer quotes (what shellCommand
// does). This runs the real cmd.exe to prove the wrapping actually works,
// not just that the string looks right.
func TestShellCommandHandlesChainedQuotedPaths(t *testing.T) {
	dir := t.TempDir()
	command := `echo "first" && echo "second" && echo "third"`
	cmd := shellCommand(context.Background(), command)

	var out bytes.Buffer
	cmd.Dir = dir
	cmd.Stdout = &out
	cmd.Stderr = &out
	if err := cmd.Run(); err != nil {
		t.Fatalf("cmd.Run() failed: %v\noutput: %s", err, out.String())
	}

	got := out.String()
	for _, want := range []string{"first", "second", "third"} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected output to contain %q, got: %s", want, got)
		}
	}
}
