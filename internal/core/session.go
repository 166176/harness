// Package core 实现 agent 主循环：决策封装、工具分发、护栏拦截、
// HITL 审批、反馈回灌与停机判断（SPEC §6.5，自研、不用任何框架）。
package core

// State 表示会话的停机状态。
type State string

const (
	StateRunning    State = "running"
	StateCompleted  State = "completed"
	StateFailed     State = "failed"
	StateTerminated State = "terminated"
)

// Step 记录主循环中单个工具动作的治理决策、参数、结果与结构化反馈。
type Step struct {
	Seq      int
	Decision string
	Rule     string
	ToolName string
	Args     map[string]any
	Result   string
	Feedback string
}

// Session 是一次修复任务的完整状态；JSON 落盘于 memory.Store（key "session:<id>"）。
type Session struct {
	ID, Repo, Task, TestCmd string
	State                   State
	Steps                   []Step
	MaxTurns                int
}
