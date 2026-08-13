package tools

import (
	"context"
	"fmt"

	"github.com/166176/harness/internal/llm"
)

type Registry struct{ byName map[string]Tool }

func RegistryOf(tools []Tool) *Registry {
	r := &Registry{byName: map[string]Tool{}}
	for _, tl := range tools {
		r.byName[tl.Name()] = tl
	}
	return r
}

// Specs 返回注册表中全部工具的 LLM schema，供主循环组装工具说明与调用参数。
func (rg *Registry) Specs() []llm.ToolSpec {
	out := make([]llm.ToolSpec, 0, len(rg.byName))
	for _, tl := range rg.byName {
		out = append(out, tl.Spec())
	}
	return out
}

func (rg *Registry) Dispatch(ctx context.Context, name string, args map[string]any) (string, error) {
	tl, ok := rg.byName[name]
	if !ok {
		return "", fmt.Errorf("unknown tool %q", name)
	}
	return tl.Execute(ctx, args)
}
