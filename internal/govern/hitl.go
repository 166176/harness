package govern

import (
	"context"
	"errors"
	"sync"
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
	ID, SessionID string
	Action        Action
	Rule, Risk    string
	Status        ApprovalStatus
	DecidedBy     string
}

// Manager 管理审批生命周期；同一会话同时最多一个 pending 审批。
type Manager struct {
	mu        sync.Mutex
	byID      map[string]*Approval
	bySession map[string]string // sessionID -> approvalID
	decided   map[string]chan struct{}

	// BeforeTimeoutDecide 是测试钩子：在 Await 超时分支落地 Timeout 决策前被调用（nil 时无操作）。
	BeforeTimeoutDecide func()
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
		_ = m.Decide(id, Timeout, "timeout") // fail-closed
		return m.status(id)
	case <-ctx.Done():
		_ = m.Decide(id, Timeout, "cancel")
		return Timeout
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

func newID() string {
	return time.Now().Format("20060102150405.000000000")
}
