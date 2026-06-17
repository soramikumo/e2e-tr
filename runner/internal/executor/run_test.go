package executor_test

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	"e2e-runner/internal/domain"
	"e2e-runner/internal/executor"
	"e2e-runner/internal/store"
)

// blockingRunner は ctx がキャンセルされるまでブロックする。タイムアウトテスト用。
type blockingRunner struct{}

func (blockingRunner) Run(ctx context.Context, _ executor.RunOptions) error {
	<-ctx.Done()
	return ctx.Err()
}

// errorRunner は常にエラーを返す。失敗ケースのテスト用。
type errorRunner struct{}

func (errorRunner) Run(_ context.Context, _ executor.RunOptions) error {
	return errors.New("command failed: exit status 1")
}

// fakeRunner は Runner インターフェースの偽物。npx を一切起動しない。
type fakeRunner struct {
	lastOpts executor.RunOptions // 最後に渡された引数を記録する
	err      error               // Run() が返すエラーを外から設定できる
}

func (f *fakeRunner) Run(_ context.Context, opts executor.RunOptions) error {
	f.lastOpts = opts
	return f.err
}

// テストは、実際に npx を起動せずに、RunOptions が正しく構築されているかを検証する。
func TestExecuteTest_TagRun_PassesCorrectArgs(t *testing.T) {
	fake := &fakeRunner{}

	ex := executor.New(fake, "/tmp/tests")

	run := domain.NewRun("smoke", "")
	run.Files = []string{"login.spec.ts", "signup.spec.ts"}
	ex.ExecuteTest(context.Background(), run, store.NewMemoryRunStore(), 5*time.Second)

	want := []string{"playwright", "test", "tests/login.spec.ts", "tests/signup.spec.ts", "--reporter=line,html", "--output", "test-results/" + run.ID}
	if !slices.Equal(fake.lastOpts.Args, want) {
		t.Errorf("Args = %v, want %v", fake.lastOpts.Args, want)
	}
}

// レポート/アーティファクトの出力先が run.ID 単位に分離されることを確認する(#88)。
// HTML レポートは env PLAYWRIGHT_HTML_OUTPUT_DIR=playwright-report/<id>、
// アーティファクトは CLI --output test-results/<id> で渡る。
func TestExecuteTest_IsolatesReportAndArtifactsByRunID(t *testing.T) {
	fake := &fakeRunner{}
	ex := executor.New(fake, "/tmp/tests")

	run := domain.NewRun("smoke", "")
	run.Files = []string{"login.spec.ts"}
	ex.ExecuteTest(context.Background(), run, store.NewMemoryRunStore(), 5*time.Second)

	// --output が test-results/<run.ID> を指す引数として渡っていること。
	wantOutput := "test-results/" + run.ID
	foundOutput := false
	for i, arg := range fake.lastOpts.Args {
		if arg == "--output" && i+1 < len(fake.lastOpts.Args) && fake.lastOpts.Args[i+1] == wantOutput {
			foundOutput = true
		}
	}
	if !foundOutput {
		t.Errorf("Args = %v, want --output %q", fake.lastOpts.Args, wantOutput)
	}

	// env に PLAYWRIGHT_HTML_OUTPUT_DIR=playwright-report/<run.ID> が含まれること。
	wantEnv := "PLAYWRIGHT_HTML_OUTPUT_DIR=playwright-report/" + run.ID
	if !slices.Contains(fake.lastOpts.Env, wantEnv) {
		t.Errorf("Env = %v, want to contain %q", fake.lastOpts.Env, wantEnv)
	}
}

// 異なる run.ID では出力先パスも異なる(並列実行で衝突しない)ことを確認する。
func TestExecuteTest_DifferentRunsGetDifferentOutputDirs(t *testing.T) {
	ex := executor.New(&fakeRunner{}, "/tmp/tests")

	collect := func() (string, string) {
		fake := &fakeRunner{}
		ex = executor.New(fake, "/tmp/tests")
		run := domain.NewRun("", "login")
		ex.ExecuteTest(context.Background(), run, store.NewMemoryRunStore(), 5*time.Second)
		var html, output string
		for _, e := range fake.lastOpts.Env {
			if v, ok := strings.CutPrefix(e, "PLAYWRIGHT_HTML_OUTPUT_DIR="); ok {
				html = v
			}
		}
		for i, a := range fake.lastOpts.Args {
			if a == "--output" && i+1 < len(fake.lastOpts.Args) {
				output = fake.lastOpts.Args[i+1]
			}
		}
		return html, output
	}

	html1, out1 := collect()
	html2, out2 := collect()
	if html1 == html2 {
		t.Errorf("two runs got the same HTML output dir %q (must differ by run.ID)", html1)
	}
	if out1 == out2 {
		t.Errorf("two runs got the same artifact dir %q (must differ by run.ID)", out1)
	}
}

