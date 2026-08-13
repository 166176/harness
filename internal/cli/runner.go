package cli

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/166176/harness/internal/config"
	"github.com/166176/harness/internal/core"
	"github.com/166176/harness/internal/govern"
	"github.com/166176/harness/internal/llm"
	"github.com/166176/harness/internal/memory"
	"github.com/166176/harness/internal/tools"
	"gopkg.in/yaml.v3"
)

// buildCoreRunner 构造治理装配完备的核心 Runner（C1）：
// Guard 闭包注入 SecretKey（泄露比对），HITL/Store 由调用方传入（serve 共享 / run 独立），
// ApprovalDecider 由调用方决定（CLI y/n 或 WebUI Await）。
func buildCoreRunner(client llm.Client, key string, policy govern.Policy, hitl *govern.Manager, store *memory.Store, cfg *config.Config, repo, testCmd string, decider func(context.Context, *govern.Approval) govern.ApprovalStatus) *core.Runner {
	timeout := time.Duration(policy.ApprovalTimeoutSeconds) * time.Second
	hint := ""
	if store != nil {
		abs, err := filepath.Abs(repo)
		if err != nil {
			abs = repo
		}
		sum := sha256.Sum256([]byte(abs))
		var h map[string]string
		if ok, _ := store.Get("hint:"+hex.EncodeToString(sum[:])[:16], &h); ok {
			hint = fmt.Sprintf("任务：%s\n测试命令：%s", h["task"], h["test"])
		}
	}
	return &core.Runner{
		LLM:   client,
		Tools: tools.RegistryOf(assembleTools(repo, testCmd)),
		Guard: func(gc govern.GuardContext, act govern.Action) govern.Verdict {
			gc.SecretKey = key // 泄露比对需要已存 key（core.Runner 只填 RepoRoot）
			return govern.Check(policy, gc, act)
		},
		HITL:            hitl,
		Policy:          policy,
		ApprovalTimeout: timeout,
		MaxTurns:        cfg.MaxTurns,
		TimeBudget:      time.Duration(cfg.TimeoutSec) * time.Second,
		Store:           store,
		ApprovalDecider: decider,
		Hint:            hint,
	}
}

// loadCustomPolicy 读取并解析自定义策略 YAML（I2 增加正则校验，见 Policy.Validate）。
// 读取失败/解析失败均报错（fail-closed），调用方按配置错误处理。
func loadCustomPolicy(path string) (govern.Policy, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return govern.Policy{}, fmt.Errorf("策略文件读取失败：%w", err)
	}
	var p govern.Policy
	if err := yaml.Unmarshal(b, &p); err != nil {
		return govern.Policy{}, fmt.Errorf("策略解析失败：%w", err)
	}
	p = withTimeoutFallback(p)
	if err := p.Validate(); err != nil {
		return govern.Policy{}, fmt.Errorf("策略正则非法（fail-closed）：%w", err)
	}
	return p, nil
}
