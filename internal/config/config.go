package config

import (
	"os"
	"time"
)

type Config struct {
	ListenAddr        string
	StateFile         string
	RetentionInterval time.Duration
	RetentionAge      time.Duration
}

func Load() Config {
	return Config{
		ListenAddr:        value("LISTEN_ADDR", ":8080"),
		StateFile:         value("STATE_FILE", "./data/state.json"),
		RetentionInterval: duration("RETENTION_INTERVAL", time.Hour),
		RetentionAge:      duration("RETENTION_AGE", 30*24*time.Hour),
	}
}

func value(key, fallback string) string {
	if configured := os.Getenv(key); configured != "" {
		return configured
	}
	return fallback
}

func duration(key string, fallback time.Duration) time.Duration {
	parsed, err := time.ParseDuration(os.Getenv(key))
	if err != nil || parsed <= 0 {
		return fallback
	}
	return parsed
}
