package core

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/166176/harness/internal/feedback"
	"github.com/166176/harness/internal/govern"
	"github.com/166176/harness/internal/llm"
	"github.com/166176/harness/internal/memory"
	"github.com/166176/harness/internal/tools"
)

// Runner 主循环（§A.4-A：只依赖接口，测试全部注入 Mock）。
type Runner struct {
	LLM             llm.Client
	Tools           *tools.Registry
	Guard           func(govern.GuardContext, govern.Action) govern.Verdict // 注入 Check，测试可替换
	HITL            *govern.Manager
	Policy          govern.Policy
	ApprovalTimeout time.Duration
	MaxTurns        int
	TimeBudget      time.Duration
	Store           *memory.Store
	Feedback        func(format, out string, code int) []feedback.TestFailure            // 注入，测试可替换
	ApprovalDecider func(ctx context.Context, ap *govern.Approval) govern.ApprovalStatus // 测试注入自动 deny
}

var exitRe = regexp.MustCompile(`exit=(\d+)`)

// Run 执行主循环直至停机：Completed（Done 或测试全绿）、
// Failed（轮数/时间预算耗尽、连续 3 轮相同失败指纹）。
func (r *Runner) Run(ctx context.Context, sess *Session) error {
	sess.State = StateRunning
	maxTurns := sess.MaxTurns
	if maxTurns <= 0 {
		maxTurns = r.MaxTurns
	}
	if maxTurns <= 0 {
		maxTurns = 20
	}
	deadline := time.Time{}
	if r.TimeBudget > 0 {
		deadline = time.Now().Add(r.TimeBudget)
	}
	decider := r.ApprovalDecider
	if decider == nil {
		decider = func(ctx context.Context, ap *govern.Approval) govern.ApprovalStatus {
			return r.HITL.Await(ctx, ap.ID, r.ApprovalTimeout)
		}
	}
	feed := r.Feedback
	if feed == nil {
		feed = feedback.Parse
	}

	var rounds [][]llm.Message // 每轮 = assistant 消息 + 各工具结果
	var lastFingerprint string
	consecutive := 0
	latestFeedback := ""

	for turn := 0; turn < maxTurns; turn++ {
		if !deadline.IsZero() && !time.Now().Before(deadline) {
			sess.State = StateFailed
			break
		}
		msgs := r.assemble(sess, rounds, latestFeedback)
		resp, err := r.LLM.Complete(ctx, msgs, r.toolSpecs())
		if err != nil {
			return err
		}
		fp := fingerprint(resp.Message)
		if fp == lastFingerprint {
			consecutive++
		} else {
			lastFingerprint = fp
			consecutive = 1
		}
		if consecutive >= 3 {
			sess.State = StateFailed
			break
		}
		round := []llm.Message{resp.Message}
		if resp.Done {
			rounds = append(rounds, round)
			sess.State = StateCompleted
			break
		}
		stop := false
		for _, call := range resp.Message.ToolCalls {
			step := Step{Seq: len(sess.Steps) + 1, ToolName: call.Name}
			args, perr := parseArgs(call.Arguments)
			if perr != nil {
				step.Decision = string(govern.Deny)
				step.Rule = "bad-arguments"
				step.Result = perr.Error()
				sess.Steps = append(sess.Steps, step)
				round = append(round, toolResult(call.ID, "拒绝：参数解析失败 "+perr.Error()))
				continue
			}
			step.Args = args
			action := govern.Action{Tool: call.Name, Args: args}
			v := r.verdict(sess, action)
			step.Decision = string(v.Decision)
			step.Rule = v.Rule
			switch v.Decision {
			case govern.Deny:
				msg := "拒绝：命中规则 " + v.Rule
				step.Result = msg
				sess.Steps = append(sess.Steps, step)
				round = append(round, toolResult(call.ID, msg))
				continue
			case govern.NeedsApproval:
				ap, cerr := r.HITL.Create(sess.ID, action, v.Rule, v.Rule)
				if cerr != nil {
					step.Result = cerr.Error()
					sess.Steps = append(sess.Steps, step)
					round = append(round, toolResult(call.ID, "拒绝：审批创建失败 "+cerr.Error()))
					continue
				}
				status := decider(ctx, ap)
				if status != govern.Approved {
					msg := "拒绝：审批未通过（" + string(status) + "）"
					step.Result = msg
					sess.Steps = append(sess.Steps, step)
					round = append(round, toolResult(call.ID, msg))
					continue
				}
			}
			out, derr := r.Tools.Dispatch(ctx, call.Name, args)
			if derr != nil {
				step.Result = derr.Error()
				sess.Steps = append(sess.Steps, step)
				round = append(round, toolResult(call.ID, "执行失败："+derr.Error()))
				continue
			}
			step.Result = out
			if call.Name == "run_test" {
				code := exitCode(out)
				format := "gotest"
				if strings.Contains(sess.TestCmd, "pytest") {
					format = "pytest"
				}
				fails := feed(format, out, code)
				step.Feedback = formatFailures(fails)
				sess.Steps = append(sess.Steps, step)
				round = append(round, toolResult(call.ID, out))
				if code == 0 && len(fails) == 0 {
					latestFeedback = ""
					sess.State = StateCompleted
					stop = true
					break
				}
				latestFeedback = step.Feedback
				continue
			}
			sess.Steps = append(sess.Steps, step)
			round = append(round, toolResult(call.ID, out))
		}
		rounds = append(rounds, round)
		if stop {
			break
		}
		if err := r.persist(sess); err != nil {
			return err
		}
	}
	if sess.State == StateRunning {
		sess.State = StateFailed
	}
	return r.persist(sess)
}

