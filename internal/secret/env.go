package secret

import (
	"errors"
	"os"
)

// envProvider 从环境变量读取凭据（云端容器模式：无 keyring、无 .env 文件，
// 由平台注入环境变量；明文风险 = 进程环境可见，SPEC §4.2 已声明）。
type envProvider struct {
	name string
}

// EnvProvider 返回读取指定环境变量的 Provider；变量为空 → ErrNotFound。
// 云端部署（Render 等）使用：GAVEL_API_KEY。
func EnvProvider(name string) Provider {
	return &envProvider{name: name}
}

func (e *envProvider) Name() string { return "env:" + e.name }

func (e *envProvider) Get() (string, error) {
	v := os.Getenv(e.name)
	if v == "" {
		return "", ErrNotFound
	}
	return v, nil
}

// Set/Clear 不支持：环境变量由平台管理，不通过 CLI 写入。
func (e *envProvider) Set(string) error { return errEnvReadOnly }

func (e *envProvider) Clear() error { return errEnvReadOnly }

var errEnvReadOnly = errors.New("env provider 为只读（由平台注入，请用 gavel key set 或 .env 文件写入）")
