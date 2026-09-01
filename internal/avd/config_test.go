package avd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadWriteConfigRoundTrip(t *testing.T) {
	avdHome := t.TempDir()
	name := "Test_AVD"
	avdDir := filepath.Join(avdHome, name+".avd")
	if err := os.MkdirAll(avdDir, 0o755); err != nil {
		t.Fatal(err)
	}

	original := "avd.ini.encoding=UTF-8\n" +
		"AvdId=Test_AVD\n" +
		"hw.ramSize=1536\n" +
		"# a comment that must survive\n" +
		"hw.lcd.density=420\n"
	if err := os.WriteFile(ConfigPath(avdHome, name), []byte(original), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := ReadConfig(avdHome, name)
	if err != nil {
		t.Fatal(err)
	}
	if cfg["hw.ramSize"] != "1536" || cfg["hw.lcd.density"] != "420" || cfg["AvdId"] != "Test_AVD" {
		t.Fatalf("unexpected parsed config: %+v", cfg)
	}

	if err := WriteConfig(avdHome, name, map[string]string{
		"hw.ramSize":  "2048",
		"vm.heapSize": "512", // new key, not present in the original file
	}); err != nil {
		t.Fatal(err)
	}

	updated, err := ReadConfig(avdHome, name)
	if err != nil {
		t.Fatal(err)
	}
	if updated["hw.ramSize"] != "2048" {
		t.Fatalf("expected hw.ramSize to be updated to 2048, got %q", updated["hw.ramSize"])
	}
	if updated["vm.heapSize"] != "512" {
		t.Fatalf("expected new key vm.heapSize to be appended, got %q", updated["vm.heapSize"])
	}
	if updated["hw.lcd.density"] != "420" || updated["AvdId"] != "Test_AVD" {
		t.Fatalf("expected untouched keys to survive the write, got %+v", updated)
	}

	raw, err := os.ReadFile(ConfigPath(avdHome, name))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), "# a comment that must survive") {
		t.Fatalf("expected the comment line to survive WriteConfig, got:\n%s", raw)
	}
}
