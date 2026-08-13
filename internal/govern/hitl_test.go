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
