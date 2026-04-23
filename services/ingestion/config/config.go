package config

import (
	"log/slog"
	"os"
)

type Config struct {
	DatabaseURL string
}

func Load() Config {
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		slog.Error("DATABASE_URL is required")
		os.Exit(1)
	}
	return Config{DatabaseURL: url}
}
