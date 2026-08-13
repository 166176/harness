// Package cli 装配 gavel 命令行：version / key / demo / serve / run 子命令，
// 退出码按 SPEC §3.2：0=全绿 1=预算耗尽 2=人工终止 3=配置错误。
package cli

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/166176/harness/internal/config"
	"github.com/166176/harness/internal/core"
	"github.com/166176/harness/internal/demo"
	"github.com/166176/harness/internal/govern"
	"github.com/166176/harness/internal/llm"
	"github.com/166176/harness/internal/memory"
	"github.com/166176/harness/internal/secret"
	"github.com/166176/harness/internal/server"
	"github.com/166176/harness/internal/tools"
	"github.com/166176/harness/internal/version"
)

// 退出码（SPEC §3.2）。
const (
	ExitOK     = 0 // 全绿
	ExitBudget = 1 // 预算耗尽
	ExitHITL   = 2 // 人工终止
	ExitConfig = 3 // 配置错误
)

// app 是一次 CLI 调用的上下文；provider 全部可注入（测试用 RunForTest）。
type app struct {
	stdin   io.Reader
	stdout  io.Writer
	stderr  io.Writer
	keyring secret.Provider // key set/clear 目标（SPEC §3.2：写入 keyring）
	secret  secret.Provider // 运行期链式读取（keyring → .env）
	dotenv  secret.Provider // key status 定位存储位置
}

// newLLMClient 为可注入工厂：默认接入真实 OpenAI 兼容客户端（T16，llm.NewOpenAIClient）。
// 测试可整体替换该变量，避免真实联网。
var newLLMClient = func(baseURL, model, apiKey string) llm.Client {
	return llm.NewOpenAIClient(baseURL, apiKey, model)
}

// Run 是生产入口：main.go 以 os.Args[1:]、os.Stdin、os.Stdout、os.Stderr 调用。
func Run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	kr := secret.KeyringProvider()
	denv := secret.DotenvProvider(".env")
	a := &app{
		stdin:   stdin,
		stdout:  stdout,
		stderr:  stderr,
		keyring: kr,
		secret:  secret.Chain(kr, denv),
		dotenv:  denv,
	}
	return a.run(normalizeArgs(args))
}

// RunForTest 是测试入口：注入 fake secret provider，stdout+stderr 合并捕获到 string。
func RunForTest(args []string, prov secret.Provider) (string, int) {
	var buf bytes.Buffer
	a := &app{
		stdin:   strings.NewReader(""),
		stdout:  &buf,
		stderr:  &buf,
		keyring: prov,
		secret:  prov,
		dotenv:  prov,
	}
	code := a.run(normalizeArgs(args))
	return buf.String(), code
}

// normalizeArgs 剥离可能的程序名（RunForTest 按 brief 传入 args[0]=="gavel"）。
func normalizeArgs(args []string) []string {
	if len(args) > 0 && (args[0] == "gavel" || args[0] == "gavel.exe") {
		return args[1:]
	}
	return args
}

func (a *app) run(args []string) int {
	if len(args) == 0 {
		a.usage(a.stderr)
		return ExitConfig
	}
	switch args[0] {
	case "version", "-v", "--version":
		return a.cmdVersion()
	case "key":
		return a.cmdKey(args[1:])
	case "demo":
		return a.cmdDemo(args[1:])
	case "serve":
		return a.cmdServe(args[1:])
	case "run":
		return a.cmdRun(args[1:])
	case "help", "-h", "--help":
		a.usage(a.stdout)
		return ExitOK
	default:
		fmt.Fprintf(a.stderr, "未知子命令 %q\n\n", args[0])
		a.usage(a.stderr)
		return ExitConfig
	}
}

const usageText = `gavel：AI 修复 agent（SPEC §3.2）

用法：
  gavel run     --repo <路径> --test "<测试命令>" --task "<任务>" [--model] [--max-turns N] [--timeout 秒] [--policy <yaml>] [--config <yaml>]
  gavel demo    [--json]
  gavel serve   [--port 8080] [--host 地址]
  gavel key     set | status | clear
  gavel version

退出码：0=全绿 1=预算耗尽 2=人工终止 3=配置错误
`

func (a *app) usage(w io.Writer) {
	fmt.Fprint(w, usageText)
}

func (a *app) cmdVersion() int {
	fmt.Fprintf(a.stdout, "gavel %s\n", version.Version)
	return ExitOK
}

