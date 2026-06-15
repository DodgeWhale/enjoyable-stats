package bot_test

import (
	"testing"

	"github.com/DodgeWhale/enjoyable-stats/bot"
)

func TestValidateSteamID_acceptsValid17DigitID(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  bool
	}{
		{"valid 17-digit ID", "76561198012345678", true},
		{"too short", "7656119801234567", false},
		{"too long", "765611980123456789", false},
		{"contains letters", "7656119801234567A", false},
		{"empty string", "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := bot.ValidateSteamID(tc.input); got != tc.want {
				t.Errorf("ValidateSteamID(%q) = %v, want %v", tc.input, got, tc.want)
			}
		})
	}
}
