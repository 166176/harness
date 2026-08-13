package govern

import (
	"context"
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
