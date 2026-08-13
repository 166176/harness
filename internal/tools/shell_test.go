package tools

import (
	"context"
	"strings"
	"testing"
)

type fakeRunner struct{ called []string }

func (f *fakeRunner) Run(_ context.Context, dir, cmd string, _ int) (string, string, int, error) {
	f.called = append(f.called, dir+"::"+cmd)
	return "out", "err", 0, nil
}

func TestShellToolInvokesRunner(t *testing.T) {
	f := &fakeRunner{}
	tl := ShellTool(f, "/repo")
	out, err := tl.Execute(context.Background(), map[string]any{"command": "go test ./..."})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "out") {
		t.Fatalf("got %q", out)
	}
	if len(f.called) != 1 || !strings.Contains(f.called[0], "go test ./...") {
		t.Fatalf("runner 未收到命令: %v", f.called)
	}
}
