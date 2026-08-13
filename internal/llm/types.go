package llm

import "context"

// Package llm 定义 LLM 抽象接口，供主循环/工具层/演示所有离线机制测试注入。
// 后续任务依赖的精确签名（Task 2 产出）。

type Role string

const (
	RoleSystem    Role = "system"
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
	RoleTool      Role = "tool"
)

// ToolCall 表示 LLM 请求执行的工具调用；Arguments 为 JSON 字符串。
type ToolCall struct {
	ID        string
	Name      string
	Arguments string
}

type Message struct {
	Role       Role
	Content    string
	ToolCalls  []ToolCall
	ToolCallID string
}

type ToolSpec struct {
	Name        string
	Description string
	Parameters  map[string]any
}

// Completion 为一次补全的结果；Done=true 表示 LLM 宣告任务结束。
type Completion struct {
	Message Message
	Done    bool
}

// Client 是所有 LLM（真实 OpenAI 兼容 / Mock）共用的抽象接口。
type Client interface {
	Complete(ctx context.Context, msgs []Message, tools []ToolSpec) (*Completion, error)
}
