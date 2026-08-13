package tools

import (
	"errors"
	"path/filepath"
	"strings"
)

// errEscape 表示路径解析后越出 repo 根。
var errEscape = errors.New("path escapes repo root")

// evalSymlinks 是 filepath.EvalSymlinks 的可替换封装，测试可在无 symlink 权限的
// 环境下注入假解析器；生产环境始终指向标准库实现。
var evalSymlinks = filepath.EvalSymlinks

// ResolveInside 归一化路径并解析 symlink，校验最终路径位于 root 内。
// 目标存在时返回完整真实路径；目标不存在时对父目录做 symlink 解析（父目录
// 必然已存在或可逐级解析），再拼最终组件——父目录解析后仍在 root 内才允许，
// 封堵「子路径经 symlink 指向根外 + 最终组件不存在」的逃逸。
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

	// 目标存在：返回完整真实路径。
	if resolved, err := evalSymlinks(clean); err == nil {
		if err := checkInside(absRoot, resolved); err != nil {
			return "", err
		}
		return resolved, nil
	}

	// 目标不存在：解析父目录（逐级向上直到已存在的最深祖先）后再拼最终组件；
	// 父目录解析后越界即拒绝。
	resolvedParent, err := resolveExistingParent(absRoot, filepath.Dir(clean))
	if err != nil {
		return "", err
	}
	return filepath.Join(resolvedParent, filepath.Base(clean)), nil
}

// resolveExistingParent 从 parent 出发逐级向上解析 symlink，直到命中已存在的
// 最深祖先；解析结果须仍在 root 内，未解析的词法后缀原样拼回。
func resolveExistingParent(root, parent string) (string, error) {
	var suffix []string
	cur := parent
	for {
		resolved, err := evalSymlinks(cur)
		if err == nil {
			if err := checkInside(root, resolved); err != nil {
				return "", err
			}
			for i := len(suffix) - 1; i >= 0; i-- {
				resolved = filepath.Join(resolved, suffix[i])
			}
			return resolved, nil
		}
		next := filepath.Dir(cur)
		if next == cur {
			return "", errEscape // 已到卷根仍无已存在祖先可解析
		}
		suffix = append(suffix, filepath.Base(cur))
		cur = next
	}
}

// checkInside 校验 resolved 位于 root 内（含 root 自身）。
func checkInside(root, resolved string) error {
	rel, err := filepath.Rel(root, resolved)
	if err != nil {
		return errEscape
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return errEscape
	}
	return nil
}
