// Package server 提供 gavel 的 HTTP 服务：REST 审批接口、SSE 事件流、
// 密钥掩码状态查询与 webui 静态资源内嵌（SPEC §A.8 WebUI 部署）。
package server

import (
	"context"
	"embed"
	"io/fs"
	"net/http"

	"github.com/166176/harness/internal/govern"
	"github.com/166176/harness/internal/memory"
	"github.com/166176/harness/internal/secret"
)

//go:embed all:webui/dist
var distFS embed.FS

// Deps 是 server 的装配依赖。
type Deps struct {
	HITL   *govern.Manager
	Store  *memory.Store
	Secret secret.Provider
	// DemoFunc 承接 T13 的 demo.Run 三场景（本任务用函数类型，避免跨包依赖）。
	DemoFunc func(ctx context.Context) ([]map[string]any, error)
}

type srv struct {
	deps     Deps
	hub      *eventHub
	static   http.Handler
	staticOK bool
}

// New 装配全部路由并返回 http.Handler。
func New(d Deps) http.Handler {
	s := &srv{deps: d, hub: newEventHub()}
	sub, err := fs.Sub(distFS, "webui/dist")
	if err != nil {
		sub = distFS
	}
	s.static = http.FileServerFS(sub)
	s.staticOK = indexExists(sub)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/sessions", s.handleSessions)
	mux.HandleFunc("GET /api/sessions/{id}", s.handleSession)
	mux.HandleFunc("GET /api/approvals/pending", s.handlePendingApprovals)
	mux.HandleFunc("POST /api/approvals/{id}", s.handleDecide)
	mux.HandleFunc("GET /api/events", s.handleEvents)
	mux.HandleFunc("GET /api/key/status", s.handleKeyStatus)
	mux.HandleFunc("GET /api/demo", s.handleDemo)
	mux.HandleFunc("GET /", s.handleStatic)
	return mux
}

func indexExists(f fs.FS) bool {
	_, err := fs.Stat(f, "index.html")
	return err == nil
}

// handleStatic 提供内嵌 webui/dist 静态文件；产物缺失时 503 提示先构建前端。
func (s *srv) handleStatic(w http.ResponseWriter, r *http.Request) {
	if !s.staticOK {
		http.Error(w, "webui 未构建：请先构建前端产物（T15 生成 webui/dist）", http.StatusServiceUnavailable)
		return
	}
	s.static.ServeHTTP(w, r)
}
