// Package config 提供 YAML 配置加载与内置默认值。
package config

import (
	"os"

	"gopkg.in/yaml.v3"
)

// Config 聚合运行时配置项。
type Config struct {
	BaseURL    string `yaml:"base_url"`
	Model      string `yaml:"model"`
	MaxTurns   int    `yaml:"max_turns"`
	TimeoutSec int    `yaml:"timeout_seconds"`
	PolicyPath string `yaml:"policy_path"`
}

// Default 返回内置默认配置。
func Default() Config {
	return Config{
		BaseURL:    "https://api.deepseek.com",
		Model:      "deepseek-chat",
		MaxTurns:   20,
		TimeoutSec: 900,
	}
}

// Load 读取 YAML 配置：文件不存在时回退默认值，存在时在默认值之上覆盖。
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			c := Default()
			return &c, nil
		}
		return nil, err
	}
	c := Default()
	if err := yaml.Unmarshal(data, &c); err != nil {
		return nil, err
	}
	return &c, nil
}
