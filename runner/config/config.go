package config

import "os"

type Config struct {
	TestsDir string
	Port     string
	DBPath   string
	UseNoVNC bool
}

func Load() *Config {
	return &Config{
		TestsDir: env("TESTS_DIR", "../tests"),
		Port:     env("PORT", ":8080"),
		DBPath:   env("DB_PATH", "./runner.db"),
		// USE_NOVNC 環境変数が "true" に設定されている場合にはLocalで起動しないといけない
		UseNoVNC: os.Getenv("USE_NOVNC") == "true",
	}
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
