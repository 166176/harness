package llm

import "context"

// ScriptedMock 按脚本队列返回响应；耗尽后返回 Done=true。
// 供所有离线机制测试使用，不联网。
type ScriptedMock struct {
	Steps []Completion
	calls int
}

func (m *ScriptedMock) Complete(_ context.Context, _ []Message, _ []ToolSpec) (*Completion, error) {
	if m.calls >= len(m.Steps) {
		return &Completion{Message: Message{Role: RoleAssistant}, Done: true}, nil
	}
	c := m.Steps[m.calls]
	m.calls++
	return &c, nil
}
