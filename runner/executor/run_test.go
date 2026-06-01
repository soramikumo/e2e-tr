package executor_test

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"

	"e2e-runner/domain"
	"e2e-runner/executor"
	"e2e-runner/store"
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
	ex.ExecuteTest(run, store.NewMemoryRunStore(), 5*time.Second)

	want := []string{"playwright", "test", "--grep", "@smoke", "--reporter=line,html"}
	if !slices.Equal(fake.lastOpts.Args, want) {
		t.Errorf("Args = %v, want %v", fake.lastOpts.Args, want)
	}
}

// trace を許可した実行では --trace on が引数末尾に加わる。
func TestExecuteTest_TraceEnabled_AddsTraceOn(t *testing.T) {
	fake := &fakeRunner{}
	ex := executor.New(fake, "/tmp/tests")

	run := domain.NewRun("smoke", "")
	run.Trace = true
	ex.ExecuteTest(run, store.NewMemoryRunStore(), 5*time.Second)

	want := []string{"playwright", "test", "--grep", "@smoke", "--reporter=line,html", "--trace", "on"}
	if !slices.Equal(fake.lastOpts.Args, want) {
		t.Errorf("Args = %v, want %v", fake.lastOpts.Args, want)
	}
}

// trace を許可しない実行(既定)では --trace は加わらない。
func TestExecuteTest_TraceDisabled_OmitsTraceFlag(t *testing.T) {
	fake := &fakeRunner{}
	ex := executor.New(fake, "/tmp/tests")

	run := domain.NewRun("smoke", "")
	ex.ExecuteTest(run, store.NewMemoryRunStore(), 5*time.Second)

	if slices.Contains(fake.lastOpts.Args, "--trace") {
		t.Errorf("Args = %v, must not contain --trace when Trace is false", fake.lastOpts.Args)
	}
}

func TestExecuteTest_FileRun_AddsSpecSuffix(t *testing.T) {
	fake := &fakeRunner{}
	ex := executor.New(fake, "/tmp/tests")

	// ファイル名に .spec.ts がない場合、自動で付与されるはず
	run := domain.NewRun("", "login")
	ex.ExecuteTest(run, store.NewMemoryRunStore(), 5*time.Second)

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
	ex.ExecuteTest(run, store.NewMemoryRunStore(), 5*time.Second)

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
	rs := store.NewMemoryRunStore()
	ex.ExecuteTest(run, rs, 10*time.Millisecond) // 非常に短いタイムアウト

	if run.GetStatus() != domain.StatusFailed {
		t.Errorf("Status = %q, want %q", run.GetStatus(), domain.StatusFailed)
	}
}

// Runner がエラーを返したとき、Run のステータスが failed になることを確認する。
func TestExecuteTest_OnRunnerError_MarksRunFailed(t *testing.T) {
	ex := executor.New(errorRunner{}, "/tmp/tests")

	run := domain.NewRun("smoke", "")
	rs := store.NewMemoryRunStore()
	ex.ExecuteTest(run, rs, 5*time.Second)

	if run.GetStatus() != domain.StatusFailed {
		t.Errorf("Status = %q, want %q", run.GetStatus(), domain.StatusFailed)
	}
}

// Runner が成功したとき、Run のステータスが done になることを確認する。
func TestExecuteTest_OnSuccess_MarksRunDone(t *testing.T) {
	ex := executor.New(&fakeRunner{}, "/tmp/tests") // fakeRunner は nil を返す

	run := domain.NewRun("smoke", "")
	rs := store.NewMemoryRunStore()
	ex.ExecuteTest(run, rs, 5*time.Second)

	if run.GetStatus() != domain.StatusDone {
		t.Errorf("Status = %q, want %q", run.GetStatus(), domain.StatusDone)
	}
}