// cmdDemo 用纯 MockLLM 执行三场景机制演示（SPEC §3.2）：不联网、不读凭据。
// 任一场景 FAIL → 退出码非 0（此处取 1）。
func (a *app) cmdDemo(args []string) int {
	fs := flag.NewFlagSet("demo", flag.ContinueOnError)
	fs.SetOutput(a.stderr)
	jsonOut := fs.Bool("json", false, "输出机器可读 JSON")
	if err := fs.Parse(args); err != nil {
		return ExitConfig
	}
	results := demo.Run(context.Background())
	if *jsonOut {
		b, err := json.Marshal(results)
		if err != nil {
			fmt.Fprintf(a.stderr, "demo: JSON 序列化失败：%v\n", err)
			return ExitConfig
		}
		fmt.Fprintln(a.stdout, string(b))
	} else {
		for _, r := range results {
			status := "PASS"
			if !r.Pass {
				status = "FAIL"
			}
			fmt.Fprintf(a.stdout, "%s %s\n", status, r.Name)
			for _, tr := range r.Trace {
				fmt.Fprintf(a.stdout, "  - %s\n", tr)
			}
		}
	}
	for _, r := range results {
		if !r.Pass {
			return ExitBudget
		}
	}
	return ExitOK
}

// cmdServe 启动 REST+SSE 服务并托管内嵌 WebUI（SPEC §3.2）。
// 云端模式（Render 等）可用 $PORT 注入监听端口；key 由 .env 提供。
func (a *app) cmdServe(args []string) int {
	fs := flag.NewFlagSet("serve", flag.ContinueOnError)
	fs.SetOutput(a.stderr)
	port := fs.Int("port", 0, "监听端口（默认 8080，或 $PORT）")
	host := fs.String("host", "", "监听地址（默认全接口）")
	policyPath := fs.String("policy", "", "护栏策略 YAML（可选，覆盖默认策略）")
	if err := fs.Parse(args); err != nil {
		return ExitConfig
	}
	p := *port
	if p == 0 {
		if env := os.Getenv("PORT"); env != "" {
			if n, err := strconv.Atoi(env); err == nil && n > 0 {
				p = n
			}
		}
		if p == 0 {
			p = 8080
		}
	}
	h := server.New(a.serveDeps(*policyPath))
	addr := net.JoinHostPort(*host, strconv.Itoa(p))
	fmt.Fprintf(a.stdout, "gavel serve 监听 http://%s\n", addr)
	if err := http.ListenAndServe(addr, h); err != nil {
		fmt.Fprintf(a.stderr, "serve: %v（端口可能被占用）\n", err)
		return ExitConfig
	}
	return ExitOK
}

