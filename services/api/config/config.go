package config

import (
	"log/slog"
	"os"
)

type Config struct {
	DatabaseURL    string
	TogetherAPIKey string
	Port           string
}

func Load() Config {
	dbURL := os.Getenv("DATABASE_URL")
	apiKey := os.Getenv("TOGETHER_AI_API_KEY")
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	if dbURL == "" || apiKey == "" {
		slog.Error("DATABASE_URL and TOGETHER_AI_API_KEY are required")
		os.Exit(1)
	}
	return Config{DatabaseURL: dbURL, TogetherAPIKey: apiKey, Port: port}
}
