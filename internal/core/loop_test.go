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

// 治理优先项目默认值必须 fail-closed：nil Guard 不得放行任何工具动作。
func TestNilGuardFailsClosed(t *testing.T) {
	mock := &llm.ScriptedMock{Steps: []llm.Completion{
		{Message: llm.Message{Role: llm.RoleAssistant, ToolCalls: []llm.ToolCall{{ID: "1", Name: "write_file", Arguments: `{"path":"calc.go","content":"fixed"}`}}}}, // Guard 缺失时该动作必须被拦截
	}}
	r := &Runner{
		LLM:        mock,
		Tools:      tools.RegistryOf(nil), // 空 Registry
		Guard:      nil,                   // 关键：护栏未装配
		HITL:       govern.NewManager(),
		MaxTurns:   3,
		TimeBudget: time.Minute,
		Store:      memory.NewStore(t.TempDir()),
	}
	sess := &Session{ID: "s1", Repo: t.TempDir(), Task: "t", TestCmd: "go test ./...", MaxTurns: 3}
	if err := r.Run(context.Background(), sess); err != nil {
		t.Fatalf("Run 不应 panic，也不应以其他方式失败：%v", err)
	}
	// 该动作不得被执行：若存在对应 Step，其 Decision 必须为 Deny。
	for _, st := range sess.Steps {
		if st.ToolName == "write_file" {
			if st.Decision != string(govern.Deny) {
				t.Fatalf("nil Guard 下 write_file 被放行：decision=%q rule=%q", st.Decision, st.Rule)
			}
			if st.Rule != "guard-not-configured" {
				t.Fatalf("预期规则 guard-not-configured，got %q", st.Rule)
			}
			return
		}
	}
	t.Fatal("未记录 write_file 的治理决策步骤")
}

// Guard 返回 NeedsApproval 而 HITL 未装配时，应返回配置错误而非 panic。
func TestNilHITLReturnsError(t *testing.T) {
	mock := &llm.ScriptedMock{Steps: []llm.Completion{
		{Message: llm.Message{Role: llm.RoleAssistant, ToolCalls: []llm.ToolCall{{ID: "1", Name: "run_shell", Arguments: `{"command":"go test ./..."}`}}}}, // 触发审批路径
	}}
	r := &Runner{
		LLM:   mock,
		Tools: tools.RegistryOf(nil),
		Guard: func(govern.GuardContext, govern.Action) govern.Verdict {
			return govern.Verdict{Decision: govern.NeedsApproval, Rule: "test"}
		},
		HITL:     nil, // 关键：审批管理器未装配
		MaxTurns: 3,
		Store:    memory.NewStore(t.TempDir()),
	}
	sess := &Session{ID: "s1", Repo: t.TempDir(), Task: "t", TestCmd: "go test ./...", MaxTurns: 3}
	err := r.Run(context.Background(), sess)
	if err == nil {
		t.Fatal("预期 nil HITL 返回配置错误，got nil")
	}
	if !strings.Contains(err.Error(), "HITL not configured") {
		t.Fatalf("错误信息不符：%v", err)
	}
}

// F1：HITL.Create 后、Await 解除前会话必须已落盘，否则控制台看不到首轮审批。
func TestSessionPersistedBeforeApprovalAwait(t *testing.T) {
	mock := &llm.ScriptedMock{Steps: []llm.Completion{
		{Message: llm.Message{Role: llm.RoleAssistant, ToolCalls: []llm.ToolCall{{ID: "1", Name: "run_shell", Arguments: `{"command":"go test ./..."}`}}}},
	}}
	store := memory.NewStore(t.TempDir())
	entered := make(chan struct{})
	release := make(chan struct{})
	r := &Runner{
		LLM:   mock,
		Tools: tools.RegistryOf(nil),
		Guard: func(govern.GuardContext, govern.Action) govern.Verdict {
			return govern.Verdict{Decision: govern.NeedsApproval, Rule: "test"}
		},
		HITL:     govern.NewManager(),
		MaxTurns: 3,
		Store:    store,
		ApprovalDecider: func(_ context.Context, _ *govern.Approval) govern.ApprovalStatus {
			close(entered)
			<-release
			return govern.Denied
		},
	}
	sess := &Session{ID: "s1", Repo: t.TempDir(), Task: "t", TestCmd: "go test ./...", MaxTurns: 3}
	runErr := make(chan error, 1)
	go func() { runErr <- r.Run(context.Background(), sess) }()
	select {
	case <-entered:
	case <-time.After(5 * time.Second):
		t.Fatal("decider 未被调用（审批路径未触发）")
	}
	// decider 阻塞 = Await 尚未解除；此刻 Store 必须已含该会话记录。
	var got map[string]any
	ok, err := store.Get("session:"+sess.ID, &got)
	if err != nil || !ok {
		t.Fatalf("Await 解除前 Store 无会话记录: ok=%v err=%v", ok, err)
	}
	close(release)
	if err := <-runErr; err != nil {
		t.Fatal(err)
	}
}

// Tools 未装配时，工具分发应返回配置错误而非 panic。
func TestNilToolsReturnsError(t *testing.T) {
	mock := &llm.ScriptedMock{Steps: []llm.Completion{
		{Message: llm.Message{Role: llm.RoleAssistant, ToolCalls: []llm.ToolCall{{ID: "1", Name: "run_shell", Arguments: `{"command":"go test ./..."}`}}}}, // 触发工具分发路径
	}}
	r := &Runner{
		LLM:   mock,
		Tools: nil, // 关键：工具注册表未装配
		Guard: func(govern.GuardContext, govern.Action) govern.Verdict {
			return govern.Verdict{Decision: govern.Allow}
		},
		HITL:     govern.NewManager(),
		MaxTurns: 3,
		Store:    memory.NewStore(t.TempDir()),
	}
	sess := &Session{ID: "s1", Repo: t.TempDir(), Task: "t", TestCmd: "go test ./...", MaxTurns: 3}
	err := r.Run(context.Background(), sess)
	if err == nil {
		t.Fatal("预期 nil Tools 返回配置错误，got nil")
	}
	if !strings.Contains(err.Error(), "Tools not configured") {
		t.Fatalf("错误信息不符：%v", err)
	}
}
