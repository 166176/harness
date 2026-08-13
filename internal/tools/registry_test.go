package tools

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestRegistryDispatchUnknown(t *testing.T) {
	rg := RegistryOf(nil)
	_, err := rg.Dispatch(context.Background(), "nope", nil)
	if err == nil {
		t.Fatal("未知工具应报错")
	}
}

func TestRegistryDispatchKnown(t *testing.T) {
	root := t.TempDir()
	// brief 原样测试在空目录上 list_files 输出为空字符串，与断言矛盾；先种一个文件。
	if err := os.WriteFile(filepath.Join(root, "a.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	rg := RegistryOf(FileTools(root))
	out, err := rg.Dispatch(context.Background(), "list_files", map[string]any{"path": "."})
	if err != nil {
		t.Fatal(err)
	}
	if out == "" {
		t.Fatal("输出不应为空")
	}
}
