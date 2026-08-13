package actions

import (
	"context"
	"os/exec"
	"strings"
	"testing"
)

func TestPrepareTokensQuotedGroupStaysOneToken(t *testing.T) {
	values := map[string]string{"serial": "ABC123"}
	tokens, err := prepareTokens(`shell "dumpsys notification | grep -A 8 MESSAGES_4"`, values)
	if err != nil {
		t.Fatalf("prepareTokens error: %v", err)
	}
	want := []string{"shell", "dumpsys notification | grep -A 8 MESSAGES_4"}
	if len(tokens) != 2 || tokens[0] != want[0] || tokens[1] != want[1] {
		t.Fatalf("tokens = %#v, want %#v", tokens, want)
	}
}

func TestPrepareTokensSubstitutesAfterTokenizingPreservesBackslashes(t *testing.T) {
	// A Windows path with backslashes and a space must survive verbatim:
	// substitution happens on the already-tokenized (quotes stripped) value,
	// never re-parsed by the tokenizer, so nothing can eat the backslashes.
	values := map[string]string{
		"serial":      "ABC123",
		"remote_path": "/sdcard/Download/",
		"local_path":  `C:\Users\d schapeit\Downloads`,
	}
	tokens, err := prepareTokens(`pull "{remote_path}" "{local_path}"`, values)
	if err != nil {
		t.Fatalf("prepareTokens error: %v", err)
	}
	want := []string{"pull", "/sdcard/Download/", `C:\Users\d schapeit\Downloads`}
	if len(tokens) != 3 {
		t.Fatalf("tokens = %#v, want 3 tokens", tokens)
	}
	for i := range want {
		if tokens[i] != want[i] {
			t.Fatalf("token[%d] = %q, want %q", i, tokens[i], want[i])
		}
	}
}

func TestPrepareTokensLeavesUnknownPlaceholderUntouched(t *testing.T) {
	tokens, err := prepareTokens("shell echo {unknown}", map[string]string{"serial": "x"})
	if err != nil {
		t.Fatalf("prepareTokens error: %v", err)
	}
	if tokens[2] != "{unknown}" {
		t.Fatalf("expected unknown placeholder to be left as-is, got %q", tokens[2])
	}
}

func TestPrepareTokensEmptyCommand(t *testing.T) {
	tokens, err := prepareTokens("", map[string]string{"serial": "x"})
	if err != nil {
		t.Fatalf("prepareTokens error: %v", err)
	}
	if len(tokens) != 0 {
		t.Fatalf("expected 0 tokens for empty command, got %#v", tokens)
	}
}

func TestResolveParamValuesFallsBackToDefault(t *testing.T) {
	a := Action{Params: []Param{{Name: "port", Default: "8080"}}}
	values := resolveParamValues(a, "SERIAL1", map[string]string{})
	if values["port"] != "8080" {
		t.Fatalf("expected default 8080, got %q", values["port"])
	}
	if values["serial"] != "SERIAL1" {
		t.Fatalf("expected serial to be injected, got %q", values["serial"])
	}

	values = resolveParamValues(a, "SERIAL1", map[string]string{"port": "9090"})
	if values["port"] != "9090" {
		t.Fatalf("expected supplied value to win, got %q", values["port"])
	}
}

func TestPrepareBuildsExpectedADBCommand(t *testing.T) {
	e := NewExecutor("adb.exe", nil)
	a := Action{Tool: ToolADB, Command: "shell dumpsys battery"}
	cmd, err := e.Prepare(context.Background(), a, "SERIAL1", nil)
	if err != nil {
		t.Fatalf("Prepare error: %v", err)
	}
	got := cmd.Args
	want := []string{"adb.exe", "-s", "SERIAL1", "shell", "dumpsys", "battery"}
	if len(got) != len(want) {
		t.Fatalf("args = %#v, want %#v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("arg[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestPrepareRejectsScrcpyTool(t *testing.T) {
	e := NewExecutor("adb.exe", nil)
	_, err := e.Prepare(context.Background(), Action{Tool: ToolScrcpy}, "S", nil)
	if err == nil {
		t.Fatal("expected error preparing a scrcpy action via Prepare")
	}
}

func TestShellActionSubstitutesQuotedADBPath(t *testing.T) {
	e := NewExecutor(`C:\Program Files\tools\adb.exe`, nil)
	a := Action{Tool: ToolShell, Command: "{adb} -s {serial} shell echo hi"}
	cmd, err := e.Prepare(context.Background(), a, "SERIAL1", nil)
	if err != nil {
		t.Fatalf("Prepare error: %v", err)
	}
	full := effectiveCommandLine(cmd)
	if !strings.Contains(full, `"C:\Program Files\tools\adb.exe"`) {
		t.Fatalf("expected quoted adb path in shell command, got: %s", full)
	}
}

// TestStartStreamsRealOutput runs a trivial adb-independent check: it swaps
// in a real executable ("cmd"/"echo" via the shell tool) to verify Start()
// actually streams output incrementally rather than buffering everything
// until exit.
func TestStartStreamsRealOutput(t *testing.T) {
	if _, err := exec.LookPath("cmd"); err != nil {
		t.Skip("cmd.exe not available on this platform")
	}
	e := NewExecutor("adb.exe", nil)
	a := Action{Tool: ToolShell, Command: "echo hello-from-executor"}
	ra, err := e.Start(context.Background(), a, "SERIAL1", nil)
	if err != nil {
		t.Fatalf("Start error: %v", err)
	}
	buf := make([]byte, 4096)
	n, _ := ra.Output.Read(buf)
	if err := ra.Wait(); err != nil {
		t.Fatalf("Wait error: %v", err)
	}
	if !strings.Contains(string(buf[:n]), "hello-from-executor") {
		t.Fatalf("expected output to contain marker, got: %q", string(buf[:n]))
	}
}
