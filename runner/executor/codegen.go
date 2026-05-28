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

func (e *Executor) ExecuteCodegen(c *domain.Codegen, cs store.CodegenStore, vncManager *vnc.Manager) {
	name := domain.SanitizeName(c.Name)
	outputFile := filepath.Join(e.TestsDir, "tests", name+".spec.ts")

	c.Send(domain.CodegenEvent{Type: "status", Message: "記録中... ブラウザを閉じると保存されます"})

	var stderr bytes.Buffer
	opts := RunOptions{
		Dir:    e.TestsDir,
		Name:   "npx",
		Args:   []string{"playwright", "codegen", "--output", outputFile, c.URL},
		Stderr: &stderr,
	}

	if vncManager != nil {
		if session, ok := vncManager.Get(c.ID); ok {
			defer vncManager.Stop(c.ID)
			opts.Env = append(os.Environ(), "DISPLAY="+session.Display)
		}
	}

	err := e.Runner.Run(context.Background(), opts)

	if _, statErr := os.Stat(outputFile); statErr != nil {
		if err != nil {
			c.Finish("", fmt.Errorf("記録失敗: %v\n%s", err, stderr.String()))
		} else {
			c.Finish("", fmt.Errorf("ファイルが保存されませんでした\n%s", stderr.String()))
		}
		return
	}

	c.Finish(filepath.Base(outputFile), nil)
	cs.Save(c)
}
