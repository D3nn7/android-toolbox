package avd

import (
	"os"
	"path/filepath"
	"strings"
)

// ConfigPath returns the path to an AVD's config.ini - a flat "key = value"
// file (no sections) holding its specs: RAM/heap size, LCD density, GPU
// mode, data partition size, skin, ...
func ConfigPath(avdHome, name string) string {
	return filepath.Join(avdHome, name+".avd", "config.ini")
}

// ReadConfig parses an AVD's config.ini into a plain key/value map.
func ReadConfig(avdHome, name string) (map[string]string, error) {
	data, err := os.ReadFile(ConfigPath(avdHome, name))
	if err != nil {
		return nil, err
	}
	cfg := map[string]string{}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		idx := strings.Index(line, "=")
		if idx < 0 {
			continue
		}
		cfg[strings.TrimSpace(line[:idx])] = strings.TrimSpace(line[idx+1:])
	}
	return cfg, nil
}

// WriteConfig patches the given key/value pairs into an AVD's config.ini,
// preserving every existing line (including comments and any key not
// mentioned in updates) and its original order; keys in updates that don't
// already exist in the file are appended.
func WriteConfig(avdHome, name string, updates map[string]string) error {
	path := ConfigPath(avdHome, name)
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	lines := strings.Split(string(data), "\n")
	applied := make(map[string]bool, len(updates))
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		idx := strings.Index(trimmed, "=")
		if idx < 0 {
			continue
		}
		key := strings.TrimSpace(trimmed[:idx])
		if v, ok := updates[key]; ok {
			lines[i] = key + " = " + v
			applied[key] = true
		}
	}
	for key, v := range updates {
		if !applied[key] {
			lines = append(lines, key+" = "+v)
		}
	}

	return os.WriteFile(path, []byte(strings.Join(lines, "\n")), 0o644)
}
