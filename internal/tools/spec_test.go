package tools

import (
	"encoding/json"
	"reflect"
	"strings"
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

// eqStrings 比较字符串切片，nil 与空切片视为相等。
func eqStrings(a, b []string) bool {
	if len(a) == 0 && len(b) == 0 {
		return true
	}
	return reflect.DeepEqual(a, b)
}

// TestShellToolSpecDeclaresCommand 校验 run_shell/run_test 的 Spec().Parameters
// 声明 command 属性（R1 修复：真实 LLM 按 schema 生成参数时必须产生 command 字段）。
func TestShellToolSpecDeclaresCommand(t *testing.T) {
	tests := []struct {
		name     string
		tool     Tool
		required []string
	}{
		{"run_shell", ShellTool(&fakeRunner{}, "/repo"), []string{"command"}},
		{"run_test", TestTool(&fakeRunner{}, "/repo", "go test ./..."), nil},
	}
	for _, tc := range tests {
		params := tc.tool.Spec().Parameters
		if params["type"] != "object" {
			t.Fatalf("%s: Parameters 缺 type=object，got %v", tc.name, params["type"])
		}
		props, ok := params["properties"].(map[string]any)
		if !ok {
			t.Fatalf("%s: properties 缺失或类型错误，got %v", tc.name, params["properties"])
		}
		p, ok := props["command"].(map[string]any)
		if !ok || p["type"] != "string" {
			t.Fatalf("%s: 属性 command 应为 {type: string}，got %v", tc.name, props["command"])
		}
		var req []string
		if r, ok := params["required"].([]string); ok {
			req = r
		}
		if !eqStrings(req, tc.required) {
			t.Fatalf("%s: required 应为 %v，got %v", tc.name, tc.required, params["required"])
		}
	}
}

// TestToolSpecsJSONMarshalNoNullRequired 回归测试：真实 LLM 验收发现 DeepSeek 拒绝
// `required: null`（"null is not of type array"）。schema JSON 序列化后 required
// 必须是数组（可空数组），不得为 null。mock 驱动的单测此前未暴露此缺陷。
func TestToolSpecsJSONMarshalNoNullRequired(t *testing.T) {
	tools := []Tool{
		ShellTool(&fakeRunner{}, "/repo"),
		TestTool(&fakeRunner{}, "/repo", "go test ./..."),
	}
	for _, tl := range tools {
		spec := tl.Spec()
		b, err := json.Marshal(spec)
		if err != nil {
			t.Fatalf("%s: Marshal 失败: %v", spec.Name, err)
		}
		if strings.Contains(string(b), `"required":null`) {
			t.Fatalf("%s: schema required 为 null（真实 LLM 拒绝），got %s", spec.Name, b)
		}
		req, ok := spec.Parameters["required"]
		if !ok {
			t.Fatalf("%s: Parameters 缺 required", spec.Name)
		}
		if _, ok := req.([]string); !ok {
			t.Fatalf("%s: required 应为 []string，got %T (%v)", spec.Name, req, req)
		}
	}
}
