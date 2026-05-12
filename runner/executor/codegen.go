package executor

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"e2e-runner/domain"
	"e2e-runner/store"
	"e2e-runner/vnc"
)

func ExecuteCodegen(c *domain.Codegen, testsDir string, cs store.CodegenStore, vncManager *vnc.Manager) {
	name := domain.SanitizeName(c.Name)
	outputFile := filepath.Join(testsDir, "tests", name+".spec.ts")

	c.Send(domain.CodegenEvent{Type: "status", Message: "記録中... ブラウザを閉じると保存されます"})

	var stderr bytes.Buffer
	cmd := exec.Command("npx", "playwright", "codegen", "--output", outputFile, c.URL)
	cmd.Dir = testsDir
	cmd.Stderr = &stderr

	if vncManager != nil {
		if session, ok := vncManager.Get(c.ID); ok {
			defer vncManager.Stop(c.ID)
			cmd.Env = append(os.Environ(), "DISPLAY="+session.Display)
		}
	}

	if err := cmd.Start(); err != nil {
		c.Finish("", fmt.Errorf("起動失敗: %v", err))
		return
	}

	err := cmd.Wait()

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
