package tools

import (
	"context"
	"fmt"
	"os/exec"
	"runtime"
	"time"
)

// CommandRunner 抽象 shell 执行；测试注入 FakeRunner，避免真实进程依赖。
type CommandRunner interface {
	Run(ctx context.Context, dir, command string, timeoutSec int) (stdout, stderr string, exitCode int, err error)
}

// RealRunner 在真实 shell 中执行命令。
type RealRunner struct{}

func (RealRunner) Run(ctx context.Context, dir, command string, timeoutSec int) (string, string, int, error) {
	tctx, cancel := context.WithTimeout(ctx, time.Duration(timeoutSec)*time.Second)
	defer cancel()
	var c *exec.Cmd
	if runtime.GOOS == "windows" {
		c = exec.CommandContext(tctx, "cmd", "/c", command)
	} else {
		c = exec.CommandContext(tctx, "sh", "-c", command)
	}
	c.Dir = dir
	var stdout, stderr buf
	c.Stdout = &stdout
	c.Stderr = &stderr
	err := c.Run()
	code := 0
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			code = ee.ExitCode()
		} else {
			return stdout.s, stderr.s, -1, err
		}
	}
	return stdout.s, stderr.s, code, nil
}

type buf struct{ s string }

func (b *buf) Write(p []byte) (int, error) { b.s += string(p); return len(p), nil }

func ShellTool(r CommandRunner, root string) Tool {
	return &fileTool{name: "run_shell", desc: "在仓库根执行 shell 命令（60s 超时）", root: root,
		params: fileParams([]string{"command"}, []string{"command"}),
		fn: func(ctx context.Context, _ string, a map[string]any) (string, error) {
			out, e, code, err := r.Run(ctx, root, argStr(a, "command"), 60)
			if err != nil {
				return "", err
			}
			return fmt.Sprintf("exit=%d\nstdout:\n%s\nstderr:\n%s", code, out, e), nil
		}}
}

// TestTool 执行会话测试命令并返回原始输出（结构化解析在 feedback 层）。
func TestTool(r CommandRunner, root, testCmd string) Tool {
	return &fileTool{name: "run_test", desc: "运行项目测试命令", root: root,
		params: fileParams([]string{"command"}, nil),
		fn: func(ctx context.Context, _ string, a map[string]any) (string, error) {
			cmd := testCmd
			if override := argStr(a, "command"); override != "" {
				cmd = override
			}
			out, e, code, err := r.Run(ctx, root, cmd, 300)
			if err != nil {
				return "", err
			}
			return fmt.Sprintf("exit=%d\n%s\n%s", code, out, e), nil
		}}
}