// cmdRun 执行一次修复会话（SPEC §3.2）：
// flag → 仓库存在性 → config.Load → key 检查 → policy 覆盖 → 组装 Runner → 主循环。
func (a *app) cmdRun(args []string) int {
	fs := flag.NewFlagSet("run", flag.ContinueOnError)
	fs.SetOutput(a.stderr)
	repo := fs.String("repo", "", "目标仓库路径（必填）")
	testCmd := fs.String("test", "", "测试命令，如 go test ./...（必填）")
	task := fs.String("task", "", "自然语言修复任务（必填）")
	model := fs.String("model", "", "模型名（覆盖配置）")
	maxTurns := fs.Int("max-turns", 0, "最大轮数（默认取配置或 20）")
	timeoutSec := fs.Int("timeout", 0, "会话总时长预算（秒，默认取配置）")
	policyPath := fs.String("policy", "", "护栏策略 YAML（可选，覆盖默认策略）")
	configPath := fs.String("config", "", "配置文件路径（可选）")
	if err := fs.Parse(args); err != nil {
		return ExitConfig
	}
	if *repo == "" || *testCmd == "" || *task == "" {
		fmt.Fprintln(a.stderr, "run: --repo/--test/--task 为必填")
		return ExitConfig
	}
	if fi, err := os.Stat(*repo); err != nil || !fi.IsDir() {
		fmt.Fprintf(a.stderr, "run: 仓库路径不可用：%s\n", *repo)
		return ExitConfig
	}
	cfg, err := config.Load(*configPath)
	if err != nil {
		fmt.Fprintf(a.stderr, "run: 配置加载失败：%v\n", err)
		return ExitConfig
	}
	key, err := a.secret.Get()
	if err != nil {
		fmt.Fprintln(a.stderr, "run: 未配置 API key，请先运行 `gavel key set`（或提供 .env 中的 GAVEL_API_KEY）")
		return ExitConfig
	}
	modelName := cfg.Model
	if *model != "" {
		modelName = *model
	}
	maxT := cfg.MaxTurns
	if *maxTurns > 0 {
		maxT = *maxTurns
	}
	budget := time.Duration(cfg.TimeoutSec) * time.Second
	if *timeoutSec > 0 {
		budget = time.Duration(*timeoutSec) * time.Second
	}
	policy := govern.DefaultPolicy()
	if *policyPath != "" {
		p, perr := loadCustomPolicy(*policyPath)
		if perr != nil {
			fmt.Fprintf(a.stderr, "run: %v\n", perr)
			return ExitConfig
		}
		policy = p
	}
	// newLLMClient 默认接入真实适配器（llm.NewOpenAIClient）；仅在测试替换为 nil 时停于此。
	client := newLLMClient(cfg.BaseURL, modelName, key)
	if client == nil {
		fmt.Fprintln(a.stderr, "run: LLM 客户端创建失败（工厂被测试替换为 nil）")
		return ExitConfig
	}
	hitl := govern.NewManager()
	store := memory.NewStore(filepath.Join(dataDirFn(), "sessions"))
	cfg.MaxTurns = maxT
	cfg.TimeoutSec = int(budget / time.Second)
	runner := buildCoreRunner(client, key, policy, hitl, store, cfg, *repo, *testCmd,
		a.approvalDecider(hitl, time.Duration(policy.ApprovalTimeoutSeconds)*time.Second))
	sess := &core.Session{
		ID:       newSessionID(),
		Repo:     *repo,
		Task:     *task,
		TestCmd:  *testCmd,
		MaxTurns: maxT,
	}
	if err := runner.Run(context.Background(), sess); err != nil {
		fmt.Fprintf(a.stderr, "run: 会话执行失败：%v\n", err)
		return ExitBudget
	}
	if sess.State == core.StateCompleted {
		saveProjectHint(store, *repo, *task, *testCmd) // §3.5 项目约定库：成功后沉淀跨会话记忆
	}
	printRunSummary(a.stdout, sess)
	switch sess.State {
	case core.StateCompleted:
		return ExitOK
	case core.StateTerminated:
		return ExitHITL
	default:
		return ExitBudget
	}
}

// withTimeoutFallback 对自定义策略做缺省兜底（T14 Important）：
// ApprovalTimeoutSeconds<=0 时回退为默认策略的值，避免审批超时配置缺失导致行为异常。
func withTimeoutFallback(p govern.Policy) govern.Policy {
	if p.ApprovalTimeoutSeconds <= 0 {
		p.ApprovalTimeoutSeconds = govern.DefaultPolicy().ApprovalTimeoutSeconds
	}
	return p
}

// assembleTools 组装会话工具集：文件四件套 + run_shell + run_test（真实执行器）。
func assembleTools(repo, testCmd string) []tools.Tool {
	var ts []tools.Tool
	ts = append(ts, tools.FileTools(repo)...)
	ts = append(ts,
		tools.ShellTool(tools.RealRunner{}, repo),
		tools.TestTool(tools.RealRunner{}, repo, testCmd),
	)
	return ts
}

// saveProjectHint 在会话成功后把任务与测试命令沉淀为项目约定（§3.5 记忆维度），
// 下次同仓库会话经 core.Runner.Hint 按需装配进 system 提示。
func saveProjectHint(store *memory.Store, repo, task, testCmd string) {
	if store == nil {
		return
	}
	abs, err := filepath.Abs(repo)
	if err != nil {
		abs = repo
	}
	sum := sha256.Sum256([]byte(abs))
	key := "hint:" + hex.EncodeToString(sum[:])[:16]
	_ = store.Put(key, map[string]string{"task": task, "test": testCmd})
}

// printRunSummary 输出会话 id + 终态 + 摘要（SPEC §3.2）。
func printRunSummary(w io.Writer, sess *core.Session) {
	fmt.Fprintf(w, "session: %s\n", sess.ID)
	fmt.Fprintf(w, "state: %s\n", sess.State)
	fmt.Fprintf(w, "steps: %d\n", len(sess.Steps))
}

// dataDirFn 可注入（测试替换为临时目录）；生产默认 ~/.gavel（SPEC §3.4）。
var dataDirFn = func() string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		home = "."
	}
	return filepath.Join(home, ".gavel")
}

func newSessionID() string {
	return strconv.FormatInt(time.Now().UnixNano(), 36)
}
