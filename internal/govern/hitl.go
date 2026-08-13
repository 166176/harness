package govern

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

// ApprovalStatus 表示一次人工审批的状态。
type ApprovalStatus string

const (
	Pending  ApprovalStatus = "pending"
	Approved ApprovalStatus = "approved"
	Denied   ApprovalStatus = "denied"
	Timeout  ApprovalStatus = "timeout"
)

// Approval 记录单个会话内一次待人工决断的动作。
type Approval struct {
	ID        string         `json:"id"`
	SessionID string         `json:"sessionId"`
	Action    Action         `json:"action"`
	Rule      string         `json:"rule"`
	Risk      string         `json:"risk"`
	Status    ApprovalStatus `json:"status"`
	DecidedBy string         `json:"decidedBy"`
}

// Manager 管理审批生命周期；同一会话同时最多一个 pending 审批。
type Manager struct {
	mu        sync.Mutex
	byID      map[string]*Approval
	bySession map[string]string // sessionID -> approvalID
	decided   map[string]chan struct{}

	// BeforeTimeoutDecide 是测试钩子：在 Await 超时分支落地 Timeout 决策前被调用（nil 时无操作）。
	BeforeTimeoutDecide func()

	// BeforeCtxCancelDecide 是测试钩子：在 Await 的 ctx 取消分支落地 Timeout 决策前被调用（nil 时无操作）。
	BeforeCtxCancelDecide func()
}

func NewManager() *Manager {
	return &Manager{byID: map[string]*Approval{}, bySession: map[string]string{}, decided: map[string]chan struct{}{}}
}

func (m *Manager) Create(sessionID string, a Action, rule, risk string) (*Approval, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if id, ok := m.bySession[sessionID]; ok {
		if m.byID[id].Status == Pending {
			return nil, errors.New("session already has pending approval")
		}
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
		if m.BeforeTimeoutDecide != nil {
			m.BeforeTimeoutDecide()
		}
		if err := m.Decide(id, Timeout, "timeout"); err != nil {
			return Timeout // fail-closed
		}
		return m.status(id)
	case <-ctx.Done():
		if m.BeforeCtxCancelDecide != nil {
			m.BeforeCtxCancelDecide()
		}
		if err := m.Decide(id, Timeout, "cancel"); err != nil {
			return Timeout // fail-closed
		}
		return m.status(id)
	}
}

func (m *Manager) Decide(id string, status ApprovalStatus, by string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	ap, ok := m.byID[id]
	if !ok {
		return errors.New("unknown approval")
	}
	if ap.Status != Pending {
		return nil
	}
	if status != Approved && status != Denied && status != Timeout {
		return errors.New("bad status")
	}
	ap.Status = status
	ap.DecidedBy = by
	close(m.decided[id])
	return nil
}

func (m *Manager) Pending(sessionID string) (*Approval, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	id, ok := m.bySession[sessionID]
	if !ok {
		return nil, false
	}
	ap := m.byID[id]
	if ap.Status != Pending {
		return nil, false
	}
	return ap, true
}

// PendingAll 返回全部 pending 审批（跨会话），供审批控制台枚举。
func (m *Manager) PendingAll() []*Approval {
	m.mu.Lock()
	defer m.mu.Unlock()
	aps := make([]*Approval, 0, len(m.byID))
	for _, ap := range m.byID {
		if ap.Status == Pending {
			aps = append(aps, ap)
		}
	}
	return aps
}

// Get 按 id 返回审批（含已决），未命中返回 false。
func (m *Manager) Get(id string) (*Approval, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	ap, ok := m.byID[id]
	return ap, ok
}

func (m *Manager) ch(id string) chan struct{} {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.decided[id]
}

func (m *Manager) status(id string) ApprovalStatus {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.byID[id].Status
}

// idSeq 保证同一时刻连续创建（Windows time.Now 精度较粗）时 ID 仍唯一。
var idSeq atomic.Uint64

func newID() string {
	return fmt.Sprintf("%s-%04d", time.Now().Format("20060102150405.000000000"), idSeq.Add(1))
}
