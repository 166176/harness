package server

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/166176/harness/internal/govern"
	"github.com/166176/harness/internal/memory"
	"github.com/166176/harness/internal/secret"
)

// memSecret 是测试用内存凭据 provider（brief 自带，照用）。
type memSecret struct{ v string }

func (m *memSecret) Name() string         { return "mem" }
func (m *memSecret) Get() (string, error) { return m.v, nil }
func (m *memSecret) Set(k string) error {
	m.v = k
	return nil
}
func (m *memSecret) Clear() error {
	m.v = ""
	return nil
}

var _ secret.Provider = (*memSecret)(nil)

func TestKeyStatusMasksSecret(t *testing.T) {
	h := New(Deps{HITL: govern.NewManager(), Store: memory.NewStore(t.TempDir()), Secret: &memSecret{v: "sk-abcdefghij1234"}})
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest("GET", "/api/key/status", nil))
	if rr.Code != 200 {
		t.Fatalf("code=%d", rr.Code)
	}
	var body map[string]string
	_ = json.Unmarshal(rr.Body.Bytes(), &body)
	if body["secret"] == "sk-abcdefghij1234" {
		t.Fatal("接口泄露明文")
	}
	if body["mask"] == "" {
		t.Fatal("掩码为空")
	}
}

func TestDecideApproval(t *testing.T) {
	m := govern.NewManager()
	ap, err := m.Create("s1", govern.Action{Tool: "run_shell", Args: map[string]any{"command": "rm -rf ."}}, "dangerous", "risk")
	if err != nil {
		t.Fatal(err)
	}
	h := New(Deps{HITL: m, Store: memory.NewStore(t.TempDir()), Secret: &memSecret{}})
	body := bytes.NewBufferString(`{"decision":"approved"}`)
	req := httptest.NewRequest("POST", "/api/approvals/"+ap.ID, body)
	req.SetPathValue("id", ap.ID)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != 200 {
		t.Fatalf("code=%d body=%s", rr.Code, rr.Body.String())
	}
	st := m.Await(t.Context(), ap.ID, 10*time.Millisecond)
	if st != govern.Approved {
		t.Fatalf("应为 approved，got %s", st)
	}
}

func TestSessionsListAndGet(t *testing.T) {
	st := memory.NewStore(t.TempDir())
	_ = st.Put("session:s1", map[string]any{"ID": "s1", "State": "completed"})
	h := New(Deps{HITL: govern.NewManager(), Store: st, Secret: &memSecret{}})

	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/api/sessions", nil))
	if rr.Code != http.StatusOK {
		t.Fatalf("code=%d", rr.Code)
	}
	var ids []string
	if err := json.Unmarshal(rr.Body.Bytes(), &ids); err != nil {
		t.Fatal(err)
	}
	if len(ids) != 1 || ids[0] != "s1" {
		t.Fatalf("ids=%v", ids)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/sessions/s1", nil)
	req.SetPathValue("id", "s1")
	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("code=%d body=%s", rr.Code, rr.Body.String())
	}
	var sess map[string]any
	_ = json.Unmarshal(rr.Body.Bytes(), &sess)
	if sess["ID"] != "s1" {
		t.Fatalf("sess=%v", sess)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/sessions/missing", nil)
	req.SetPathValue("id", "missing")
	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("code=%d", rr.Code)
	}
}

