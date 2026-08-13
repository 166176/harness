package tools

import (
	"reflect"
	"testing"
)

// TestFileToolSpecsDeclareParameters 校验各文件工具 Spec().Parameters 完整声明
// properties 与 required，避免真实 LLM 按 schema 生成参数时缺失字段。
func TestFileToolSpecsDeclareParameters(t *testing.T) {
	want := map[string]struct {
		props    []string
		required []string
	}{
		"read_file":  {props: []string{"path"}, required: []string{"path"}},
		"write_file": {props: []string{"path", "content"}, required: []string{"path", "content"}},
		"list_files": {props: []string{"path"}, required: []string{"path"}},
		"grep":       {props: []string{"path", "pattern"}, required: []string{"path", "pattern"}},
	}
	seen := map[string]bool{}
	for _, tl := range FileTools(t.TempDir()) {
		seen[tl.Name()] = true
		w, ok := want[tl.Name()]
		if !ok {
			t.Fatalf("意外工具 %s", tl.Name())
		}
		params := tl.Spec().Parameters
		if params["type"] != "object" {
			t.Fatalf("%s: Parameters 缺 type=object，got %v", tl.Name(), params["type"])
		}
		props, ok := params["properties"].(map[string]any)
		if !ok {
			t.Fatalf("%s: properties 缺失或类型错误，got %v", tl.Name(), params["properties"])
		}
		for _, k := range w.props {
			p, ok := props[k].(map[string]any)
			if !ok || p["type"] != "string" {
				t.Fatalf("%s: 属性 %s 应为 {type: string}，got %v", tl.Name(), k, props[k])
			}
		}
		req, ok := params["required"].([]string)
		if !ok || !reflect.DeepEqual(req, w.required) {
			t.Fatalf("%s: required 应为 %v，got %v", tl.Name(), w.required, params["required"])
		}
	}
	for name := range want {
		if !seen[name] {
			t.Fatalf("缺少工具 %s", name)
		}
	}
}
