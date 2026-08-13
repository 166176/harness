package secret

import (
	"fmt"
	"os"
	"strings"
)

// dotenvKey 为 .env 文件中存储凭据的固定键名。
const dotenvKey = "GAVEL_API_KEY"

// dotenvProvider 以单行 "GAVEL_API_KEY=<value>" 读写 .env 文件，权限 0600。
type dotenvProvider struct {
	path string
}

// DotenvProvider 返回基于指定路径 .env 文件的 Provider。
func DotenvProvider(path string) Provider {
	return &dotenvProvider{path: path}
}

func (d *dotenvProvider) Name() string { return "dotenv" }

// Get 读取单行 GAVEL_API_KEY=<value>；文件不存在 → ErrNotFound。
func (d *dotenvProvider) Get() (string, error) {
	b, err := os.ReadFile(d.path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", ErrNotFound
		}
		return "", err
	}
	line := strings.TrimSpace(string(b))
	prefix := dotenvKey + "="
	if !strings.HasPrefix(line, prefix) {
		return "", fmt.Errorf("secret: dotenv %s: missing %q line", d.path, prefix)
	}
	return line[len(prefix):], nil
}

// Set 以 0600 权限写入单行 GAVEL_API_KEY=<value>。
func (d *dotenvProvider) Set(key string) error {
	if err := os.WriteFile(d.path, []byte(dotenvKey+"="+key+"\n"), 0o600); err != nil {
		return err
	}
	// 已存在文件不随 WriteFile 改变权限，显式确保 0600。
	return os.Chmod(d.path, 0o600)
}

// Clear 删除文件；文件本就不存在视为成功（幂等）。
func (d *dotenvProvider) Clear() error {
	if err := os.Remove(d.path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}
