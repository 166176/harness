package secret

import (
	"errors"

	"github.com/zalando/go-keyring"
)

// keyring 使用的固定 service 与 user（安全红线：不做成可配置的明文回退）。
const (
	keyringService = "gavel"
	keyringUser    = "default"
)

// keyringProvider 基于操作系统 keyring（Windows Credential Manager / Secret Service）。
type keyringProvider struct{}

// KeyringProvider 返回 go-keyring 后端 Provider，service="gavel"，user="default"。
func KeyringProvider() Provider { return keyringProvider{} }

func (k keyringProvider) Name() string { return "keyring" }

// Get 把 go-keyring 的 ErrNotFound 映射为本包 ErrNotFound，其余错误原样上抛。
func (k keyringProvider) Get() (string, error) {
	v, err := keyring.Get(keyringService, keyringUser)
	if err != nil {
		if errors.Is(err, keyring.ErrNotFound) {
			return "", ErrNotFound
		}
		return "", err
	}
	return v, nil
}

func (k keyringProvider) Set(key string) error {
	return keyring.Set(keyringService, keyringUser, key)
}

// Clear 同样把 go-keyring 的 ErrNotFound 映射为本包 ErrNotFound，
// 避免外来哨兵错误泄漏到包外。
func (k keyringProvider) Clear() error {
	if err := keyring.Delete(keyringService, keyringUser); err != nil {
		if errors.Is(err, keyring.ErrNotFound) {
			return ErrNotFound
		}
		return err
	}
	return nil
}
