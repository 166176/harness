package govern

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestAwaitTimesOutToDeny(t *testing.T) {
	m := NewManager()
	a, err := m.Create("s1", Action{Tool: "run_shell", Args: map[string]any{"command": "rm -rf ."}}, "dangerous", "删除全部")
	if err != nil {
		t.Fatal(err)
	}
	st := m.Await(context.Background(), a.ID, 10*time.Millisecond)
	if st != Timeout {
		t.Fatalf("超时默认应为 timeout，got %s", st)
	}
}

func TestDecideApprove(t *testing.T) {
	m := NewManager()
	a, _ := m.Create("s1", Action{}, "r", "risk")
	go func() {
		time.Sleep(20 * time.Millisecond)
		_ = m.Decide(a.ID, Approved, "webui")
	}()
	st := m.Await(context.Background(), a.ID, time.Second)
	if st != Approved {
		t.Fatalf("got %s", st)
	}
	if ap, _ := m.Pending("s1"); ap != nil {
		t.Fatal("审批后不应再有 pending")
	}
}

func TestSinglePendingPerSession(t *testing.T) {
	m := NewManager()
	_, err1 := m.Create("s1", Action{}, "r", "risk")
	_, err2 := m.Create("s1", Action{}, "r", "risk")
	if err1 != nil || err2 == nil {
		t.Fatal("同会话第二个 pending 应报错")
	}
}

func TestAwaitTimeoutRaceWithConcurrentDecideReturnsRealStatus(t *testing.T) {
	m := NewManager()
	a, err := m.Create("s1", Action{}, "r", "risk")
	if err != nil {
		t.Fatal(err)
	}
	// 模拟窄窗口交错：time.After 已触发、超时分支落地 Timeout 决策前，
	// 外部并发 Decide 先落地 Approved。hook 在超时分支内、落地 Timeout 前执行。
	var hookErr error
	m.BeforeTimeoutDecide = func() {
		hookErr = m.Decide(a.ID, Approved, "concurrent")
	}
	st := m.Await(context.Background(), a.ID, 10*time.Millisecond)
	if hookErr != nil {
		t.Fatalf("hook 内 Decide 失败: %v", hookErr)
	}
	if st != Approved {
		t.Fatalf("Await 返回值应与真实终态一致（approved），got %s", st)
	}
	if got := m.status(a.ID); got != Approved {
		t.Fatalf("真实终态应为 approved，got %s", got)
	}
}

func TestAwaitUnknownIDReturnsTimeout(t *testing.T) {
	m := NewManager()
	st := m.Await(context.Background(), "no-such-id", 10*time.Millisecond)
	if st != Timeout {
		t.Fatalf("未知 id 应返回 timeout（fail-closed），got %s", st)
	}
}

// F1：控制台需要跨会话枚举全部 pending 审批。
func TestPendingAllListsAcrossSessions(t *testing.T) {
	m := NewManager()
	if _, err := m.Create("s1", Action{}, "r", "risk"); err != nil {
		t.Fatal(err)
	}
	if _, err := m.Create("s2", Action{}, "r", "risk"); err != nil {
		t.Fatal(err)
	}
	aps := m.PendingAll()
	if len(aps) != 2 {
		t.Fatalf("PendingAll 应返回 2 条，got %d", len(aps))
	}
}

// F1：decided 事件需要按 id 读取最终状态（含已决审批）。
func TestGetHitAndMiss(t *testing.T) {
	m := NewManager()
	ap, err := m.Create("s1", Action{}, "r", "risk")
	if err != nil {
		t.Fatal(err)
	}
	got, ok := m.Get(ap.ID)
	if !ok || got.ID != ap.ID {
		t.Fatalf("Get 命中失败: ok=%v got=%+v", ok, got)
	}
	if _, ok := m.Get("no-such-id"); ok {
		t.Fatal("Get 未命中应返回 false")
	}
}

// F1：Approval JSON 输出 camelCase，/api/approvals/pending 与 SSE 扁平字段风格一致。
func TestApprovalJSONCamelCase(t *testing.T) {
	ap := &Approval{ID: "a1", SessionID: "s1", Rule: "r", Risk: "risk", Status: Pending, DecidedBy: "u"}
	b, err := json.Marshal(ap)
	if err != nil {
		t.Fatal(err)
	}
	raw := string(b)
	for _, key := range []string{`"id"`, `"sessionId"`, `"decidedBy"`} {
		if !strings.Contains(raw, key) {
			t.Fatalf("缺 camelCase 键 %s: %s", key, raw)
		}
	}
	if strings.Contains(raw, `"SessionID"`) || strings.Contains(raw, `"DecidedBy"`) {
		t.Fatalf("出现 PascalCase 键: %s", raw)
	}
}

func TestAwaitCtxCancelRaceReturnsRealStatus(t *testing.T) {
	m := NewManager()
	a, err := m.Create("s1", Action{}, "r", "risk")
	if err != nil {
		t.Fatal(err)
	}
	// 模拟窄窗口交错：ctx 已取消、取消分支落地 Timeout 决策前，
	// 外部并发 Decide 先落地 Approved。hook 在取消分支内、落地 Timeout 前执行。
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var hookErr error
	m.BeforeCtxCancelDecide = func() {
		hookErr = m.Decide(a.ID, Approved, "concurrent")
	}
	done := make(chan ApprovalStatus, 1)
	go func() { done <- m.Await(ctx, a.ID, time.Minute) }()
	cancel()
	st := <-done
	if hookErr != nil {
		t.Fatalf("hook 内 Decide 失败: %v", hookErr)
	}
	if st != Approved {
		t.Fatalf("Await 返回值应与真实终态一致（approved），got %s", st)
	}
	if got := m.status(a.ID); got != Approved {
		t.Fatalf("真实终态应为 approved，got %s", got)
	}
}
