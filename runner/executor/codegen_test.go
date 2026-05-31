package executor_test

import (
	"slices"
	"strings"
	"testing"

	"e2e-runner/domain"
	"e2e-runner/executor"
	"e2e-runner/store"
	"e2e-runner/vnc"
)

// fakeVNC は VNCSessions の偽物。実際の Xvnc を起動せずセッションを注入する。
type fakeVNC struct{ sess *vnc.Session }

func (f fakeVNC) Get(string) (*vnc.Session, bool) {
	if f.sess != nil {
		return f.sess, true
	}
	return nil, false
}
func (fakeVNC) Stop(string) {}

// VNC セッション配下のとき、ブラウザを画面いっぱいに開く --viewport-size と
// DISPLAY が渡り、Inspector を殺す PW_CODEGEN_NO_INSPECTOR は付かないことを確認する。
func TestExecuteCodegen_UnderVNC_FillsFramebufferAndKeepsInspector(t *testing.T) {
	fake := &fakeRunner{}
	ex := executor.New(fake, t.TempDir())
	c := domain.NewCodegen("https://example.com", "viewport-spec")
	vm := fakeVNC{sess: &vnc.Session{Display: ":99"}}

	ex.ExecuteCodegen(c, store.NewMemoryCodegenStore(), vm)

	if !slices.Contains(fake.lastOpts.Args, "--viewport-size=1600,820") {
		t.Errorf("args = %v, want to contain --viewport-size=1600,820", fake.lastOpts.Args)
	}
	if !slices.Contains(fake.lastOpts.Env, "DISPLAY=:99") {
		t.Errorf("env = %v, want to contain DISPLAY=:99", fake.lastOpts.Env)
	}
	for _, e := range fake.lastOpts.Env {
		if strings.HasPrefix(e, "PW_CODEGEN_NO_INSPECTOR") {
			t.Errorf("env must not disable inspector (breaks recording/save), got %q", e)
		}
	}
}

// VNC を使わないとき、--viewport-size も DISPLAY も付かないことを確認する。
func TestExecuteCodegen_WithoutVNC_NoViewportNoDisplay(t *testing.T) {
	fake := &fakeRunner{}
	ex := executor.New(fake, t.TempDir())
	c := domain.NewCodegen("https://example.com", "novnc-spec")

	var vm executor.VNCSessions // nil インターフェース
	ex.ExecuteCodegen(c, store.NewMemoryCodegenStore(), vm)

	for _, a := range fake.lastOpts.Args {
		if strings.HasPrefix(a, "--viewport-size") {
			t.Errorf("args should not contain viewport-size without VNC: %v", fake.lastOpts.Args)
		}
	}
	if fake.lastOpts.Env != nil {
		t.Errorf("env should be nil without VNC, got %v", fake.lastOpts.Env)
	}
}