func TestPendingApprovalsList(t *testing.T) {
	st := memory.NewStore(t.TempDir())
	_ = st.Put("session:s1", map[string]any{"ID": "s1"})
	m := govern.NewManager()
	ap, err := m.Create("s1", govern.Action{Tool: "run_shell", Args: map[string]any{"command": "rm -rf ."}}, "dangerous", "risk")
	if err != nil {
		t.Fatal(err)
	}
	h := New(Deps{HITL: m, Store: st, Secret: &memSecret{}})

	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/api/approvals/pending", nil))
	if rr.Code != http.StatusOK {
		t.Fatalf("code=%d", rr.Code)
	}
	var body struct {
		Approvals []govern.Approval `json:"approvals"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if len(body.Approvals) != 1 || body.Approvals[0].ID != ap.ID || body.Approvals[0].Status != govern.Pending {
		t.Fatalf("body=%s", rr.Body.String())
	}

	if err := m.Decide(ap.ID, govern.Approved, "test"); err != nil {
		t.Fatal(err)
	}
	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/api/approvals/pending", nil))
	if rr.Code != http.StatusOK {
		t.Fatalf("code=%d", rr.Code)
	}
	_ = json.Unmarshal(rr.Body.Bytes(), &body)
	if len(body.Approvals) != 0 {
		t.Fatalf("决策后 pending 应为空，body=%s", rr.Body.String())
	}
}

func TestDecideDeniedAndBadInput(t *testing.T) {
	m := govern.NewManager()
	ap, err := m.Create("s2", govern.Action{Tool: "write_file", Args: map[string]any{"path": "x.go"}}, "r", "risk")
	if err != nil {
		t.Fatal(err)
	}
	h := New(Deps{HITL: m, Store: memory.NewStore(t.TempDir()), Secret: &memSecret{}})

	req := httptest.NewRequest(http.MethodPost, "/api/approvals/"+ap.ID, bytes.NewBufferString(`{"decision":"denied"}`))
	req.SetPathValue("id", ap.ID)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("code=%d body=%s", rr.Code, rr.Body.String())
	}
	if st := m.Await(t.Context(), ap.ID, 10*time.Millisecond); st != govern.Denied {
		t.Fatalf("应为 denied，got %s", st)
	}

	req = httptest.NewRequest(http.MethodPost, "/api/approvals/"+ap.ID, bytes.NewBufferString(`{"decision":"maybe"}`))
	req.SetPathValue("id", ap.ID)
	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("非法 decision 应 400，code=%d", rr.Code)
	}

	req = httptest.NewRequest(http.MethodPost, "/api/approvals/nope", bytes.NewBufferString(`{"decision":"approved"}`))
	req.SetPathValue("id", "nope")
	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("未知审批应 404，code=%d", rr.Code)
	}
}

func TestEventsStreamsSSE(t *testing.T) {
	st := memory.NewStore(t.TempDir())
	_ = st.Put("session:s3", map[string]any{"ID": "s3"})
	m := govern.NewManager()
	ap, err := m.Create("s3", govern.Action{Tool: "run_shell", Args: map[string]any{"command": "make"}}, "dangerous", "risk")
	if err != nil {
		t.Fatal(err)
	}
	h := New(Deps{HITL: m, Store: st, Secret: &memSecret{}})
	ts := httptest.NewServer(h)
	defer ts.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, ts.URL+"/api/events", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if ct := resp.Header.Get("Content-Type"); ct != "text/event-stream" {
		t.Fatalf("Content-Type=%s", ct)
	}
	if xa := resp.Header.Get("X-Accel-Buffering"); xa != "no" {
		t.Fatalf("X-Accel-Buffering=%s", xa)
	}

	sc := bufio.NewScanner(resp.Body)
	for sc.Scan() {
		line := sc.Text()
		if strings.HasPrefix(line, "data: ") && strings.Contains(line, `"type":"pending"`) && strings.Contains(line, ap.ID) {
			return // 收到初始 pending 快照事件
		}
	}
	if sc.Err() != nil {
		t.Fatal(sc.Err())
	}
	t.Fatal("未收到 pending SSE 事件")
}

func TestEventsDecidedBroadcast(t *testing.T) {
	st := memory.NewStore(t.TempDir())
	_ = st.Put("session:s4", map[string]any{"ID": "s4"})
	m := govern.NewManager()
	ap, err := m.Create("s4", govern.Action{Tool: "run_shell", Args: map[string]any{"command": "make"}}, "dangerous", "risk")
	if err != nil {
		t.Fatal(err)
	}
	h := New(Deps{HITL: m, Store: st, Secret: &memSecret{}})
	ts := httptest.NewServer(h)
	defer ts.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, ts.URL+"/api/events", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	postDone := make(chan error, 1)
	go func() {
		body := bytes.NewBufferString(`{"decision":"approved"}`)
		preq, _ := http.NewRequest(http.MethodPost, ts.URL+"/api/approvals/"+ap.ID, body)
		res, err := http.DefaultClient.Do(preq)
		if err == nil {
			res.Body.Close()
		}
		postDone <- err
	}()

	sc := bufio.NewScanner(resp.Body)
	for sc.Scan() {
		line := sc.Text()
		if strings.HasPrefix(line, "data: ") && strings.Contains(line, `"type":"decided"`) && strings.Contains(line, ap.ID) {
			if err := <-postDone; err != nil {
				t.Fatalf("post: %v", err)
			}
			return
		}
	}
	if sc.Err() != nil {
		t.Fatal(sc.Err())
	}
	t.Fatal("未收到 decided SSE 广播")
}

func TestDemoEndpoint(t *testing.T) {
	h := New(Deps{HITL: govern.NewManager(), Store: memory.NewStore(t.TempDir()), Secret: &memSecret{}})
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/api/demo", nil))
	if rr.Code != http.StatusNotImplemented {
		t.Fatalf("DemoFunc=nil 应 501，code=%d", rr.Code)
	}

	h = New(Deps{
		HITL:   govern.NewManager(),
		Store:  memory.NewStore(t.TempDir()),
		Secret: &memSecret{},
		DemoFunc: func(ctx context.Context) ([]map[string]any, error) {
			return []map[string]any{{"name": "guardrail-intercept", "pass": true}}, nil
		},
	})
	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/api/demo", nil))
	if rr.Code != http.StatusOK {
		t.Fatalf("code=%d body=%s", rr.Code, rr.Body.String())
	}
	var rs []map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &rs); err != nil {
		t.Fatal(err)
	}
	if len(rs) != 1 || rs[0]["name"] != "guardrail-intercept" {
		t.Fatalf("rs=%v", rs)
	}
}

func TestStaticIndexServed(t *testing.T) {
	h := New(Deps{HITL: govern.NewManager(), Store: memory.NewStore(t.TempDir()), Secret: &memSecret{}})
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/", nil))
	if rr.Code != http.StatusOK {
		t.Fatalf("code=%d body=%s", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), "gavel") {
		t.Fatalf("body=%s", rr.Body.String())
	}
}