// trace を許可した実行では --trace on が引数末尾に加わる。
func TestExecuteTest_TraceEnabled_AddsTraceOn(t *testing.T) {
	fake := &fakeRunner{}
	ex := executor.New(fake, "/tmp/tests")

	run := domain.NewRun("smoke", "")
	run.Files = []string{"login.spec.ts"}
	run.Trace = true
	ex.ExecuteTest(context.Background(), run, store.NewMemoryRunStore(), 5*time.Second)

	want := []string{"playwright", "test", "tests/login.spec.ts", "--reporter=line,html", "--trace", "on", "--output", "test-results/" + run.ID}
	if !slices.Equal(fake.lastOpts.Args, want) {
		t.Errorf("Args = %v, want %v", fake.lastOpts.Args, want)
	}
}

// trace を許可しない実行(既定)では --trace は加わらない。
func TestExecuteTest_TraceDisabled_OmitsTraceFlag(t *testing.T) {
	fake := &fakeRunner{}
	ex := executor.New(fake, "/tmp/tests")

	run := domain.NewRun("smoke", "")
	run.Files = []string{"login.spec.ts"}
	ex.ExecuteTest(context.Background(), run, store.NewMemoryRunStore(), 5*time.Second)

	if slices.Contains(fake.lastOpts.Args, "--trace") {
		t.Errorf("Args = %v, must not contain --trace when Trace is false", fake.lastOpts.Args)
	}
}

func TestExecuteTest_FileRun_AddsSpecSuffix(t *testing.T) {
	fake := &fakeRunner{}
	ex := executor.New(fake, "/tmp/tests")

	// ファイル名に .spec.ts がない場合、自動で付与されるはず
	run := domain.NewRun("", "login")
	ex.ExecuteTest(context.Background(), run, store.NewMemoryRunStore(), 5*time.Second)

	found := false
	for _, arg := range fake.lastOpts.Args {
		if arg == "tests/login.spec.ts" {
			found = true
		}
	}
	if !found {
		t.Errorf("Args = %v, want tests/login.spec.ts to be present", fake.lastOpts.Args)
	}
}

// .spec.ts がすでについているファイル名を渡したとき、二重にならないことを確認する。
func TestExecuteTest_FileAlreadyHasSpecTsSuffix_NotDuplicated(t *testing.T) {
	fake := &fakeRunner{}
	ex := executor.New(fake, "/tmp/tests")

	run := domain.NewRun("", "login.spec.ts")
	ex.ExecuteTest(context.Background(), run, store.NewMemoryRunStore(), 5*time.Second)

	if slices.Contains(fake.lastOpts.Args, "tests/login.spec.ts.spec.ts") {
		t.Errorf("Args = %v, .spec.ts was duplicated", fake.lastOpts.Args)
	}
	if !slices.Contains(fake.lastOpts.Args, "tests/login.spec.ts") {
		t.Errorf("Args = %v, want tests/login.spec.ts to be present", fake.lastOpts.Args)
	}
}

// Runner がタイムアウトで終了したとき、Run のステータスが failed になることを確認する。
func TestExecuteTest_OnTimeout_MarksRunFailed(t *testing.T) {
	ex := executor.New(blockingRunner{}, "/tmp/tests")

	run := domain.NewRun("smoke", "")
	run.Files = []string{"login.spec.ts"}
	rs := store.NewMemoryRunStore()
	ex.ExecuteTest(context.Background(), run, rs, 10*time.Millisecond) // 非常に短いタイムアウト

	if run.GetStatus() != domain.StatusFailed {
		t.Errorf("Status = %q, want %q", run.GetStatus(), domain.StatusFailed)
	}
}

// Runner がエラーを返したとき、Run のステータスが failed になることを確認する。
func TestExecuteTest_OnRunnerError_MarksRunFailed(t *testing.T) {
	ex := executor.New(errorRunner{}, "/tmp/tests")

	run := domain.NewRun("smoke", "")
	run.Files = []string{"login.spec.ts"}
	rs := store.NewMemoryRunStore()
	ex.ExecuteTest(context.Background(), run, rs, 5*time.Second)

	if run.GetStatus() != domain.StatusFailed {
		t.Errorf("Status = %q, want %q", run.GetStatus(), domain.StatusFailed)
	}
}

// Runner が成功したとき、Run のステータスが done になることを確認する。
func TestExecuteTest_OnSuccess_MarksRunDone(t *testing.T) {
	ex := executor.New(&fakeRunner{}, "/tmp/tests") // fakeRunner は nil を返す

	run := domain.NewRun("smoke", "")
	run.Files = []string{"login.spec.ts"}
	rs := store.NewMemoryRunStore()
	ex.ExecuteTest(context.Background(), run, rs, 5*time.Second)

	if run.GetStatus() != domain.StatusDone {
		t.Errorf("Status = %q, want %q", run.GetStatus(), domain.StatusDone)
	}
}
