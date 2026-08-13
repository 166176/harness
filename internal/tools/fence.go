package tools

import (
	"errors"
	"path/filepath"
)

// ResolveInside 归一化路径并解析 symlink，校验最终路径位于 root 内。
func ResolveInside(root, p string) (string, error) {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	full := p
	if !filepath.IsAbs(full) {
		full = filepath.Join(absRoot, p)
	}
	clean := filepath.Clean(full)
	resolved, err := filepath.EvalSymlinks(clean)
	if err != nil {
		resolved = clean // 目标不存在时允许继续（后续工具会报错）
	}
	resolved, _ = filepath.Abs(resolved)
	rel, err := filepath.Rel(absRoot, resolved)
	if err != nil || rel == ".." || (len(rel) >= 3 && rel[:3] == ".."+string(filepath.Separator)) {
		return "", errEscape
	}
	return resolved, nil
}

var errEscape = errors.New("path escapes repo root")
