package adb

import "testing"

// realBatteryOutput is a verbatim capture of "dumpsys battery" from a real
// Samsung tablet used during development of this tool. Real devices emit a
// lot of vendor-specific noise beyond the standard AOSP fields - the parser
// must ignore all of that without choking on it.
const realBatteryOutput = `Current Battery Service state:
  AC powered: false
  USB powered: true
  Wireless powered: false
  Max charging current: 0
  Max charging voltage: 0
  Charge counter: 4370000
  status: 5
  health: 2
  present: true
  level: 100
  scale: 100
  voltage: 4179
  temperature: 273
  technology: Li-ion
  batteryMiscEvent: 0
FEATURE_WIRELESS_FAST_CHARGER_CONTROL: true
LLB CAL: 20231225
BatteryInfoBackUp
  mSavedBatteryAsoc: 93
`

func TestParseDumpsysBatteryRealCapture(t *testing.T) {
	b := ParseDumpsysBattery(realBatteryOutput)
	if b.Level != 100 || b.Scale != 100 {
		t.Fatalf("unexpected level/scale: %+v", b)
	}
	if b.Status != 5 || b.StatusText() != "full" {
		t.Fatalf("unexpected status: %+v", b)
	}
	if !b.USBPowered || b.ACPowered {
		t.Fatalf("unexpected power flags: %+v", b)
	}
	if b.Technology != "Li-ion" {
		t.Fatalf("unexpected technology: %q", b.Technology)
	}
	if !b.Charging() {
		t.Fatal("expected Charging() to be true when USB powered")
	}
}

func TestParseDumpsysBatteryDischarging(t *testing.T) {
	out := "  AC powered: false\n  USB powered: false\n  status: 3\n  level: 42\n  scale: 100\n"
	b := ParseDumpsysBattery(out)
	if b.Charging() {
		t.Fatal("expected Charging() to be false while discharging and unpowered")
	}
	if b.Level != 42 {
		t.Fatalf("unexpected level: %d", b.Level)
	}
}
