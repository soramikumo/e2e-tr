package config

import (
	"os"
	"testing"
	"time"
)

// TestLoad_MaxConcurrentRuns は MAX_CONCURRENT_RUNS の解釈とクランプを検証する。
// 0 や負値はバッファ無し/panic を招くため、最低 1 にクランプされること、
// 未設定なら既定の 4 になることを確認する。
func TestLoad_MaxConcurrentRuns(t *testing.T) {
	tests := []struct {
		name string
		set  bool // 環境変数を設定するか（false なら未設定）
		env  string
		want int
	}{
		{name: "未設定なら既定の4", set: false, want: 4},
		{name: "正値はそのまま", set: true, env: "8", want: 8},
		{name: "0は1にクランプ", set: true, env: "0", want: 1},
		{name: "負値は1にクランプ", set: true, env: "-3", want: 1},
		{name: "不正値は既定の4", set: true, env: "abc", want: 4},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.set {
				os.Setenv("MAX_CONCURRENT_RUNS", tt.env)
			} else {
				os.Unsetenv("MAX_CONCURRENT_RUNS")
			}
			t.Cleanup(func() { os.Unsetenv("MAX_CONCURRENT_RUNS") })

			got := Load().MaxConcurrentRuns
			if got != tt.want {
				t.Errorf("MaxConcurrentRuns = %d, want %d", got, tt.want)
			}
		})
	}
}

// TestLoad_RunTimeout は RUN_TIMEOUT_MINUTES の解釈とフォールバックを検証する。
// 0 以下は「即タイムアウト」になり実行が成立しないため、既定の 30 分に
// フォールバックすること、未設定でも 30 分になることを確認する。
func TestLoad_RunTimeout(t *testing.T) {
	tests := []struct {
		name string
		set  bool
		env  string
		want time.Duration
	}{
		{name: "未設定なら既定の30分", set: false, want: 30 * time.Minute},
		{name: "正値はそのまま", set: true, env: "10", want: 10 * time.Minute},
		{name: "0は既定の30分", set: true, env: "0", want: 30 * time.Minute},
		{name: "負値は既定の30分", set: true, env: "-5", want: 30 * time.Minute},
		{name: "不正値は既定の30分", set: true, env: "xyz", want: 30 * time.Minute},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.set {
				os.Setenv("RUN_TIMEOUT_MINUTES", tt.env)
			} else {
				os.Unsetenv("RUN_TIMEOUT_MINUTES")
			}
			t.Cleanup(func() { os.Unsetenv("RUN_TIMEOUT_MINUTES") })

			got := Load().RunTimeout
			if got != tt.want {
				t.Errorf("RunTimeout = %v, want %v", got, tt.want)
			}
		})
	}
}
