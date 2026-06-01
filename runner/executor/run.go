package executor

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"time"

	"e2e-runner/domain"
	"e2e-runner/store"
)

func (e *Executor) ExecuteTest(run *domain.Run, rs store.RunStore, timeout time.Duration) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
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
		rs.Save(run)
		return
	}
	// trace を許可した実行では成功時も trace を保存する(config 既定の on-first-retry を上書き)。
	if run.Trace {
		args = append(args, "--trace", "on")
	}
	stdout := newLineWriter(func(line string) { run.AddLog(line) })
	stderr := newLineWriter(func(line string) { run.AddLog("[stderr] " + line) })
	err := e.Runner.Run(ctx, RunOptions{
		Dir:    e.TestsDir,
		Name:   "npx",
		Args:   args,
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
	rs.Save(run)
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
