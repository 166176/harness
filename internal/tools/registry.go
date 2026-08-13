package tools

import (
	"context"
	"fmt"
)

type Registry struct{ byName map[string]Tool }

func RegistryOf(tools []Tool) *Registry {
	r := &Registry{byName: map[string]Tool{}}
	for _, tl := range tools {
		r.byName[tl.Name()] = tl
	}
	return r
}

func (rg *Registry) Dispatch(ctx context.Context, name string, args map[string]any) (string, error) {
	tl, ok := rg.byName[name]
	if !ok {
		return "", fmt.Errorf("unknown tool %q", name)
	}
	return tl.Execute(ctx, args)
}
