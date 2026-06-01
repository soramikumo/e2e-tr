package handler

import (
	"testing"

	"e2e-runner/vnc"
)

// nil *vnc.Manager を渡したとき、型なし nil インターフェースとして返ること（typed-nil
// を境界で止めること）を保証する。これが破れると ExecuteCodegen の nil ガードをすり抜け、
// UseNoVNC=false かつ VNCManager 未設定の経路で nil レシーバ panic が再発する。
func TestVNCSessions_NilManager_ReturnsUntypedNil(t *testing.T) {
	if vncSessions(nil) != nil {
		t.Fatal("typed-nil が VNCSessions インターフェースに漏れている")
	}

	var m *vnc.Manager // 明示的に typed-nil を作って渡しても漏れないこと
	if vncSessions(m) != nil {
		t.Fatal("typed-nil の *vnc.Manager がインターフェースに漏れている")
	}
}

// 非 nil の *vnc.Manager はそのまま返ること。
func TestVNCSessions_NonNilManager_PassesThrough(t *testing.T) {
	m := vnc.NewManager(vnc.Options{})
	if vncSessions(m) == nil {
		t.Fatal("非 nil の *vnc.Manager が nil インターフェースになった")
	}
}
