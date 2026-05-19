package executor

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"sync"
	"time"

	"e2e-runner/domain"
	"e2e-runner/store"
)

func ExecuteTest(run *domain.Run, testsDir string, rs store.RunStore, timeout time.Duration) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	var cmd *exec.Cmd
	if run.File != "" {
		file := domain.SanitizeName(run.File)
		if !strings.HasSuffix(file, ".spec.ts") {
			file += ".spec.ts"
		}
		run.AddLog(fmt.Sprintf("[info] テスト開始: %s", file))
		cmd = exec.CommandContext(ctx, "npx", "playwright", "test", "tests/"+file, "--reporter=line,html")
	} else {
		run.AddLog(fmt.Sprintf("[info] テスト開始: @%s", run.Tag))
		cmd = exec.CommandContext(ctx, "npx", "playwright", "test", "--grep", "@"+run.Tag, "--reporter=line,html")
	}
	cmd.Dir = testsDir

	stdout, _ := cmd.StdoutPipe()
	stderr, _ := cmd.StderrPipe()

	if err := cmd.Start(); err != nil {
		run.AddLog(fmt.Sprintf("[error] 起動失敗: %v", err))
		run.Finish(false)
		rs.Save(run)
		return
	}

	var wg sync.WaitGroup
	pipe := func(r io.Reader, prefix string) {
		defer wg.Done()
		sc := bufio.NewScanner(r)
		for sc.Scan() {
			if line := strings.TrimSpace(sc.Text()); line != "" {
				run.AddLog(prefix + line)
			}
		}
	}
	wg.Add(2)
	go pipe(stdout, "")
	go pipe(stderr, "[stderr] ")
	wg.Wait()

	if err := cmd.Wait(); err != nil {
		if ctx.Err() != nil {
			run.AddLog("[error] タイムアウトにより強制終了")
			run.Finish(false)
			rs.Save(run)
			return
		}
		run.AddLog(fmt.Sprintf("[info] テスト終了: 失敗 (%v)", err))
		run.Finish(false)
	} else {
		run.AddLog("[info] テスト終了: 成功")
		run.Finish(true)
	}
	rs.Save(run)
}
