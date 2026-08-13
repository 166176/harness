package tools

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/166176/harness/internal/llm"
)

// Tool 是 harness 可分发的最小动作单元。
type Tool interface {
	Name() string
	Spec() llm.ToolSpec
	Execute(ctx context.Context, args map[string]any) (string, error)
}

type fileTool struct {
	name, desc string
	root       string
	params     map[string]any
	fn         func(ctx context.Context, root string, args map[string]any) (string, error)
}

func (t *fileTool) Name() string { return t.name }
func (t *fileTool) Spec() llm.ToolSpec {
	p := t.params
	if p == nil {
		// 未显式声明 schema 的工具（如 run_shell/run_test）保持原有 path 属性回退。
		p = map[string]any{"type": "object", "properties": map[string]any{"path": map[string]any{"type": "string"}}}
	}
	return llm.ToolSpec{Name: t.name, Description: t.desc, Parameters: p}
}
func (t *fileTool) Execute(ctx context.Context, args map[string]any) (string, error) {
	return t.fn(ctx, t.root, args)
}

// fileParams 构造工具参数 schema；属性均为 string，required 声明必填字段。
func fileParams(propNames, required []string) map[string]any {
	props := map[string]any{}
	for _, n := range propNames {
		props[n] = map[string]any{"type": "string"}
	}
	return map[string]any{"type": "object", "properties": props, "required": required}
}

func argStr(args map[string]any, key string) string {
	if v, ok := args[key].(string); ok {
		return v
	}
	return ""
}

// FileTools 返回 read_file/write_file/list_files/grep 四个工具。
func FileTools(root string) []Tool {
	return []Tool{
		&fileTool{name: "read_file", desc: "读取仓库内文件", params: fileParams([]string{"path"}, []string{"path"}), root: root, fn: func(_ context.Context, root string, a map[string]any) (string, error) {
			p, err := ResolveInside(root, argStr(a, "path"))
			if err != nil {
				return "", err
			}
			b, err := os.ReadFile(p)
			if err != nil {
				return "", err
			}
			return string(b), nil
		}},
		&fileTool{name: "write_file", desc: "写入仓库内文件", params: fileParams([]string{"path", "content"}, []string{"path", "content"}), root: root, fn: func(_ context.Context, root string, a map[string]any) (string, error) {
			p, err := ResolveInside(root, argStr(a, "path"))
			if err != nil {
				return "", err
			}
			if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
				return "", err
			}
			c := argStr(a, "content")
			if err := os.WriteFile(p, []byte(c), 0o644); err != nil {
				return "", err
			}
			return fmt.Sprintf("wrote %d bytes", len(c)), nil
		}},
		&fileTool{name: "list_files", desc: "列出目录", params: fileParams([]string{"path"}, []string{"path"}), root: root, fn: func(_ context.Context, root string, a map[string]any) (string, error) {
			p, err := ResolveInside(root, argStr(a, "path"))
			if err != nil {
				return "", err
			}
			ents, err := os.ReadDir(p)
			if err != nil {
				return "", err
			}
			var b strings.Builder
			for _, e := range ents {
				fmt.Fprintf(&b, "%s\n", e.Name())
			}
			return b.String(), nil
		}},
		&fileTool{name: "grep", desc: "文本搜索", params: fileParams([]string{"path", "pattern"}, []string{"path", "pattern"}), root: root, fn: func(_ context.Context, root string, a map[string]any) (string, error) {
			p, err := ResolveInside(root, argStr(a, "path"))
			if err != nil {
				return "", err
			}
			pat := argStr(a, "pattern")
			var out []string
			_ = filepath.WalkDir(p, func(path string, d os.DirEntry, err error) error {
				if err != nil || d.IsDir() || d.Name() == ".git" {
					return nil
				}
				if b, e := os.ReadFile(path); e == nil && strings.Contains(string(b), pat) {
					out = append(out, path)
				}
				return nil
			})
			return strings.Join(out, "\n"), nil
		}},
	}
}
