package config

import (
	"os"
	"strconv"
	"time"
)

type Config struct {
	TestsDir          string
	Port              string
	DBPath            string
	UseNoVNC          bool
	RunTimeout        time.Duration
	MaxConcurrentRuns int
}

func Load() *Config {
	minutes := 30
	if v := os.Getenv("RUN_TIMEOUT_MINUTES"); v != "" {
		if m, err := strconv.Atoi(v); err == nil {
			minutes = m
		}
	}
	maxConcurrent := 1
	if v := os.Getenv("MAX_CONCURRENT_RUNS"); v != "" {
		if m, err := strconv.Atoi(v); err == nil {
			maxConcurrent = m
		}
	}
	return &Config{
		TestsDir: env("TESTS_DIR", "../tests"),
		Port:     env("PORT", ":8080"),
		DBPath:   env("DB_PATH", "./runner.db"),
		// USE_NOVNC 環境変数が "true" に設定されている場合にはLocalで起動しないといけない
		UseNoVNC:          os.Getenv("USE_NOVNC") == "true",
		RunTimeout:        time.Duration(minutes) * time.Minute,
		MaxConcurrentRuns: maxConcurrent,
	}
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
