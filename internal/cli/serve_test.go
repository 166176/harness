package cli

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/166176/harness/internal/llm"
	"github.com/166176/harness/internal/server"
)

// C1：有 key 时 serve 装配的 SessionRunner 非 nil，端点经 httptest 返回 202。
func TestServeDepsRunnerWiredWithKey(t *testing.T) {
	oldClient := newLLMClient
	oldDir := dataDirFn
	defer func() { newLLMClient = oldClient; dataDirFn = oldDir }()
	dir := t.TempDir()
	dataDirFn = func() string { return dir }
	newLLMClient = func(baseURL, model, apiKey string) llm.Client {
		return &llm.ScriptedMock{Steps: []llm.Completion{{Message: llm.Message{Role: llm.RoleAssistant}, Done: true}}}
	}
	a := &app{stdin: strings.NewReader(""), stdout: io.Discard, stderr: io.Discard, secret: fakeSecretProvider{}}
	deps := a.serveDeps("")
	if deps.SessionRunner == nil {
		t.Fatal("有 key 时 SessionRunner 不应为 nil")
	}
	h := server.New(deps)
	rr := httptest.NewRecorder()
	body := bytes.NewBufferString(`{"repo":"/tmp/r","test":"go test ./...","task":"修复"}`)
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/api/sessions", body))
	if rr.Code != http.StatusAccepted {
		t.Fatalf("code=%d body=%s", rr.Code, rr.Body.String())
	}
}

// C1：无 key 时 serve 仍装配 SessionRunner，但端点返回 500 + key set 引导。
func TestServeDepsSessionEndpoint500WithoutKey(t *testing.T) {
	oldDir := dataDirFn
	defer func() { dataDirFn = oldDir }()
	dir := t.TempDir()
	dataDirFn = func() string { return dir }
	a := &app{stdin: strings.NewReader(""), stdout: io.Discard, stderr: io.Discard, secret: emptySecretProvider{}}
	deps := a.serveDeps("")
	if deps.SessionRunner == nil {
		t.Fatal("无 key 时也应装配 SessionRunner（由端点返回 500 引导）")
	}
	h := server.New(deps)
	rr := httptest.NewRecorder()
	body := bytes.NewBufferString(`{"repo":"r","test":"t","task":"x"}`)
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/api/sessions", body))
	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("code=%d body=%s", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), "key set") {
		t.Fatalf("500 应包含 key set 引导：%s", rr.Body.String())
	}
}
