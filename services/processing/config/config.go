package config

import (
	"log/slog"
	"os"
)

type Config struct {
	DatabaseURL    string
	TogetherAPIKey string
}

func Load() Config {
	dbURL := os.Getenv("DATABASE_URL")
	apiKey := os.Getenv("TOGETHER_AI_API_KEY")
	if dbURL == "" || apiKey == "" {
		slog.Error("DATABASE_URL and TOGETHER_AI_API_KEY are required")
		os.Exit(1)
	}
	return Config{DatabaseURL: dbURL, TogetherAPIKey: apiKey}
}
