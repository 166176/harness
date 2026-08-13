package demo

import (
	"context"
	"testing"
)

func TestAllThreeScenariosPass(t *testing.T) {
	rs := Run(context.Background())
	if len(rs) != 3 {
		t.Fatalf("应有 3 个场景，got %d", len(rs))
	}
	for _, r := range rs {
		if !r.Pass {
			t.Fatalf("场景 %s 失败: %v", r.Name, r.Trace)
		}
	}
}

func TestScenarioNames(t *testing.T) {
	rs := Run(context.Background())
	names := map[string]bool{}
	for _, r := range rs {
		names[r.Name] = true
	}
	for _, want := range []string{"guardrail-intercept", "feedback-loop", "hitl-timeout-autodeny"} {
		if !names[want] {
			t.Fatalf("缺少场景 %s", want)
		}
	}
}
