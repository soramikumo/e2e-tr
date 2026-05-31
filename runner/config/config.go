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
		// USE_NOVNC: コンテナ/PaaS など画面なし環境では true（Xvfb+noVNC を使う）。
		// env 名とフィールド名を同じ極性で対応させ、反転は一切挟まない。
		UseNoVNC:          boolEnv("USE_NOVNC", true),
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

// boolEnv は env を bool として解釈する。未設定・不正値なら fallback を返す。
// "1"/"t"/"true"/"0"/"f"/"false" などを受け付ける（strconv.ParseBool 準拠）。
func boolEnv(key string, fallback bool) bool {
	v, err := strconv.ParseBool(os.Getenv(key))
	if err != nil {
		return fallback
	}
	return v
}
