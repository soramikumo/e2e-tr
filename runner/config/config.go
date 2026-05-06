package config

import "os"

type Config struct {
	TestsDir string
	Port     string
	DBPath   string
}

func Load() *Config {
	return &Config{
		TestsDir: env("TESTS_DIR", "../tests"),
		Port:     env("PORT", ":8080"),
		DBPath:   env("DB_PATH", "./runner.db"),
	}
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
