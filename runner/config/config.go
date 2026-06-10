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

	// KasmVNC(Xvnc)のセキュリティ構成。既定は内部/コンテナ localhost 前提の
	// 無認証・平文 ws。Azure 等へ HTTPS 公開する際は env で締める。
	VNCSecurityTypes    string // -SecurityTypes（既定 "None"=無認証）
	VNCDisableBasicAuth bool   // 既定 true。BasicAuth を使うなら false
	VNCSSLOnly          bool   // 既定 false。true で wss を強制（-sslOnly 1）
	VNCInterface        string // バインド先（既定 "0.0.0.0"）
}

func Load() *Config {
	minutes := 30
	if v := os.Getenv("RUN_TIMEOUT_MINUTES"); v != "" {
		if m, err := strconv.Atoi(v); err == nil {
			minutes = m
		}
	}
	// 0以下のタイムアウトは「即タイムアウト」を意味してしまい実行が成立しない。
	// 不正な指定は無効化し、既定の 30 分にフォールバックする。
	if minutes <= 0 {
		minutes = 30
	}
	// 既定で複数テストを並列実行できるようにする(MAX_CONCURRENT_RUNS で調整可)。
	maxConcurrent := 4
	if v := os.Getenv("MAX_CONCURRENT_RUNS"); v != "" {
		if m, err := strconv.Atoi(v); err == nil {
			maxConcurrent = m
		}
	}
	// この値は handler 側で並列実行枠の semaphore チャネルのバッファ長になる。
	// 0 だとバッファ無しチャネルになり全実行が 429 で弾かれ、負値だと make が
	// panic する。最低 1 を保証して常に1件は実行できるようにクランプする。
	if maxConcurrent < 1 {
		maxConcurrent = 1
	}
	return &Config{
		TestsDir: env("TESTS_DIR", "../tests"),
		Port:     env("PORT", ":8080"),
		DBPath:   env("DB_PATH", "./runner.db"),
		// USE_NOVNC: コンテナ/PaaS など画面なし環境では true（KasmVNC の Xvnc を使う）。
		// env 名とフィールド名を同じ極性で対応させ、反転は一切挟まない。
		UseNoVNC:          boolEnv("USE_NOVNC", true),
		RunTimeout:        time.Duration(minutes) * time.Minute,
		MaxConcurrentRuns: maxConcurrent,

		VNCSecurityTypes:    env("VNC_SECURITY_TYPES", "None"),
		VNCDisableBasicAuth: boolEnv("VNC_DISABLE_BASIC_AUTH", true),
		VNCSSLOnly:          boolEnv("VNC_SSL_ONLY", false),
		VNCInterface:        env("VNC_INTERFACE", "0.0.0.0"),
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
