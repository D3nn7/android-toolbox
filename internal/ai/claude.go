package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

func init() {
	Register("claude", newClaudeProvider)
}

// claudeProvider shells out to the Claude Code CLI's non-interactive print
// mode (claude -p ... --output-format json) to turn a free-text request
// into a strict-JSON ActionDraft.
type claudeProvider struct {
	command        string
	timeoutSeconds int
	promptPath     string
}

func newClaudeProvider(command string, timeoutSeconds int, promptPath string) (Provider, error) {
	if command == "" {
		command = "claude"
	}
	return &claudeProvider{command: command, timeoutSeconds: timeoutSeconds, promptPath: promptPath}, nil
}

func (p *claudeProvider) Name() string { return "claude" }

func (p *claudeProvider) Available() error {
	if _, err := exec.LookPath(p.command); err != nil {
		return fmt.Errorf("claude CLI (%s) not found on PATH: %w", p.command, err)
	}
	return nil
}

// disallowedTools keeps this call to a single, side-effect-free text
// completion: we only want a JSON object back, never file edits, shell
// commands, or web access run on the user's behalf.
var disallowedTools = []string{
	"Bash", "Read", "Write", "Edit", "MultiEdit", "NotebookEdit",
	"WebFetch", "WebSearch", "Glob", "Grep", "Task",
}

func (p *claudeProvider) GenerateAction(ctx context.Context, req GenerateRequest) (ActionDraft, error) {
	systemPrompt, err := LoadSystemPrompt(p.promptPath)
	if err != nil {
		return ActionDraft{}, fmt.Errorf("could not load system prompt: %w", err)
	}

	timeout := time.Duration(p.timeoutSeconds) * time.Second
	if timeout <= 0 {
		timeout = 120 * time.Second
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// The prompt must come first: claude's arg parser treats variadic flags
	// like --disallowedTools as greedily consuming every following
	// non-flag token, so a trailing positional prompt gets silently
	// swallowed into the tools list instead of being read as the prompt.
	args := []string{
		buildUserPrompt(req),
		"--print",
		"--output-format", "json",
		"--system-prompt", systemPrompt,
		"--disallowedTools", strings.Join(disallowedTools, ","),
	}

	cmd := exec.CommandContext(ctx, p.command, args...)
	out, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return ActionDraft{}, fmt.Errorf("claude cli failed: %w (stderr: %s)", err, strings.TrimSpace(string(exitErr.Stderr)))
		}
		return ActionDraft{}, fmt.Errorf("could not run claude cli: %w", err)
	}

	var envelope struct {
		IsError bool   `json:"is_error"`
		Result  string `json:"result"`
	}
	if err := json.Unmarshal(out, &envelope); err != nil {
		return ActionDraft{}, fmt.Errorf("could not parse claude cli response: %w", err)
	}
	if envelope.IsError {
		return ActionDraft{}, fmt.Errorf("claude reported an error: %s", envelope.Result)
	}

	return parseActionDraft(envelope.Result)
}

// hostOSDescription names the OS the generated "tool: shell" command will
// actually execute on, in the same words used by the system prompt's "Host
// OS accuracy" section - so the model tailors shell syntax (cmd.exe vs sh)
// to the machine android-toolbox is running on right now, not a generic
// cross-platform guess.
func hostOSDescription() string {
	switch runtime.GOOS {
	case "windows":
		return "Windows (tool: shell runs via cmd.exe)"
	case "darwin":
		return "macOS (tool: shell runs via sh)"
	default:
		return "Linux (tool: shell runs via sh)"
	}
}

func buildUserPrompt(req GenerateRequest) string {
	var b strings.Builder
	b.WriteString("Host OS for this action: ")
	b.WriteString(hostOSDescription())
	b.WriteString("\n\n")
	if req.ExistingAction != nil {
		existingJSON, _ := json.Marshal(req.ExistingAction)
		b.WriteString("You are revising the following existing action. Keep its \"id\" field exactly as-is and only change what the user request asks for:\n")
		b.Write(existingJSON)
		b.WriteString("\n\n")
	} else if len(req.ExistingIDs) > 0 {
		b.WriteString("Action IDs already in use (do not reuse): ")
		b.WriteString(strings.Join(req.ExistingIDs, ", "))
		b.WriteString("\n\n")
	}
	b.WriteString("User request: ")
	b.WriteString(req.UserPrompt)
	b.WriteString("\n\nRespond only with the action's JSON object.")
	return b.String()
}

// parseActionDraft is tolerant of the model wrapping its JSON in a markdown
// fence despite instructions not to: it extracts the outermost {...} object
// before decoding.
func parseActionDraft(text string) (ActionDraft, error) {
	obj := extractJSONObject(text)
	var d ActionDraft
	if err := json.Unmarshal([]byte(obj), &d); err != nil {
		return ActionDraft{}, fmt.Errorf("AI response is not a valid JSON object: %w\nResponse: %s", err, text)
	}
	return d, nil
}

func extractJSONObject(text string) string {
	start := strings.IndexByte(text, '{')
	end := strings.LastIndexByte(text, '}')
	if start < 0 || end < 0 || end < start {
		return text
	}
	return text[start : end+1]
}
