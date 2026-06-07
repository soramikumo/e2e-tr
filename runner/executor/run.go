package executor

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"e2e-runner/domain"
	"e2e-runner/store"
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
	// baseURL を指定した実行では PLAYWRIGHT_BASE_URL を注入し、playwright.config が
	// これを読む(相対 goto の spec が dev/prod/blue-green で切り替わる)。空なら env は
	// 親から継承させ、config 既定の baseURL がそのまま使われる。
	// 認証情報も Environment 経由なら同時に注入(httpCredentials として config が拾う)。
	var env []string
	if run.BaseURL != "" || run.AuthUser != "" {
		env = append([]string{}, os.Environ()...)
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
