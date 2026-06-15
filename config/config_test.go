package config_test

import (
	"os"
	"testing"

	"github.com/DodgeWhale/enjoyable-stats/config"
)

func TestLoad_returnsTokenAndChannelFromEnv(t *testing.T) {
	os.Setenv("DISCORD_TOKEN", "test-token-abc")
	os.Setenv("DISCORD_CHANNEL_ID", "test-channel-xyz")
	defer os.Unsetenv("DISCORD_TOKEN")
	defer os.Unsetenv("DISCORD_CHANNEL_ID")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if cfg.DiscordToken != "test-token-abc" {
		t.Errorf("DiscordToken = %q, want %q", cfg.DiscordToken, "test-token-abc")
	}
	if cfg.DiscordChannelID != "test-channel-xyz" {
		t.Errorf("DiscordChannelID = %q, want %q", cfg.DiscordChannelID, "test-channel-xyz")
	}
}
