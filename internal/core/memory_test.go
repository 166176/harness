package core

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/166176/harness/internal/govern"
	"github.com/166176/harness/internal/llm"
	"github.com/166176/harness/internal/memory"
	"github.com/166176/harness/internal/tools"
)

type recordingMock struct {
	steps   []llm.Completion
	calls   int
	lastMsg []llm.Message
}

func (m *recordingMock) Complete(_ context.Context, msgs []llm.Message, _ []llm.ToolSpec) (*llm.Completion, error) {
	m.lastMsg = msgs
	if m.calls >= len(m.steps) {
		return &llm.Completion{Message: llm.Message{Role: llm.RoleAssistant}, Done: true}, nil
	}
	c := m.steps[m.calls]
	m.calls++
	return &c, nil
}

func safeRunner(t *testing.T, hitl *govern.Manager, store *memory.Store, mock llm.Client, hint string, decider func(context.Context, *govern.Approval) govern.ApprovalStatus) *Runner {
	root := t.TempDir()
	return &Runner{
		LLM: mock,
		Tools: tools.RegistryOf(append(tools.FileTools(root),
			tools.ShellTool(&fakeTestRunner{onRun: func(string) (string, int) { return "ok", 0 }}, root))),
		Guard: func(gc govern.GuardContext, a govern.Action) govern.Verdict {
			return govern.Check(govern.DefaultPolicy(), gc, a)
		},
		HITL:            hitl,
		Policy:          govern.DefaultPolicy(),
		ApprovalTimeout: time.Second,
		MaxTurns:        5,
		TimeBudget:      time.Minute,
		Store:           store,
		ApprovalDecider: decider,
		Hint:            hint,
	}
}

func TestApprovalMemorySkipsSecondApproval(t *testing.T) {
	hitl := govern.NewManager()
	store := memory.NewStore(t.TempDir())
	calls := 0
	decider := func(ctx context.Context, ap *govern.Approval) govern.ApprovalStatus {
		calls++
		_ = hitl.Decide(ap.ID, govern.Approved, "test")
		return govern.Approved
	}
	mock := &recordingMock{steps: []llm.Completion{
		{Message: llm.Message{Role: llm.RoleAssistant, ToolCalls: []llm.ToolCall{{ID: "1", Name: "run_shell", Arguments: `{"command":"rm -rf .cache"}`}}}},
		{Message: llm.Message{Role: llm.RoleAssistant, ToolCalls: []llm.ToolCall{{ID: "2", Name: "run_shell", Arguments: `{"command":"rm -rf .cache"}`}}}},
		{Message: llm.Message{Role: llm.RoleAssistant}, Done: true},
	}}
	r := safeRunner(t, hitl, store, mock, "", decider)
	sess := &Session{ID: "s-mem", Repo: t.TempDir(), Task: "t", TestCmd: "go test ./...", MaxTurns: 5}
	if err := r.Run(context.Background(), sess); err != nil {
		t.Fatal(err)
	}
	if calls != 1 {
		t.Fatalf("同一命令指纹应只审批一次，实际 %d 次", calls)
	}
	if len(sess.Steps) != 2 {
		t.Fatalf("两次工具调用应恰好 2 个 step（无双重追加），实际 %d", len(sess.Steps))
	}
	memoryStep := false
	for _, st := range sess.Steps {
		if st.Rule == "approval-memory" {
			memoryStep = true
		}
	}
	if !memoryStep {
		t.Fatal("第二次相同命令应命中 approval-memory 规则")
	}
	// 消息完整性：每个 tool_call 恰一条 tool 结果（无同 ID 双消息）
	toolCount := 0
	seen := map[string]bool{}
	for _, m := range mock.lastMsg {
		if m.Role == llm.RoleTool {
			toolCount++
			if seen[m.ToolCallID] {
				t.Fatalf("tool_call_id %s 出现重复 tool 消息", m.ToolCallID)
			}
			seen[m.ToolCallID] = true
		}
	}
	if toolCount != 2 {
		t.Fatalf("应有 2 条 tool 结果消息，实际 %d", toolCount)
	}
}

func TestHintInjectedIntoSystemPrompt(t *testing.T) {
	hitl := govern.NewManager()
	store := memory.NewStore(t.TempDir())
	mock := &recordingMock{steps: []llm.Completion{{Message: llm.Message{Role: llm.RoleAssistant}, Done: true}}}
	decider := func(ctx context.Context, ap *govern.Approval) govern.ApprovalStatus { return govern.Denied }
	r := safeRunner(t, hitl, store, mock, "测试命令已确认：go test ./...", decider)
	sess := &Session{ID: "s-hint", Repo: t.TempDir(), Task: "t", TestCmd: "go test ./...", MaxTurns: 5}
	if err := r.Run(context.Background(), sess); err != nil {
		t.Fatal(err)
	}
	if len(mock.lastMsg) == 0 || !strings.Contains(mock.lastMsg[0].Content, "测试命令已确认") {
		t.Fatalf("system 提示应注入项目约定 hint：%+v", mock.lastMsg)
	}
}
