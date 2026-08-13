package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/166176/harness/internal/govern"
	"github.com/166176/harness/internal/secret"
)

// writeJSON 以统一格式输出 JSON 响应。
func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

// handleSessions 返回存储中全部会话 id 列表。
func (s *srv) handleSessions(w http.ResponseWriter, r *http.Request) {
	if s.deps.Store == nil {
		writeJSON(w, http.StatusOK, []string{})
		return
	}
	keys, err := s.deps.Store.List("session:")
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	ids := make([]string, 0, len(keys))
	for _, k := range keys {
		ids = append(ids, strings.TrimPrefix(k, "session:"))
	}
	writeJSON(w, http.StatusOK, ids)
}

// handleSession 返回单个会话的 JSON。
func (s *srv) handleSession(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if s.deps.Store == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "session not found"})
		return
	}
	var sess map[string]any
	ok, err := s.deps.Store.Get("session:"+id, &sess)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "session not found"})
		return
	}
	writeJSON(w, http.StatusOK, sess)
}

// handlePendingApprovals 返回当前所有 pending 审批。
func (s *srv) handlePendingApprovals(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"approvals": s.pendingApprovals()})
}

// handleDecide 处理 POST /api/approvals/{id}：body {"decision":"approved|denied"}。
func (s *srv) handleDecide(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var body struct {
		Decision string `json:"decision"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "body 不是合法 JSON"})
		return
	}
	var st govern.ApprovalStatus
	switch body.Decision {
	case "approved":
		st = govern.Approved
	case "denied":
		st = govern.Denied
	default:
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "decision 必须为 approved|denied"})
		return
	}
	if s.deps.HITL == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "hitl not configured"})
		return
	}
	if err := s.deps.HITL.Decide(id, st, "webui"); err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
		return
	}
	s.hub.broadcast(decidedEvent(id, body.Decision))
	writeJSON(w, http.StatusOK, map[string]string{"id": id, "status": string(st)})
}

// handleKeyStatus 只回 provider/mask/fingerprint，绝不回显明文。
func (s *srv) handleKeyStatus(w http.ResponseWriter, r *http.Request) {
	resp := map[string]string{"provider": "", "mask": "", "fingerprint": ""}
	if s.deps.Secret != nil {
		resp["provider"] = s.deps.Secret.Name()
		if v, err := s.deps.Secret.Get(); err == nil {
			resp["mask"] = secret.Mask(v)
			resp["fingerprint"] = secret.Fingerprint(v)
		}
	}
	writeJSON(w, http.StatusOK, resp)
}

// handleDemo 执行 DemoFunc 返回三场景结果；未装配时 501。
func (s *srv) handleDemo(w http.ResponseWriter, r *http.Request) {
	if s.deps.DemoFunc == nil {
		writeJSON(w, http.StatusNotImplemented, map[string]string{"error": "demo not available（T13 未装配 DemoFunc）"})
		return
	}
	rs, err := s.deps.DemoFunc(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if rs == nil {
		rs = []map[string]any{}
	}
	writeJSON(w, http.StatusOK, rs)
}

// pendingApprovals 通过 store 会话键枚举 manager 中 pending 审批。
func (s *srv) pendingApprovals() []*govern.Approval {
	aps := []*govern.Approval{}
	if s.deps.HITL == nil || s.deps.Store == nil {
		return aps
	}
	keys, err := s.deps.Store.List("session:")
	if err != nil {
		return aps
	}
	for _, k := range keys {
		if ap, ok := s.deps.HITL.Pending(strings.TrimPrefix(k, "session:")); ok {
			aps = append(aps, ap)
		}
	}
	return aps
}

// handleEvents 建立 SSE 连接：初始 pending 快照 + 每秒轮询增量 + decided 广播。
func (s *srv) handleEvents(w http.ResponseWriter, r *http.Request) {
	fl, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}
	hdr := w.Header()
	hdr.Set("Content-Type", "text/event-stream")
	hdr.Set("Cache-Control", "no-cache")
	hdr.Set("Connection", "keep-alive")
	hdr.Set("X-Accel-Buffering", "no")

	seen := map[string]bool{}
	for _, ap := range s.pendingApprovals() {
		seen[ap.ID] = true
		writeSSE(w, fl, approvalEvent(ap))
	}
	ch, cancel := s.hub.subscribe()
	defer cancel()

	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case <-ticker.C:
			cur := s.pendingApprovals()
			curSet := map[string]bool{}
			for _, ap := range cur {
				curSet[ap.ID] = true
				if !seen[ap.ID] {
					writeSSE(w, fl, approvalEvent(ap))
				}
			}
			for id := range seen {
				if !curSet[id] {
					writeSSE(w, fl, decidedEvent(id, ""))
				}
			}
			seen = curSet
			fmt.Fprint(w, ": ping\n\n")
			fl.Flush()
		case p := <-ch:
			fmt.Fprintf(w, "data: %s\n\n", p)
			fl.Flush()
		}
	}
}

func writeSSE(w http.ResponseWriter, fl http.Flusher, payload []byte) {
	fmt.Fprintf(w, "data: %s\n\n", payload)
	fl.Flush()
}

func approvalEvent(ap *govern.Approval) []byte {
	b, _ := json.Marshal(map[string]any{
		"type":    "pending",
		"id":      ap.ID,
		"session": ap.SessionID,
		"action":  ap.Action,
		"rule":    ap.Rule,
		"risk":    ap.Risk,
		"status":  string(ap.Status),
	})
	return b
}

func decidedEvent(id, decision string) []byte {
	m := map[string]any{"type": "decided", "id": id}
	if decision != "" {
		m["decision"] = decision
	}
	b, _ := json.Marshal(m)
	return b
}

// eventHub 是进程内 SSE 广播器：POST 决定后即时推送 decided 事件。
type eventHub struct {
	mu   sync.Mutex
	subs map[chan []byte]struct{}
}

func newEventHub() *eventHub {
	return &eventHub{subs: map[chan []byte]struct{}{}}
}

func (h *eventHub) subscribe() (chan []byte, func()) {
	h.mu.Lock()
	defer h.mu.Unlock()
	ch := make(chan []byte, 16)
	h.subs[ch] = struct{}{}
	return ch, func() {
		h.mu.Lock()
		defer h.mu.Unlock()
		if _, ok := h.subs[ch]; ok {
			delete(h.subs, ch)
			close(ch)
		}
	}
}

func (h *eventHub) broadcast(p []byte) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for ch := range h.subs {
		select {
		case ch <- p:
		default: // 慢消费者丢弃该事件，不阻塞请求
		}
	}
}
