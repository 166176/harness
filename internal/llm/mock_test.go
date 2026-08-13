package llm

import (
	"context"
	"testing"
)

func TestScriptedMockPlaysScriptAndTerminates(t *testing.T) {
	m := &ScriptedMock{Steps: []Completion{
		{Message: Message{Role: RoleAssistant, ToolCalls: []ToolCall{{ID: "c1", Name: "read_file", Arguments: `{"path":"a.go"}`}}}},
	}}
	ctx := context.Background()
	c1, err := m.Complete(ctx, nil, nil)
	if err != nil { t.Fatal(err) }
	if len(c1.Message.ToolCalls) != 1 || c1.Message.ToolCalls[0].Name != "read_file" {
		t.Fatalf("unexpected: %+v", c1)
	}
	c2, err := m.Complete(ctx, nil, nil) // 脚本耗尽后应宣告结束
	if err != nil { t.Fatal(err) }
	if !c2.Done { t.Fatal("脚本耗尽后 Done 应为 true") }
}
