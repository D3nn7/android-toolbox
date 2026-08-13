package ai

import "fmt"

// Factory builds a Provider given the resolved CLI command name, a timeout,
// and the path to the (user-editable) system prompt file.
type Factory func(command string, timeoutSeconds int, promptPath string) (Provider, error)

var registry = map[string]Factory{}

// Register adds a provider factory under name. Called from each provider
// implementation's init(), so adding a new AI backend later is just a new
// file with its own init() - no changes needed here or at call sites.
func Register(name string, factory Factory) {
	registry[name] = factory
}

// New builds the named provider. Returns an error for an unknown name
// rather than panicking, since the name ultimately comes from user-editable
// settings.yaml.
func New(name, command string, timeoutSeconds int, promptPath string) (Provider, error) {
	factory, ok := registry[name]
	if !ok {
		return nil, fmt.Errorf("unbekannter KI-Provider %q (verfügbar: %v)", name, Names())
	}
	return factory(command, timeoutSeconds, promptPath)
}

// Names lists every registered provider name.
func Names() []string {
	names := make([]string, 0, len(registry))
	for n := range registry {
		names = append(names, n)
	}
	return names
}
