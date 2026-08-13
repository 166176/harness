package cli

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/166176/harness/internal/govern"
	"golang.org/x/term"
)

// approvalDecider 构造 run 的审批决定器（SPEC §6.3 CLI 通道）：
// stdin 为 TTY 时打印规则/风险/动作并读 y/n（非法输入按拒绝，fail-closed）；
// 非 TTY 回退 HITL.Await（等待 WebUI 决定或超时）。
func (a *app) approvalDecider(hitl *govern.Manager, timeout time.Duration) func(context.Context, *govern.Approval) govern.ApprovalStatus {
	if !stdinIsTTY(a.stdin) {
		return func(ctx context.Context, ap *govern.Approval) govern.ApprovalStatus {
			return hitl.Await(ctx, ap.ID, timeout)
		}
	}
	return func(_ context.Context, ap *govern.Approval) govern.ApprovalStatus {
		fmt.Fprintf(a.stdout, "\n⚠ 需要人工审批\n  规则: %s\n  风险: %s\n  动作: %s %s\n允许执行？[y/N] ",
			ap.Rule, ap.Risk, ap.Action.Tool, formatArgs(ap.Action.Args))
		line, err := readLine(a.stdin)
		if err != nil {
			fmt.Fprintf(a.stderr, "读取审批输入失败：%v（按拒绝处理）\n", err)
			_ = hitl.Decide(ap.ID, govern.Denied, "cli")
			return govern.Denied
		}
		switch strings.ToLower(strings.TrimSpace(line)) {
		case "y", "yes":
			if err := hitl.Decide(ap.ID, govern.Approved, "cli"); err != nil {
				fmt.Fprintf(a.stderr, "审批记录失败：%v（按拒绝处理）\n", err)
				return govern.Denied
			}
			return govern.Approved
		default:
			_ = hitl.Decide(ap.ID, govern.Denied, "cli")
			return govern.Denied
		}
	}
}

// stdinIsTTY 判断输入是否为交互式终端（TTY 才走 y/n 通道）。
func stdinIsTTY(r io.Reader) bool {
	f, ok := r.(*os.File)
	if !ok || f == nil {
		return false
	}
	return term.IsTerminal(int(f.Fd()))
}

// readLine 读取一行输入；EOF 且无内容时返回错误。
func readLine(r io.Reader) (string, error) {
	line, err := bufio.NewReader(r).ReadString('\n')
	if err != nil && len(line) == 0 {
		return "", err
	}
	return line, nil
}

// formatArgs 以 JSON 稳定输出动作参数（审批提示用）。
func formatArgs(args map[string]any) string {
	b, err := json.Marshal(args)
	if err != nil {
		return fmt.Sprint(args)
	}
	return string(b)
}
