package executor

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"e2e-runner/domain"
	"e2e-runner/store"
)

func ExecuteCodegen(c *domain.Codegen, testsDir string, cs store.CodegenStore) {
	name := domain.SanitizeName(c.Name)
	outputFile := filepath.Join(testsDir, "tests", name+".spec.ts")

	c.Send(domain.CodegenEvent{Type: "status", Message: "ブラウザを起動しています..."})

	cmd := exec.Command("npx", "playwright", "codegen", "--output", outputFile, c.URL)
	cmd.Dir = testsDir

	if err := cmd.Start(); err != nil {
		c.Finish("", fmt.Errorf("起動失敗: %v", err))
		return
	}

	c.Send(domain.CodegenEvent{Type: "status", Message: "記録中... ブラウザを閉じると保存されます"})

	err := cmd.Wait()

	if _, statErr := os.Stat(outputFile); statErr != nil {
		if err != nil {
			c.Finish("", fmt.Errorf("記録失敗: %v", err))
		} else {
			c.Finish("", fmt.Errorf("ファイルが保存されませんでした"))
		}
		return
	}

	c.Finish(outputFile, nil)
	cs.Save(c)
}
