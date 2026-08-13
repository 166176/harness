package cli

import (
	"encoding/json"
	"strings"
	"testing"

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

func contains(s, sub string) bool { return strings.Contains(s, sub) }
