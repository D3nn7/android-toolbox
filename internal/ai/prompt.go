package ai

import (
	_ "embed"
	"os"
)

//go:embed system_prompt.default.md
var DefaultSystemPrompt []byte

// LoadSystemPrompt reads the user-editable system prompt file, seeding it
// with the built-in default the first time it's requested - mirroring how
// actions.Load seeds actions.yaml.
func LoadSystemPrompt(path string) (string, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		if err := os.WriteFile(path, DefaultSystemPrompt, 0o644); err != nil {
			return "", err
		}
		return string(DefaultSystemPrompt), nil
	}
	if err != nil {
		return "", err
	}
	return string(data), nil
}