// assemble 组装本轮消息：system 提示（任务、repo、工具说明）+ 最近 5 轮 + 最新反馈。
func (r *Runner) assemble(sess *Session, rounds [][]llm.Message, latestFeedback string) []llm.Message {
	msgs := []llm.Message{{Role: llm.RoleSystem, Content: r.systemPrompt(sess)}}
	start := 0
	if len(rounds) > 5 {
		start = len(rounds) - 5
	}
	for _, round := range rounds[start:] {
		msgs = append(msgs, round...)
	}
	if latestFeedback != "" {
		msgs = append(msgs, llm.Message{Role: llm.RoleUser, Content: "最新测试反馈：\n" + latestFeedback})
	}
	return msgs
}

func (r *Runner) systemPrompt(sess *Session) string {
	var b strings.Builder
	fmt.Fprintf(&b, "你是 gavel 编码 agent，目标是修复仓库中失败的测试。\n任务：%s\n仓库：%s\n测试命令：%s\n", sess.Task, sess.Repo, sess.TestCmd)
	b.WriteString("可用工具：\n")
	for _, s := range r.toolSpecs() {
		fmt.Fprintf(&b, "- %s: %s\n", s.Name, s.Description)
	}
	b.WriteString("每轮可调用工具；收到 run_test 反馈后继续修改，直到测试全绿。")
	return b.String()
}

func (r *Runner) toolSpecs() []llm.ToolSpec {
	if r.Tools == nil {
		return nil
	}
	return r.Tools.Specs()
}

// verdict 默认放行（Guard 为 nil 时），生产路径注入 govern.Check。
func (r *Runner) verdict(sess *Session, a govern.Action) govern.Verdict {
	if r.Guard == nil {
		return govern.Verdict{Decision: govern.Allow}
	}
	return r.Guard(govern.GuardContext{RepoRoot: sess.Repo}, a)
}

func (r *Runner) persist(sess *Session) error {
	if r.Store == nil {
		return nil
	}
	return r.Store.Put("session:"+sess.ID, sess)
}

func parseArgs(raw string) (map[string]any, error) {
	args := map[string]any{}
	if strings.TrimSpace(raw) == "" {
		return args, nil
	}
	if err := json.Unmarshal([]byte(raw), &args); err != nil {
		return nil, fmt.Errorf("arguments 不是合法 JSON：%w", err)
	}
	return args, nil
}

func exitCode(out string) int {
	if m := exitRe.FindStringSubmatch(out); m != nil {
		if n, err := strconv.Atoi(m[1]); err == nil {
			return n
		}
	}
	return -1
}

func formatFailures(fails []feedback.TestFailure) string {
	if len(fails) == 0 {
		return ""
	}
	lines := make([]string, 0, len(fails))
	for _, f := range fails {
		lines = append(lines, fmt.Sprintf("%s:%d: %s [%s]", f.File, f.Line, f.Message, f.Kind))
	}
	return strings.Join(lines, "\n")
}

func fingerprint(m llm.Message) string {
	b, err := json.Marshal(m)
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

func toolResult(id, content string) llm.Message {
	return llm.Message{Role: llm.RoleTool, ToolCallID: id, Content: content}
}
