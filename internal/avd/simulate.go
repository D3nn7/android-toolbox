package avd

import (
	"context"
	"fmt"

	"android-toolbox/internal/adb"
)

// NetworkSpeedProfiles/NetworkDelayProfiles are the fixed profile names the
// emulator console's "network speed"/"network delay" commands accept -
// exposed here so the TUI's simulation form can offer a select rather than
// free text.
var (
	NetworkSpeedProfiles = []string{"gsm", "edge", "umts", "hsdpa", "lte", "evdo", "full"}
	NetworkDelayProfiles = []string{"none", "gprs", "edge", "umts"}
)

// SetGPS sets the emulator's simulated GPS fix via the console's "geo fix"
// command, which - unusually - takes longitude before latitude.
func SetGPS(ctx context.Context, client *adb.Client, serial string, lat, lon float64) error {
	_, err := client.Emu(ctx, serial, "geo", "fix", fmt.Sprintf("%g", lon), fmt.Sprintf("%g", lat))
	return err
}

// SetNetworkSpeed simulates a network throughput profile (one of
// NetworkSpeedProfiles).
func SetNetworkSpeed(ctx context.Context, client *adb.Client, serial, profile string) error {
	_, err := client.Emu(ctx, serial, "network", "speed", profile)
	return err
}

// SetNetworkDelay simulates a network latency profile (one of
// NetworkDelayProfiles).
func SetNetworkDelay(ctx context.Context, client *adb.Client, serial, profile string) error {
	_, err := client.Emu(ctx, serial, "network", "delay", profile)
	return err
}

// SetBattery simulates a battery level and charging state.
func SetBattery(ctx context.Context, client *adb.Client, serial string, percent int, charging bool) error {
	if _, err := client.Emu(ctx, serial, "power", "capacity", fmt.Sprintf("%d", percent)); err != nil {
		return err
	}
	state := "off"
	if charging {
		state = "on"
	}
	_, err := client.Emu(ctx, serial, "power", "ac", state)
	return err
}

// Rotate toggles the emulator's simulated screen orientation.
func Rotate(ctx context.Context, client *adb.Client, serial string) error {
	_, err := client.Emu(ctx, serial, "rotate")
	return err
}

// SendSMS simulates an incoming SMS from a given sender number.
func SendSMS(ctx context.Context, client *adb.Client, serial, from, text string) error {
	_, err := client.Emu(ctx, serial, "sms", "send", from, text)
	return err
}

// PlaceCall simulates an incoming voice call from a given number.
func PlaceCall(ctx context.Context, client *adb.Client, serial, number string) error {
	_, err := client.Emu(ctx, serial, "gsm", "call", number)
	return err
}
