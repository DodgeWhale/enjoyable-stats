package config

import (
	"fmt"
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	DiscordToken     string
	DiscordChannelID string
}

func Load() (*Config, error) {
	_ = godotenv.Load()

	token := os.Getenv("DISCORD_TOKEN")
	if token == "" {
		return nil, fmt.Errorf("config: DISCORD_TOKEN is required but not set")
	}

	return &Config{
		DiscordToken:     token,
		DiscordChannelID: os.Getenv("DISCORD_CHANNEL_ID"),
	}, nil
}

// LoadOptional loads config without requiring DISCORD_TOKEN (for local debug runs).
func LoadOptional() (*Config, error) {
	_ = godotenv.Load()
	return &Config{
		DiscordToken:     os.Getenv("DISCORD_TOKEN"),
		DiscordChannelID: os.Getenv("DISCORD_CHANNEL_ID"),
	}, nil
}
