package cli

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRunBadPolicyRegexExit3(t *testing.T) {
	dir := t.TempDir()
	policyFile := filepath.Join(dir, "bad.yaml")
	if err := os.WriteFile(policyFile, []byte("approval_patterns:\n  - '([a-z'\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// 先给一个合法 key 与 repo，让流程走到策略加载
	_, code := RunForTest([]string{"gavel", "run", "--repo", dir, "--test", "go test ./...", "--task", "t", "--policy", policyFile}, fakeSecretProvider{})
	if code != 3 {
		t.Fatalf("坏正则策略应退出 3（配置错误），got %d", code)
	}
}
