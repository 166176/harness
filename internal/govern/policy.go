package govern

import (
	_ "embed"

	"gopkg.in/yaml.v3"
)

// Package govern 提供护栏规则引擎（确定性代码，非提示词）与 HITL 状态机。
// Task 5 产出：Policy 结构 + 默认策略 + Check() 三态判定。

//go:embed default_policy.yaml
var defaultPolicyYAML []byte

func DefaultPolicy() Policy {
	var p Policy
	_ = yaml.Unmarshal(defaultPolicyYAML, &p) // 内置文件保证可解析
	return p
}
