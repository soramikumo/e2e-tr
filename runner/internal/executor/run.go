package executor

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"e2e-runner/internal/domain"
	"e2e-runner/internal/store"
)

// ExecuteTest はテストを実行し結果を rs に保存する。reqCtx はリクエスト由来の
// context(マルチテナント実装では owner_id を載せている)。コマンドのタイムアウトは
// reqCtx から派生させ、値(owner_id)を保ちつつ実行時間だけ制限する。
// 保存(rs.Save)には reqCtx を使う ── タイムアウトした ctx で保存すると、失敗の
// 記録自体がキャンセルされてしまうため。
func (e *Executor) ExecuteTest(reqCtx context.Context, run *domain.Run, rs store.RunStore, timeout time.Duration) {
	ctx, cancel := context.WithTimeout(reqCtx, timeout)
	defer cancel()

	var args []string
	switch {
	case len(run.Files) > 0:
		// タグ実行: 割当済みの複数シナリオをまとめて走らせる。
		run.AddLog(fmt.Sprintf("[info] テスト開始: @%s (%d 件)", run.Tag, len(run.Files)))
		args = []string{"playwright", "test"}
		for _, f := range run.Files {
			args = append(args, "tests/"+specFileName(f))
		}
		args = append(args, "--reporter=line,html")
	case run.File != "":
		file := specFileName(run.File)
		run.AddLog(fmt.Sprintf("[info] テスト開始: %s", file))
		args = []string{"playwright", "test", "tests/" + file, "--reporter=line,html"}
	default:
		run.AddLog("[error] 実行対象がありません")
		run.Finish(false)
		rs.Save(reqCtx, run)
		return
	}
	// trace を許可した実行では成功時も trace を保存する(config 既定の on-first-retry を上書き)。
	if run.Trace {
		args = append(args, "--trace", "on")
	}
	// アーティファクト(test-results)の出力先を run.ID 単位に分離する(#88)。
	// 既定では全 run が共有 tests/test-results/ を奪い合い、並列実行で破損する。
	// --output は artifacts(trace/screenshot/video)用の outputDir を上書きする。
	// パスは TestsDir を cwd とするコマンドからの相対(reportRelDir と対になる)。
	args = append(args, "--output", artifactRelDir(run.ID))
	// env は常に親から継承して組み立てる(以前は baseURL/auth 指定時のみ作っていた)。
	// HTML レポートの出力先を run.ID 単位に分離するため、毎回 env を構築して
	// PLAYWRIGHT_HTML_OUTPUT_DIR を注入する(#88: 共有出力先の上書き/破損を解消)。
	// PLAYWRIGHT_HTML_OUTPUT_DIR は Playwright(>=1.45、本リポは v1.60)の html
	// reporter が読む出力先 env で、config の reporter outputFolder より優先される。
	// パスはコマンド cwd(TestsDir)からの相対。
	env := append([]string{}, os.Environ()...)
	env = append(env, "PLAYWRIGHT_HTML_OUTPUT_DIR="+reportRelDir(run.ID))
	// baseURL を指定した実行では PLAYWRIGHT_BASE_URL を注入し、playwright.config が
	// これを読む(相対 goto の spec が dev/prod/blue-green で切り替わる)。空なら
	// config 既定の baseURL がそのまま使われる。
	// 認証情報も Environment 経由なら同時に注入(httpCredentials として config が拾う)。
	if run.BaseURL != "" {
		run.AddLog("[info] baseURL: " + run.BaseURL)
		env = append(env, "PLAYWRIGHT_BASE_URL="+run.BaseURL)
	}
	if run.AuthUser != "" {
		// パスワードはログに出さない(ユーザー名だけ)。
		run.AddLog("[info] basic auth: " + run.AuthUser + " ***")
		env = append(env, "PLAYWRIGHT_HTTP_AUTH_USER="+run.AuthUser)
		env = append(env, "PLAYWRIGHT_HTTP_AUTH_PASS="+run.AuthPass)
	}
	stdout := newLineWriter(func(line string) { run.AddLog(line) })
	stderr := newLineWriter(func(line string) { run.AddLog("[stderr] " + line) })
	err := e.Runner.Run(ctx, RunOptions{
		Dir:    e.TestsDir,
		Name:   "npx",
		Args:   args,
		Env:    env,
		Stdout: stdout,
		Stderr: stderr,
	})
	stdout.Flush()
	stderr.Flush()

	if err != nil {
		if ctx.Err() != nil {
			run.AddLog("[error] タイムアウトにより強制終了")
		} else {
			run.AddLog(fmt.Sprintf("[info] テスト終了: 失敗 (%v)", err))
		}
		run.Finish(false)
	} else {
		run.AddLog("[info] テスト終了: 成功")
		run.Finish(true)
	}
	rs.Save(reqCtx, run)
}

// reportRelDir は run.ID 単位の HTML レポート出力先(コマンド cwd=TestsDir からの相対)。
// run.ID はサーバ生成の16進文字列だが、念のためサニタイズしてパストラバーサルを防ぐ。
func reportRelDir(runID string) string {
	return "playwright-report/" + sanitizeRunID(runID)
}

// artifactRelDir は run.ID 単位のアーティファクト(test-results)出力先。
func artifactRelDir(runID string) string {
	return "test-results/" + sanitizeRunID(runID)
}

// sanitizeRunID は run.ID を英数字のみに制限する。16進ID(hex.EncodeToString)は
// この集合に収まるため通常は素通りするが、不正値が来てもパス要素を1つに保つ。
func sanitizeRunID(runID string) string {
	var b strings.Builder
	for _, r := range runID {
		if (r >= '0' && r <= '9') || (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') {
			b.WriteRune(r)
		}
	}
	if b.Len() == 0 {
		return "unknown"
	}
	return b.String()
}

// specFileName はファイル名をサニタイズし、.spec.ts 拡張子を補完する。
func specFileName(name string) string {
	file := domain.SanitizeName(name)
	if !strings.HasSuffix(file, ".spec.ts") {
		file += ".spec.ts"
	}
	return file
}

// lineWriter buffers incoming bytes and calls fn for each complete line.
// One instance per stream (stdout or stderr) — not safe for concurrent use.
type lineWriter struct {
	fn  func(string)
	buf []byte
}

// newLineWriter creates a lineWriter that calls fn for each complete line.
func newLineWriter(fn func(string)) *lineWriter {
	return &lineWriter{fn: fn}
}

// Write implements io.Writer. It buffers data until it sees a newline, then calls fn with the line.
func (w *lineWriter) Write(p []byte) (int, error) {
	w.buf = append(w.buf, p...)
	for {
		i := bytes.IndexByte(w.buf, '\n')
		if i < 0 {
			break
		}
		line := strings.TrimSpace(string(w.buf[:i]))
		if line != "" {
			w.fn(line)
		}
		w.buf = w.buf[i+1:]
	}
	return len(p), nil
}

// Flush emits any remaining buffered data that lacks a trailing newline.
func (w *lineWriter) Flush() {
	if line := strings.TrimSpace(string(w.buf)); line != "" {
		w.fn(line)
	}
	w.buf = nil
}
