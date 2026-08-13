package cli

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/166176/harness/internal/govern"
	"github.com/166176/harness/internal/llm"
	"github.com/166176/harness/internal/secret"
)

// fakeSecretProvider 注入固定凭据，避免测试触碰真实 keyring/.env。
type fakeSecretProvider struct{}

func (fakeSecretProvider) Name() string         { return "fake" }
func (fakeSecretProvider) Get() (string, error) { return "sk-secret-value-1234", nil }
func (fakeSecretProvider) Set(string) error     { return nil }
func (fakeSecretProvider) Clear() error         { return nil }

// emptySecretProvider 模拟未配置凭据的环境。
type emptySecretProvider struct{}

func (emptySecretProvider) Name() string         { return "fake" }
func (emptySecretProvider) Get() (string, error) { return "", secret.ErrNotFound }
func (emptySecretProvider) Set(string) error     { return nil }
func (emptySecretProvider) Clear() error         { return nil }

func TestParseKeyStatusShowsNoPlaintext(t *testing.T) {
	out, code := RunForTest([]string{"gavel", "key", "status"}, fakeSecretProvider{})
	if code != 0 {
		t.Fatalf("exit=%d out=%s", code, out)
	}
	if contains(out, "sk-secret-value-1234") {
		t.Fatal("status 回显明文")
	}
	if !contains(out, "mask") {
		t.Fatalf("out=%s", out)
	}
}

func TestDemoExitZero(t *testing.T) {
	_, code := RunForTest([]string{"gavel", "demo"}, fakeSecretProvider{})
	if code != 0 {
		t.Fatalf("demo 应退出 0")
	}
}

// SPEC §3.2 补充断言：demo --json 输出合法 JSON 且三场景全 PASS。
func TestDemoJSON(t *testing.T) {
	out, code := RunForTest([]string{"gavel", "demo", "--json"}, fakeSecretProvider{})
	if code != 0 {
		t.Fatalf("exit=%d out=%s", code, out)
	}
	if !json.Valid([]byte(out)) {
		t.Fatalf("输出不是合法 JSON：%s", out)
	}
	if !contains(out, "guardrail-intercept") {
		t.Fatalf("JSON 应包含三个场景名：%s", out)
	}
}

// SPEC §3.2：run 缺必填 flag → 退出码 3。
func TestRunMissingRequiredFlagsExit3(t *testing.T) {
	out, code := RunForTest([]string{"gavel", "run"}, fakeSecretProvider{})
	if code != 3 {
		t.Fatalf("exit=%d out=%s", code, out)
	}
}

// SPEC §3.2 边界条件：未配置 key → 退出码 3 并引导 gavel key set。
func TestRunWithoutKeyExits3WithGuidance(t *testing.T) {
	repo := t.TempDir()
	args := []string{"gavel", "run", "--repo", repo, "--test", "go test ./...", "--task", "修好测试"}
	out, code := RunForTest(args, emptySecretProvider{})
	if code != 3 {
		t.Fatalf("exit=%d out=%s", code, out)
	}
	if !contains(out, "key set") {
		t.Fatalf("应引导 gavel key set：%s", out)
	}
}

// SPEC §3.2 边界条件：repo 不存在 → 立即报错退出（3）。
func TestRunRepoMissingExit3(t *testing.T) {
	args := []string{"gavel", "run", "--repo", "Z:\\no\\such\\dir", "--test", "go test ./...", "--task", "修好测试"}
	out, code := RunForTest(args, fakeSecretProvider{})
	if code != 3 {
		t.Fatalf("exit=%d out=%s", code, out)
	}
}

func TestUnknownCommandExit3(t *testing.T) {
	out, code := RunForTest([]string{"gavel", "frobnicate"}, fakeSecretProvider{})
	if code != 3 {
		t.Fatalf("exit=%d out=%s", code, out)
	}
}

func TestVersionExit0(t *testing.T) {
	out, code := RunForTest([]string{"gavel", "version"}, fakeSecretProvider{})
	if code != 0 || !contains(out, "gavel") {
		t.Fatalf("exit=%d out=%s", code, out)
	}
}

// key status 未配置时也应给出信息性输出而非崩溃。
func TestKeyStatusNoKeyExit0(t *testing.T) {
	out, code := RunForTest([]string{"gavel", "key", "status"}, emptySecretProvider{})
	if code != 0 {
		t.Fatalf("exit=%d out=%s", code, out)
	}
	if out == "" {
		t.Fatal("未配置时也应输出提示")
	}
}

// T16：newLLMClient 工厂接入真实 OpenAI 兼容客户端（T14 遗留）。
// httptest 断言 Authorization=key 且 body.model=modelName，验证参数顺序正确。
func TestNewLLMClientFactoryWiredToOpenAI(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer sk-secret-value-1234" {
			t.Errorf("Authorization 应为 Bearer key")
			return
		}
		var body struct {
			Model string `json:"model"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Model != "deepseek-chat" {
			t.Errorf("请求体 model 应为 deepseek-chat")
			return
		}
		json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{{"message": map[string]any{"role": "assistant", "content": "ok"}}},
		})
	}))
	defer srv.Close()
	c := newLLMClient(srv.URL, "deepseek-chat", "sk-secret-value-1234")
	if c == nil {
		t.Fatal("T16 应接入真实客户端")
	}
	comp, err := c.Complete(context.Background(), []llm.Message{{Role: llm.RoleUser, Content: "hi"}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !comp.Done {
		t.Fatalf("无 tool_calls 应 Done：%+v", comp)
	}
}

// T14 Important：自定义 policy 未写 approval_timeout_seconds（或 <=0）时应回退默认值。
func TestWithTimeoutFallbackUsesDefaultWhenUnset(t *testing.T) {
	p := withTimeoutFallback(govern.Policy{ApprovalPatterns: []string{`(?i)rm -rf`}})
	want := govern.DefaultPolicy().ApprovalTimeoutSeconds
	if p.ApprovalTimeoutSeconds != want {
		t.Fatalf("未设置超时应回退默认 %d，got %d", want, p.ApprovalTimeoutSeconds)
	}
	if p.ApprovalTimeoutSeconds <= 0 {
		t.Fatal("回退后超时必须为正")
	}
}

// 显式正整数应原样保留。
func TestWithTimeoutFallbackKeepsPositiveValue(t *testing.T) {
	p := withTimeoutFallback(govern.Policy{ApprovalTimeoutSeconds: 60})
	if p.ApprovalTimeoutSeconds != 60 {
		t.Fatalf("显式 60 应保留，got %d", p.ApprovalTimeoutSeconds)
	}
}

func contains(s, sub string) bool { return strings.Contains(s, sub) }
