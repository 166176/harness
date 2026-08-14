package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// OpenAIClient 是 OpenAI 兼容 /v1/chat/completions 的真实 LLM 适配器（Task 16）。
// 实现 llm.Client 接口；API key 只出现在 Authorization 头中，绝不写入日志/错误信息。
type OpenAIClient struct {
	baseURL string
	apiKey  string
	model   string
	http    *http.Client
}

// NewOpenAIClient 构造客户端。baseURL 形如 https://api.deepseek.com（不带尾部斜杠亦可）。
func NewOpenAIClient(baseURL, apiKey, model string) *OpenAIClient {
	return &OpenAIClient{
		baseURL: strings.TrimRight(baseURL, "/"),
		apiKey:  apiKey,
		model:   model,
		http:    &http.Client{Timeout: 120 * time.Second},
	}
}

// chatRequest 对应 OpenAI chat/completions 请求体。
type chatRequest struct {
	Model    string        `json:"model"`
	Messages []chatMessage `json:"messages"`
	Tools    []chatTool    `json:"tools,omitempty"`
}

type chatMessage struct {
	Role       string     `json:"role"`
	Content    string     `json:"content"`
	ToolCalls  []chatCall `json:"tool_calls,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
}

type chatTool struct {
	Type     string       `json:"type"`
	Function chatFunction `json:"function"`
}

type chatFunction struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Parameters  map[string]any `json:"parameters"`
}

type chatCall struct {
	ID       string `json:"id"`
	Type     string `json:"type"` // OpenAI 兼容 tool_call 必须带 type:function（DeepSeek 验收实证）
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}

// chatResponse 对应 OpenAI chat/completions 响应体（只取所需字段）。
type chatResponse struct {
	Choices []struct {
		Message struct {
			Role      string     `json:"role"`
			Content   string     `json:"content"`
			ToolCalls []chatCall `json:"tool_calls"`
		} `json:"message"`
	} `json:"choices"`
}

// Complete 发送一轮对话并解析 choices[0].message.tool_calls。
// 响应无 tool_calls 时 Done=true（LLM 宣告任务结束）。
func (c *OpenAIClient) Complete(ctx context.Context, msgs []Message, tools []ToolSpec) (*Completion, error) {
	req := chatRequest{Model: c.model}
	for _, m := range msgs {
		cm := chatMessage{Role: string(m.Role), Content: m.Content, ToolCallID: m.ToolCallID}
		for _, tc := range m.ToolCalls {
			call := chatCall{ID: tc.ID, Type: "function"}
			call.Function.Name = tc.Name
			call.Function.Arguments = tc.Arguments
			cm.ToolCalls = append(cm.ToolCalls, call)
		}
		req.Messages = append(req.Messages, cm)
	}
	for _, t := range tools {
		req.Tools = append(req.Tools, chatTool{Type: "function", Function: chatFunction{
			Name: t.Name, Description: t.Description, Parameters: t.Parameters,
		}})
	}
	payload, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("llm: 请求序列化失败")
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/chat/completions", bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("llm: 请求构造失败")
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
	resp, err := c.http.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("llm: 请求失败：%v", redactKey(err.Error(), c.apiKey))
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return nil, fmt.Errorf("llm: 上游返回 %d：%s", resp.StatusCode, redactKey(string(body), c.apiKey))
	}
	var cr chatResponse
	if err := json.NewDecoder(resp.Body).Decode(&cr); err != nil {
		return nil, fmt.Errorf("llm: 响应解析失败")
	}
	if len(cr.Choices) == 0 {
		return &Completion{Message: Message{Role: RoleAssistant}, Done: true}, nil
	}
	msg := cr.Choices[0].Message
	comp := &Completion{Message: Message{Role: RoleAssistant, Content: msg.Content}}
	for _, tc := range msg.ToolCalls {
		comp.Message.ToolCalls = append(comp.Message.ToolCalls, ToolCall{
			ID: tc.ID, Name: tc.Function.Name, Arguments: tc.Function.Arguments,
		})
	}
	comp.Done = len(comp.Message.ToolCalls) == 0
	return comp, nil
}

// redactKey 从字符串中移除 API key（双保险：key 绝不落日志/错误信息）。
func redactKey(s, key string) string {
	if key == "" {
		return s
	}
	return strings.ReplaceAll(s, key, "***")
}
