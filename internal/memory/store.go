package memory

import (
	"encoding/json"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

// Store 是泛型 JSON KV 落盘：key 形如 "session:<id>"、"hint:<repoHash>"。
type Store struct{ dir string }

// NewStore 返回以 dir 为存储目录的 Store。
func NewStore(dir string) *Store { return &Store{dir: dir} }

// path 返回 key 对应的文件名：url.QueryEscape(key)+".json"。
func (s *Store) path(key string) string {
	return filepath.Join(s.dir, url.QueryEscape(key)+".json")
}

// Put 将 v JSON 序列化后落盘。
func (s *Store) Put(key string, v any) error {
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(s.dir, 0o755); err != nil {
		return err
	}
	return os.WriteFile(s.path(key), b, 0o644)
}

// Get 读取 key 对应 JSON 到 out；文件不存在返回 (false, nil)。
func (s *Store) Get(key string, out any) (bool, error) {
	b, err := os.ReadFile(s.path(key))
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	if err := json.Unmarshal(b, out); err != nil {
		return false, err
	}
	return true, nil
}

// Delete 删除 key；不存在不报错。
func (s *Store) Delete(key string) error {
	if err := os.Remove(s.path(key)); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// List 返回以 prefix 开头的 key（还原后的原始 key，非文件名）。
func (s *Store) List(prefix string) ([]string, error) {
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	esc := url.QueryEscape(prefix)
	var keys []string
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasPrefix(name, esc) || !strings.HasSuffix(name, ".json") {
			continue
		}
		key, err := url.QueryUnescape(strings.TrimSuffix(name, ".json"))
		if err != nil {
			return nil, err
		}
		keys = append(keys, key)
	}
	return keys, nil
}
