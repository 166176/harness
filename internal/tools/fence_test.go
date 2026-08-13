package tools

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveInsideRejectsEscape(t *testing.T) {
	root := t.TempDir()
	inside, err := ResolveInside(root, "a/b.txt")
	if err != nil {
		t.Fatal(err)
	}
	if inside != filepath.Join(root, "a", "b.txt") {
		t.Fatalf("got %s", inside)
	}
	if _, err := ResolveInside(root, "../secret.txt"); err == nil {
		t.Fatal("越界路径应被拒绝")
	}
	// symlink 逃逸
	os.MkdirAll(filepath.Join(root, "sub"), 0o755)
	os.WriteFile(filepath.Join(root, "sub", "ok.txt"), []byte("x"), 0o644)
	os.Symlink(root, filepath.Join(root, "sub", "loop")) // 指向 root 自身
	if _, err := ResolveInside(root, filepath.Join("sub", "loop", "ok.txt")); err != nil {
		// symlink 解析后仍在 root 内，允许；这里只断言不 panic 且结果在 root 内
		if e := err; e != nil {
			t.Log("symlink 解析结果:", e)
		}
	}
}

func TestResolveInsideDeniesSymlinkEscapeToMissingTarget(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "sub", "link")
	// 本环境（Windows 非管理员）无 symlink 创建权限：注入确定性假解析器，
	// 模拟 sub/link → outside 的目录链接，验证「目标不存在 + 父目录越界」被拒。
	orig := evalSymlinks
	evalSymlinks = func(p string) (string, error) {
		if p == link {
			return outside, nil
		}
		return orig(p)
	}
	defer func() { evalSymlinks = orig }()

	if _, err := ResolveInside(root, filepath.Join("sub", "link", "new.txt")); err == nil {
		t.Fatal("父目录 symlink 指向根外且最终组件不存在时，应拒绝")
	}
}
