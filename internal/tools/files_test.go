package tools

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestWriteFileDeniesOutsideRoot(t *testing.T) {
	root := t.TempDir()
	tools := FileTools(root)
	for _, tl := range tools {
		if tl.Name() == "write_file" {
			_, err := tl.Execute(context.Background(), map[string]any{"path": "../escape.txt", "content": "x"})
			if err == nil {
				t.Fatal("越界写必须报错")
			}
		}
	}
}

func TestWriteAndReadRoundtrip(t *testing.T) {
	root := t.TempDir()
	tools := FileTools(root)
	write, read := pick(tools, "write_file"), pick(tools, "read_file")
	if _, err := write.Execute(context.Background(), map[string]any{"path": "a.txt", "content": "hello"}); err != nil {
		t.Fatal(err)
	}
	out, err := read.Execute(context.Background(), map[string]any{"path": "a.txt"})
	if err != nil {
		t.Fatal(err)
	}
	if out != "hello" {
		t.Fatalf("got %q", out)
	}
}

func TestFileToolsDenySymlinkEscape(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "sub", "link")); err != nil {
		t.Skipf("无 symlink 创建权限，跳过（与本环境一致）: %v", err)
	}
	tools := FileTools(root)
	write, read := pick(tools, "write_file"), pick(tools, "read_file")
	// new.txt 不存在 → 词法路径仍在 root 内，但父目录 link 指向根外 → 必须拒绝。
	if _, err := write.Execute(context.Background(), map[string]any{"path": filepath.Join("sub", "link", "new.txt"), "content": "x"}); err == nil {
		t.Fatal("write_file 经父目录 symlink 逃逸必须被围栏拒绝")
	}
	if _, err := read.Execute(context.Background(), map[string]any{"path": filepath.Join("sub", "link", "new.txt")}); err == nil {
		t.Fatal("read_file 经父目录 symlink 逃逸必须被围栏拒绝")
	}
	if _, err := os.Stat(filepath.Join(outside, "new.txt")); err == nil {
		t.Fatal("根外目录不应被写入")
	}
}

func pick(ts []Tool, name string) Tool {
	for _, tl := range ts {
		if tl.Name() == name {
			return tl
		}
	}
	return nil
}
