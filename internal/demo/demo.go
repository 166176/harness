// Package demo 用纯 Mock LLM 确定性演示三大机制（§A.6）：
// ① 护栏拦截 ② 失败反馈闭环 ③ HITL 超时自动拒绝。
// 全程不联网、不读凭据。
package demo

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/166176/harness/internal/core"
	"github.com/166176/harness/internal/govern"
	"github.com/166176/harness/internal/llm"
	"github.com/166176/harness/internal/memory"
	"github.com/166176/harness/internal/tools"
)

type Result struct {
	Name  string
	Pass  bool
	Trace []string
}

// Run 在纯 MockLLM 下确定性执行三个场景，不联网、不读凭据。
func Run(ctx context.Context) []Result {
	return []Result{
		scenarioGuardrailIntercept(ctx),
		scenarioFeedbackLoop(ctx),
		scenarioHITLTimeoutAutodeny(ctx),
	}
}

// recorder 累积场景 Trace；bad 标记 Pass=false。
type recorder struct {
	pass bool
	tr   []string
}

func (r *recorder) ok(format string, args ...any) { r.tr = append(r.tr, fmt.Sprintf(format, args...)) }

func (r *recorder) bad(format string, args ...any) {
	r.pass = false
	r.tr = append(r.tr, "FAIL: "+fmt.Sprintf(format, args...))
}

func (r *recorder) result(name string) Result { return Result{Name: name, Pass: r.pass, Trace: r.tr} }

// countingRunner 假执行器：只计数，不真正执行任何命令。
type countingRunner struct{ calls int }

func (f *countingRunner) Run(_ context.Context, _, _ string, _ int) (string, string, int, error) {
	f.calls++
	return "ok", "", 0, nil
}

// fakeTestRunner 假测试执行器：第一次返回失败输出，之后返回全绿。
type fakeTestRunner struct{ runs int }

func (f *fakeTestRunner) Run(_ context.Context, _, _ string, _ int) (string, string, int, error) {
	f.runs++
	if f.runs == 1 {
		return "--- FAIL: TestCalc (0.00s)\n    calc_test.go:12: want 3, got 2\nFAIL", "", 1, nil
	}
	return "ok", "", 0, nil
}

// recordingMock 记录传给 LLM 的每条消息，用于断言反馈是否回灌。
type recordingMock struct {
	inner llm.Client
	seen  []llm.Message
}

func (m *recordingMock) Complete(ctx context.Context, msgs []llm.Message, ts []llm.ToolSpec) (*llm.Completion, error) {
	m.seen = append(m.seen, msgs...)
	return m.inner.Complete(ctx, msgs, ts)
}

func mockSaw(m *recordingMock, sub string) bool {
	for _, msg := range m.seen {
		if strings.Contains(msg.Content, sub) {
			return true
		}
	}
	return false
}

// 场景 1：护栏拦截。mock 发 run_shell "rm -rf ." → Check 判 NeedsApproval → Trace 记命中规则。
func scenarioGuardrailIntercept(ctx context.Context) Result {
	rec := &recorder{pass: true}
	action := govern.Action{Tool: "run_shell", Args: map[string]any{"command": "rm -rf ."}}
	v := govern.Check(govern.DefaultPolicy(), govern.GuardContext{}, action)
	if v.Decision != govern.NeedsApproval {
		rec.bad("govern.Check 应为 NeedsApproval，got %s (rule=%s)", v.Decision, v.Rule)
		return rec.result("guardrail-intercept")
	}
	rec.ok("govern.Check 判定 NeedsApproval，命中规则 %q", v.Rule)

	root, err := os.MkdirTemp("", "gavel-demo-s1-")
	if err != nil {
		rec.bad("临时目录创建失败：%v", err)
		return rec.result("guardrail-intercept")
	}
	defer os.RemoveAll(root)

	fake := &countingRunner{}
	mock := &llm.ScriptedMock{Steps: []llm.Completion{{
		Message: llm.Message{Role: llm.RoleAssistant, ToolCalls: []llm.ToolCall{{ID: "1", Name: "run_shell", Arguments: `{"command":"rm -rf ."}`}}},
	}}}
	r := &core.Runner{
		LLM:   mock,
		Tools: tools.RegistryOf([]tools.Tool{tools.ShellTool(fake, root)}),
		Guard: func(gc govern.GuardContext, a govern.Action) govern.Verdict {
			return govern.Check(govern.DefaultPolicy(), gc, a)
		},
		HITL:            govern.NewManager(),
		Policy:          govern.DefaultPolicy(),
		ApprovalTimeout: 50 * time.Millisecond,
		MaxTurns:        3,
		Store:           memory.NewStore(root),
		ApprovalDecider: func(_ context.Context, _ *govern.Approval) govern.ApprovalStatus {
			return govern.Denied
		},
	}
	sess := &core.Session{ID: "demo-s1", Repo: root, Task: "演示护栏拦截", TestCmd: "go test ./...", MaxTurns: 3}
	if err := r.Run(ctx, sess); err != nil {
		rec.bad("Run 失败：%v", err)
		return rec.result("guardrail-intercept")
	}
	found := false
	for _, st := range sess.Steps {
		if st.ToolName != "run_shell" {
			continue
		}
		found = true
		switch {
		case st.Decision != string(govern.NeedsApproval):
			rec.bad("步骤决策应为 approval，got %q", st.Decision)
		case st.Rule != v.Rule:
			rec.bad("步骤命中规则应为 %q，got %q", v.Rule, st.Rule)
		default:
			rec.ok("run_shell 步骤被拦截：decision=approval rule=%q result=%q", st.Rule, st.Result)
		}
	}
	if !found {
		rec.bad("未记录 run_shell 的治理步骤")
	}
	if fake.calls != 0 {
		rec.bad("护栏拦截后工具仍被执行 %d 次", fake.calls)
	} else {
		rec.ok("工具未被实际执行（执行器调用数 0）")
	}
	return rec.result("guardrail-intercept")
}

