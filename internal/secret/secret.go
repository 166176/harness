// Package secret 提供凭据安全存储：链式 Provider、keyring、.env 回退、掩码与指纹。
// 安全红线：key 绝不硬编码、绝不写入日志、绝不回显明文；对外仅暴露掩码与指纹。
package secret

import (
	"crypto/sha256"
	"errors"
	"fmt"
)

// ErrNotFound 表示目标 provider 中不存在该凭据。
var ErrNotFound = errors.New("secret not found")

// Provider 抽象一个凭据存储后端。
type Provider interface {
	Name() string
	Get() (string, error) // 不存在 → ("", ErrNotFound)
	Set(key string) error
	Clear() error
}

// chain 依次尝试多个 provider 的链式组合。
type chain struct {
	providers []Provider
}

// Chain 将多个 Provider 组合为链：
// Get 依次尝试直到某个 provider 成功；
// Set/Clear 对第一个 Get 成功的 provider 操作，全部失败则返回 ErrNotFound。
func Chain(providers ...Provider) Provider {
	return &chain{providers: providers}
}

func (c *chain) Name() string { return "chain" }

func (c *chain) Get() (string, error) {
	lastErr := ErrNotFound
	for _, p := range c.providers {
		v, err := p.Get()
		if err == nil {
			return v, nil
		}
		lastErr = err
	}
	return "", lastErr
}

// firstAvailable 返回第一个 Get 成功的 provider；全部失败返回 ErrNotFound。
func (c *chain) firstAvailable() (Provider, error) {
	for _, p := range c.providers {
		if _, err := p.Get(); err == nil {
			return p, nil
		}
	}
	return nil, ErrNotFound
}

func (c *chain) Set(key string) error {
	p, err := c.firstAvailable()
	if err != nil {
		return err
	}
	return p.Set(key)
}

func (c *chain) Clear() error {
	p, err := c.firstAvailable()
	if err != nil {
		return err
	}
	return p.Clear()
}

// Mask 只回显凭据前后缀，永不回显全量：
// 长度<=6 → "******"；否则 前3字符 + "..." + 后4字符。
func Mask(s string) string {
	if len(s) <= 6 {
		return "******"
	}
	return s[:3] + "..." + s[len(s)-4:]
}

// Fingerprint 返回凭据 sha256 hex 的前 8 位（审计/比对用，不回显明文）。
func Fingerprint(s string) string {
	sum := sha256.Sum256([]byte(s))
	return fmt.Sprintf("%x", sum)[:8]
}
