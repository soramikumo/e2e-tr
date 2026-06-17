package executor

import (
	"context"
	"io"
	"os/exec"
)

// RunOptions configures a single command execution.
type RunOptions struct {
	Dir    string
	Name   string
	Args   []string
	Env    []string  // nil means inherit parent env
	Stdout io.Writer // nil means discard
	Stderr io.Writer // nil means discard
}

// Runner executes external commands. Swap implementations for testing.
type Runner interface {
	Run(ctx context.Context, opts RunOptions) error
}

// OSRunner is the real implementation that spawns OS processes.
type OSRunner struct{}

func (OSRunner) Run(ctx context.Context, opts RunOptions) error {
	cmd := exec.CommandContext(ctx, opts.Name, opts.Args...)
	cmd.Dir = opts.Dir
	if opts.Env != nil {
		cmd.Env = opts.Env
	}
	if opts.Stdout != nil {
		cmd.Stdout = opts.Stdout
	}
	if opts.Stderr != nil {
		cmd.Stderr = opts.Stderr
	}
	return cmd.Run()
}

// Executor runs Playwright test and codegen commands via an injectable Runner.
type Executor struct {
	Runner   Runner
	TestsDir string
}

func New(r Runner, testsDir string) *Executor {
	return &Executor{Runner: r, TestsDir: testsDir}
}
