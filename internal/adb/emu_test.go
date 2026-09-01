package adb

import "testing"

func TestIsEmulatorSerial(t *testing.T) {
	cases := []struct {
		serial string
		want   bool
	}{
		{"emulator-5554", true},
		{"emulator-5556", true},
		{"5200f2fac0fa2761", false},
		{"R52WC07YCWH", false},
		{"emulator", false},
		{"emulator-", false},
		{"192.168.1.5:5555", false},
	}
	for _, c := range cases {
		if got := IsEmulatorSerial(c.serial); got != c.want {
			t.Errorf("IsEmulatorSerial(%q) = %v, want %v", c.serial, got, c.want)
		}
	}
}
