package config

import (
	"errors"
	"log/slog"
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	Homeserver     string
	BotAccessToken string
	RoomID         string
	HumanUserID    string
}

func Load() (*Config, error) {
	if err := godotenv.Load(); err != nil {
		slog.Info("No .env file found, using system environment variables")
	} else {
		slog.Info(".env file loaded")
	}

	cfg := &Config{
		Homeserver:     getEnv("MATRIX_HOMESERVER", "https://matrix.mopore.org"),
		BotAccessToken: getEnv("MATRIX_BOT_ACCESS_TOKEN", ""),
		RoomID:         getEnv("MATRIX_ROOM_ID", ""),
		HumanUserID:    getEnv("MATRIX_HUMAN_ID", "@jni:matrix.mopore.org"),
	}

	if cfg.BotAccessToken == "" {
		return nil, errors.New("missing MATRIX_BOT_ACCESS_TOKEN")
	}
	if cfg.RoomID == "" {
		return nil, errors.New("missing MATRIX_ROOM_ID")
	}

	return cfg, nil
}

func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}