// 场景 2：反馈闭环。mock 序列 [write_file, run_test, write_file(修复)]；
// 假执行器第一次返回失败输出、之后全绿；断言 run_test 反馈含文件/行号，
// 且第二步 write_file 与反馈一致（改写反馈对应源文件、内容反映 want 值）。
func scenarioFeedbackLoop(ctx context.Context) Result {
	rec := &recorder{pass: true}
	root, err := os.MkdirTemp("", "gavel-demo-s2-")
	if err != nil {
		rec.bad("临时目录创建失败：%v", err)
		return rec.result("feedback-loop")
	}
	defer os.RemoveAll(root)

	rmock := &recordingMock{inner: &llm.ScriptedMock{Steps: []llm.Completion{
		{Message: llm.Message{Role: llm.RoleAssistant, ToolCalls: []llm.ToolCall{{ID: "1", Name: "write_file", Arguments: `{"path":"calc.go","content":"package calc\n\nfunc Calc() int { return 2 }\n"}`}}}},
		{Message: llm.Message{Role: llm.RoleAssistant, ToolCalls: []llm.ToolCall{{ID: "2", Name: "run_test", Arguments: `{}`}}}},
		{Message: llm.Message{Role: llm.RoleAssistant, ToolCalls: []llm.ToolCall{{ID: "3", Name: "write_file", Arguments: `{"path":"calc.go","content":"package calc\n\nfunc Calc() int { return 3 }\n"}`}}}},
	}}}
	fake := &fakeTestRunner{}
	r := &core.Runner{
		LLM:   rmock,
		Tools: tools.RegistryOf(append(tools.FileTools(root), tools.TestTool(fake, root, "go test ./..."))),
		Guard: func(gc govern.GuardContext, a govern.Action) govern.Verdict {
			return govern.Check(govern.DefaultPolicy(), gc, a)
		},
		HITL:            govern.NewManager(),
		Policy:          govern.DefaultPolicy(),
		ApprovalTimeout: 50 * time.Millisecond,
		MaxTurns:        6,
		Store:           memory.NewStore(root),
	}
	sess := &core.Session{ID: "demo-s2", Repo: root, Task: "修复失败测试", TestCmd: "go test ./...", MaxTurns: 6}
	if err := r.Run(ctx, sess); err != nil {
		rec.bad("Run 失败：%v", err)
		return rec.result("feedback-loop")
	}
	if sess.State != core.StateCompleted {
		rec.bad("会话应为 completed，got %s", sess.State)
	}

	// 断言 run_test 的 Step.Feedback 含解析出的文件/行号。
	fb := ""
	for _, st := range sess.Steps {
		if st.ToolName == "run_test" && st.Feedback != "" {
			fb = st.Feedback
		}
	}
	if fb == "" {
		rec.bad("run_test 步骤无结构化反馈")
		return rec.result("feedback-loop")
	}
	if !strings.Contains(fb, "calc_test.go:12") {
		rec.bad("反馈未含解析出的文件/行号 calc_test.go:12：%q", fb)
	} else {
		rec.ok("run_test 反馈含解析出的文件/行号：%q", fb)
	}

	// 断言反馈已回灌给 LLM（记录 mock 收到的全部消息）。
	if !mockSaw(rmock, "calc_test.go:12") {
		rec.bad("LLM 未收到含反馈的消息（反馈闭环断裂）")
	} else {
		rec.ok("失败反馈已作为消息回灌给 LLM")
	}

	// 断言第二步 write_file 与反馈一致：目标文件 = 反馈文件去 _test 后缀，内容反映 want 3。
	var writes []core.Step
	for _, st := range sess.Steps {
		if st.ToolName == "write_file" {
			writes = append(writes, st)
		}
	}
	if len(writes) < 2 {
		rec.bad("应有两次 write_file（初始 + 修复），got %d", len(writes))
		return rec.result("feedback-loop")
	}
	fix := writes[len(writes)-1]
	p, _ := fix.Args["path"].(string)
	c, _ := fix.Args["content"].(string)
	wantPath := strings.TrimSuffix(strings.SplitN(fb, ":", 2)[0], "_test.go") + ".go"
	switch {
	case p != wantPath:
		rec.bad("修复目标应为 %s（反馈文件去 _test 后缀），got %s", wantPath, p)
	case !strings.Contains(c, "return 3"):
		rec.bad("修复内容未反映反馈中的 want 3：%q", c)
	default:
		rec.ok("修复动作与反馈一致：改写 %s，内容反映 want 3", p)
	}
	return rec.result("feedback-loop")
}

