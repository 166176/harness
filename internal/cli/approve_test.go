package cli

import (
	"bytes"
	"context"
	"io"
	"testing"
	"time"

	"github.com/166176/harness/internal/govern"
)

// C1：非 TTY 路径 ApprovalDecider 回退 HITL.Await——不消费 stdin，由 WebUI 决定解除。
// （TTY y/n 交互路径留人工验收，不进单测。）
func TestApprovalDeciderNonTTYFallsBackToAwait(t *testing.T) {
	hitl := govern.NewManager()
	ap, err := hitl.Create("s1", govern.Action{Tool: "run_shell", Args: map[string]any{"command": "rm -rf /"}}, "danger", "risk")
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	buf.WriteString("y\n")
	a := &app{stdin: &buf, stdout: io.Discard, stderr: io.Discard}
	decider := a.approvalDecider(hitl, 5*time.Second)
	done := make(chan govern.ApprovalStatus, 1)
	go func() { done <- decider(context.Background(), ap) }()
	// 模拟 WebUI 经共享 HITL 决定
	if err := hitl.Decide(ap.ID, govern.Approved, "webui"); err != nil {
		t.Fatal(err)
	}
	select {
	case st := <-done:
		if st != govern.Approved {
			t.Fatalf("应为 approved，got %s", st)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("WebUI 决定后 decider 未返回")
	}
	if got := buf.String(); got != "y\n" {
		t.Fatalf("非 TTY 路径不应消费 stdin：%q", got)
	}
}
