package llm

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// brief Step 1 逐字测试：httptest 假服务端验证请求构造与响应解析，不真联网。
func TestOpenAIClientParsesToolCall(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer sk-test" {
			t.Fatal("缺认证头")
		}
		json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{{"message": map[string]any{
				"role":       "assistant",
				"tool_calls": []map[string]any{{"id": "c1", "function": map[string]any{"name": "read_file", "arguments": `{"path":"a.go"}`}}},
			}}},
		})
	}))
	defer srv.Close()
	c := NewOpenAIClient(srv.URL, "sk-test", "deepseek-chat")
	comp, err := c.Complete(context.Background(), []Message{{Role: RoleUser, Content: "hi"}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(comp.Message.ToolCalls) != 1 || comp.Message.ToolCalls[0].Name != "read_file" {
		t.Fatalf("%+v", comp)
	}
}

// 增补：请求必须 POST {base}/chat/completions，且请求体携带 model。
// handler 在独立 goroutine 中运行，错误经 channel 回传（避免跨 goroutine t.Fatal）。
func TestOpenAIClientPostsToChatCompletions(t *testing.T) {
	got := make(chan string, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			got <- "method=" + r.Method
			return
		}
		if r.URL.Path != "/chat/completions" {
			got <- "path=" + r.URL.Path
			return
		}
		var body struct {
			Model string `json:"model"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Model != "deepseek-chat" {
			got <- "model mismatch"
			return
		}
		json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{{"message": map[string]any{"role": "assistant", "content": "ok"}}},
		})
		close(got)
	}))
	defer srv.Close()
	c := NewOpenAIClient(srv.URL, "sk-test", "deepseek-chat")
	if _, err := c.Complete(context.Background(), []Message{{Role: RoleUser, Content: "hi"}}, nil); err != nil {
		t.Fatal(err)
	}
	if msg := <-got; msg != "" {
		t.Fatal(msg)
	}
}

// 增补：错误路径不得在 error 中泄露 API key（key 不落日志）。
func TestOpenAIClientErrorDoesNotLeakKey(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error":"boom"}`))
	}))
	defer srv.Close()
	c := NewOpenAIClient(srv.URL, "sk-super-secret-key", "deepseek-chat")
	_, err := c.Complete(context.Background(), []Message{{Role: RoleUser, Content: "hi"}}, nil)
	if err == nil {
		t.Fatal("非 200 应返回错误")
	}
	if strings.Contains(err.Error(), "sk-super-secret-key") {
		t.Fatalf("错误信息泄露 API key：%v", err)
	}
}

// TestOpenAIClientToolCallsIncludeType 回归测试：真实 DeepSeek 验收发现
// 请求体 tool_calls 缺 `type:"function"` 会报 "messages[n]: missing field type"
// （400 invalid_request_error）。OpenAI 兼容 schema 要求每个 tool_call 带 type。
func TestOpenAIClientToolCallsIncludeType(t *testing.T) {
	var got []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got, _ = io.ReadAll(r.Body)
		json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{{"message": map[string]any{"role": "assistant", "content": "ok"}}},
		})
	}))
	defer srv.Close()
	c := NewOpenAIClient(srv.URL, "sk-test", "deepseek-chat")
	msgs := []Message{{
		Role:      RoleAssistant,
		Content:   "",
		ToolCalls: []ToolCall{{ID: "call_1", Name: "read_file", Arguments: `{"path":"a.go"}`}},
	}}
	if _, err := c.Complete(context.Background(), msgs, nil); err != nil {
		t.Fatal(err)
	}
	var raw struct {
		Messages []struct {
			ToolCalls []map[string]any `json:"tool_calls"`
		} `json:"messages"`
	}
	if err := json.Unmarshal(got, &raw); err != nil {
		t.Fatalf("解析请求体失败: %v", err)
	}
	if len(raw.Messages) != 1 || len(raw.Messages[0].ToolCalls) != 1 {
		t.Fatalf("请求结构不符: %s", got)
	}
	tc := raw.Messages[0].ToolCalls[0]
	if tc["type"] != "function" {
		t.Fatalf("tool_calls[0] 缺 type=function，got %s", got)
	}
}

// 增补：响应无 tool_calls 时应宣告 Done=true（循环据此结束会话）。
func TestOpenAIClientDoneWhenNoToolCalls(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{{"message": map[string]any{"role": "assistant", "content": "已修复"}}},
		})
	}))
	defer srv.Close()
	c := NewOpenAIClient(srv.URL, "sk-test", "deepseek-chat")
	comp, err := c.Complete(context.Background(), []Message{{Role: RoleUser, Content: "hi"}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !comp.Done {
		t.Fatalf("无 tool_calls 时应 Done=true：%+v", comp)
	}
}