// 场景 3：HITL 超时自动拒绝。mock 发危险命令 → ApprovalTimeout=50ms、
// ApprovalDecider=Await → 断言审批状态 Timeout 且工具未执行。
func scenarioHITLTimeoutAutodeny(ctx context.Context) Result {
	rec := &recorder{pass: true}
	root, err := os.MkdirTemp("", "gavel-demo-s3-")
	if err != nil {
		rec.bad("临时目录创建失败：%v", err)
		return rec.result("hitl-timeout-autodeny")
	}
	defer os.RemoveAll(root)

	fake := &countingRunner{}
	mock := &llm.ScriptedMock{Steps: []llm.Completion{{
		Message: llm.Message{Role: llm.RoleAssistant, ToolCalls: []llm.ToolCall{{ID: "1", Name: "run_shell", Arguments: `{"command":"rm -rf ."}`}}},
	}}}
	hitl := govern.NewManager()
	var apID string
	var got govern.ApprovalStatus
	r := &core.Runner{
		LLM:   mock,
		Tools: tools.RegistryOf([]tools.Tool{tools.ShellTool(fake, root)}),
		Guard: func(gc govern.GuardContext, a govern.Action) govern.Verdict {
			return govern.Check(govern.DefaultPolicy(), gc, a)
		},
		HITL:            hitl,
		Policy:          govern.DefaultPolicy(),
		ApprovalTimeout: 50 * time.Millisecond,
		MaxTurns:        3,
		Store:           memory.NewStore(root),
		ApprovalDecider: func(c context.Context, ap *govern.Approval) govern.ApprovalStatus {
			apID = ap.ID
			got = hitl.Await(c, ap.ID, 50*time.Millisecond) // 无人决断 → 超时自动拒绝
			return got
		},
	}
	sess := &core.Session{ID: "demo-s3", Repo: root, Task: "演示 HITL 超时", TestCmd: "go test ./...", MaxTurns: 3}
	if err := r.Run(ctx, sess); err != nil {
		rec.bad("Run 失败：%v", err)
		return rec.result("hitl-timeout-autodeny")
	}
	if got != govern.Timeout {
		rec.bad("审批状态应为 timeout，got %s", got)
	} else {
		rec.ok("审批 50ms 无人决断 → 自动拒绝（status=timeout）")
	}
	if ap, ok := hitl.Get(apID); !ok || ap.Status != govern.Timeout {
		rec.bad("Manager 中审批状态应为 timeout（ok=%v）", ok)
	}
	if fake.calls != 0 {
		rec.bad("超时拒绝后工具仍被执行 %d 次", fake.calls)
	} else {
		rec.ok("工具未被执行（执行器调用数 0）")
	}
	found := false
	for _, st := range sess.Steps {
		if st.ToolName == "run_shell" {
			found = true
			if !strings.Contains(st.Result, "timeout") {
				rec.bad("步骤结果未体现 timeout：%q", st.Result)
			}
		}
	}
	if !found {
		rec.bad("未记录 run_shell 步骤")
	}
	return rec.result("hitl-timeout-autodeny")
}
