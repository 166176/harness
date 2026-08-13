package core

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/166176/harness/internal/feedback"
	"github.com/166176/harness/internal/govern"
	"github.com/166176/harness/internal/llm"
	"github.com/166176/harness/internal/memory"
	"github.com/166176/harness/internal/tools"
)

func TestLoopFixFailingTestEndToEnd(t *testing.T) {
	mock := &llm.ScriptedMock{Steps: []llm.Completion{
		{Message: llm.Message{Role: llm.RoleAssistant, ToolCalls: []llm.ToolCall{{ID: "1", Name: "write_file", Arguments: `{"path":"calc.go","content":"fixed"}`}}}},
		{Message: llm.Message{Role: llm.RoleAssistant, ToolCalls: []llm.ToolCall{{ID: "2", Name: "run_test", Arguments: `{}`}}}},
		{Message: llm.Message{Role: llm.RoleAssistant, Content: "all green"}, Done: true},
	}}
	root := t.TempDir()
	reg := tools.RegistryOf(tools.FileTools(root))
	_ = reg
	reg2 := append([]tools.Tool{}, tools.FileTools(root)...)
	_ = reg2
	// run_test 注入假执行器，第二次返回全绿
	runs := 0
	runner := &fakeTestRunner{onRun: func(cmd string) (string, int) {
		runs++
		if runs == 1 {
			return "--- FAIL: TestCalc (0.00s)\n    calc_test.go:12: want 3, got 2\nFAIL", 1
		}
		return "ok", 0
	}}
	r := &Runner{
		LLM:   mock,
		Tools: tools.RegistryOf(append(tools.FileTools(root), tools.TestTool(runner, root, "go test ./..."))),
		Guard: func(gc govern.GuardContext, a govern.Action) govern.Verdict {
			return govern.Check(govern.DefaultPolicy(), gc, a)
		},
		HITL:            govern.NewManager(),
		Policy:          govern.DefaultPolicy(),
		ApprovalTimeout: 50 * time.Millisecond,
		MaxTurns:        5,
		TimeBudget:      time.Minute,
		Store:           memory.NewStore(t.TempDir()),
		Feedback:        feedback.Parse,
		ApprovalDecider: func(_ context.Context, ap *govern.Approval) govern.ApprovalStatus {
			_ = ap // 测试环境自动拒绝
			return govern.Denied
		},
	}
	sess := &Session{ID: "s1", Repo: root, Task: "修复失败测试", TestCmd: "go test ./...", MaxTurns: 5}
	if err := r.Run(context.Background(), sess); err != nil {
		t.Fatal(err)
	}
	if sess.State != StateCompleted {
		t.Fatalf("应为 completed，got %s", sess.State)
	}
	// 反馈闭环断言：第二轮 run_test 的反馈必须包含结构化失败
	found := false
	for _, st := range sess.Steps {
		if st.ToolName == "run_test" && st.Feedback != "" && contains(st.Feedback, "calc_test.go") {
			found = true
		}
	}
	if !found {
		t.Fatal("run_test 失败反馈未回灌（反馈闭环失效）")
	}
}

type fakeTestRunner struct {
	onRun func(cmd string) (string, int)
}

func (f *fakeTestRunner) Run(_ context.Context, _, cmd string, _ int) (string, string, int, error) {
	out, code := f.onRun(cmd)
	return out, "", code, nil
}

func contains(s, sub string) bool { return strings.Contains(s, sub) }
