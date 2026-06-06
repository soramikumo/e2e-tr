package executor

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"

	"e2e-runner/domain"
	"e2e-runner/store"
	"e2e-runner/vnc"
)

// VNCSessions は ExecuteCodegen が必要とする VNC マネージャの最小機能。
// *vnc.Manager がこれを満たす。テストで偽セッションを注入できるよう抽象化している。
type VNCSessions interface {
	Get(sessionID string) (*vnc.Session, bool)
	Stop(sessionID string)
}

func (e *Executor) ExecuteCodegen(c *domain.Codegen, cs store.CodegenStore, vncManager VNCSessions) {
	name := domain.SanitizeName(c.Name)
	outputFile := filepath.Join(e.TestsDir, "tests", name+".spec.ts")

	c.Send(domain.CodegenEvent{Type: "status", Message: "記録中... ブラウザを閉じると保存されます"})

	var stderr bytes.Buffer
	args := []string{"playwright", "codegen", "--output", outputFile}
	opts := RunOptions{
		Dir:    e.TestsDir,
		Name:   "npx",
		Stderr: &stderr,
	}

	if vncManager != nil {
		if session, ok := vncManager.Get(c.ID); ok {
			defer vncManager.Stop(c.ID)
			// Playwright は Inspector(レコーダー)をブラウザの右隣に自動配置する。
			// Inspector は記録の心臓部なので無効化できない(無効化すると保存もされない)。
			// そこでブラウザを仮想画面幅(1600)いっぱいで開き、Inspector を画面右端の
			// 外(オフスクリーン)へ押し出す。記録は生きたまま VNC からは見えなくなる。
			opts.Env = append(os.Environ(), "DISPLAY="+session.Display)
			args = append(args, "--viewport-size=1600,820")
		}
	}

	args = append(args, c.URL)
	opts.Args = args

	err := e.Runner.Run(context.Background(), opts)

	if _, statErr := os.Stat(outputFile); statErr != nil {
		if err != nil {
			c.Finish("", fmt.Errorf("記録失敗: %v\n%s", err, stderr.String()))
		} else {
			c.Finish("", fmt.Errorf("ファイルが保存されませんでした\n%s", stderr.String()))
		}
		return
	}

	// 録画した spec の最初の goto を相対化し、baseURL 上書きで環境を切り替えられる
	// 状態で保存する。失敗しても記録自体は成功扱い(相対化は付加価値)なので握りつぶす。
	if b, rerr := os.ReadFile(outputFile); rerr == nil {
		if rel := domain.RelativizeFirstGoto(string(b)); rel != string(b) {
			_ = os.WriteFile(outputFile, []byte(rel), 0o644)
		}
	}

	c.Finish(filepath.Base(outputFile), nil)
	cs.Save(c)
}
