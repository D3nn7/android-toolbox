package adb

import "testing"

// realDevicesOutput is a verbatim capture of "adb devices -l" against two
// real tablets used during development of this tool.
const realDevicesOutput = `List of devices attached
5200f2fac0fa2761       device product:gtactive2lteeea model:SM_T395 device:gtactive2lte transport_id:2
R52WC07YCWH            device product:gtactive3eea model:SM_T575 device:gtactive3 transport_id:1

`

func TestParseDevicesOutputRealCapture(t *testing.T) {
	devices := ParseDevicesOutput(realDevicesOutput)
	if len(devices) != 2 {
		t.Fatalf("expected 2 devices, got %d: %+v", len(devices), devices)
	}

	first := devices[0]
	if first.Serial != "5200f2fac0fa2761" || first.State != "device" || first.Model != "SM_T395" || first.TransportID != "2" {
		t.Fatalf("unexpected first device: %+v", first)
	}

	second := devices[1]
	if second.Serial != "R52WC07YCWH" || second.Model != "SM_T575" || !second.Connected() {
		t.Fatalf("unexpected second device: %+v", second)
	}
}

func TestParseDevicesOutputHandlesUnauthorizedAndOffline(t *testing.T) {
	out := "List of devices attached\n" +
		"ABC123          unauthorized usb:1-1 transport_id:3\n" +
		"emulator-5554   offline\n"

	devices := ParseDevicesOutput(out)
	if len(devices) != 2 {
		t.Fatalf("expected 2 devices, got %d: %+v", len(devices), devices)
	}
	if devices[0].Connected() {
		t.Fatal("unauthorized device should not report Connected()")
	}
	if devices[1].State != "offline" {
		t.Fatalf("expected offline state, got %q", devices[1].State)
	}
}

func TestParseDevicesOutputEmpty(t *testing.T) {
	devices := ParseDevicesOutput("List of devices attached\n\n")
	if len(devices) != 0 {
		t.Fatalf("expected 0 devices, got %d", len(devices))
	}
}
