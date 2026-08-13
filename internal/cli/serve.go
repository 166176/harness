package cli

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"time"

	"github.com/166176/harness/internal/config"
	"github.com/166176/harness/internal/core"
	"github.com/166176/harness/internal/demo"
	"github.com/166176/harness/internal/govern"
	"github.com/166176/harness/internal/llm"
	"github.com/166176/harness/internal/memory"
	"github.com/166176/harness/internal/server"
)

// serveDeps 装配 serve 的依赖（C1 审批链路修复）：HITL 与 Store 在
// 核心 Runner 与 HTTP 服务之间共享同一实例，会话由 POST /api/sessions 启动，
// 审批由 WebUI 决定（ApprovalDecider = HITL.Await）。
// key 缺失时 SessionRunner 仍装配，但返回带引导的错误（端点 500，fail-closed）。
func (a *app) serveDeps(policyPath string) server.Deps {
	hitl := govern.NewManager()
	store := memory.NewStore(filepath.Join(dataDirFn(), "sessions"))

	policy := govern.DefaultPolicy()
	if policyPath != "" {
		p, err := loadCustomPolicy(policyPath)
		if err != nil {
			fmt.Fprintf(a.stderr, "serve: 策略加载失败：%v\n", err)
			policy = govern.DefaultPolicy() // 启动仍继续，审批判定退回默认策略
		} else {
			policy = p
		}
	}

	key, kerr := a.secret.Get()
	var runner server.SessionRunner
	if kerr != nil || key == "" {
		runner = func(context.Context, server.SessionRequest) (string, error) {
			return "", errors.New("未配置 API key：请先运行 `gavel key set`（或提供 .env 中的 GAVEL_API_KEY）")
		}
	} else {
		cfg, cerr := config.Load("")
		if cerr != nil {
			d := config.Default()
			cfg = &d
		}
		client := newLLMClient(cfg.BaseURL, cfg.Model, key)
		if client == nil {
			runner = func(context.Context, server.SessionRequest) (string, error) {
				return "", errors.New("LLM 客户端创建失败（工厂被测试替换为 nil）")
			}
		} else {
			runner = a.sessionRunner(hitl, store, policy, cfg, client, key)
		}
	}

	return server.Deps{
		HITL:          hitl,
		Store:         store,
		Secret:        a.secret,
		SessionRunner: runner,
		DemoFunc: func(ctx context.Context) ([]map[string]any, error) {
			var out []map[string]any
			for _, r := range demo.Run(ctx) {
				out = append(out, map[string]any{"name": r.Name, "pass": r.Pass, "trace": r.Trace})
			}
			return out, nil
		},
	}
}

// sessionRunner 返回创建会话的闭包：生成 id → 建 Session → 起 goroutine 跑 Run，
// 立即返回 id（HTTP 202 与真实会话 id 一致）。
func (a *app) sessionRunner(hitl *govern.Manager, store *memory.Store, policy govern.Policy, cfg *config.Config, client llm.Client, key string) server.SessionRunner {
	decider := func(ctx context.Context, ap *govern.Approval) govern.ApprovalStatus {
		return hitl.Await(ctx, ap.ID, time.Duration(policy.ApprovalTimeoutSeconds)*time.Second)
	}
	return func(_ context.Context, req server.SessionRequest) (string, error) {
		id := newSessionID()
		cr := buildCoreRunner(client, key, policy, hitl, store, cfg, req.Repo, req.Test, decider)
		sess := &core.Session{
			ID:       id,
			Repo:     req.Repo,
			Task:     req.Task,
			TestCmd:  req.Test,
			MaxTurns: cr.MaxTurns,
		}
		go func() {
			// 请求 ctx 会在 handler 返回后取消，会话须独立运行。
			if err := cr.Run(context.Background(), sess); err != nil {
				fmt.Fprintf(a.stderr, "serve: 会话 %s 执行失败：%v\n", id, err)
			}
		}()
		return id, nil
	}
}
