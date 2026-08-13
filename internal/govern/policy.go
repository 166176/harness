package govern

import (
	_ "embed"
	"fmt"
	"regexp"

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

// Validate 逐个编译 deny/approval 正则，任一非法即返回错误（fail-closed：
// 坏正则不得静默失效导致危险命令绕过护栏）。
func (p Policy) Validate() error {
	for i, pat := range p.DenyPatterns {
		if _, err := regexp.Compile(pat); err != nil {
			return fmt.Errorf("deny_patterns[%d] %q: %w", i, pat, err)
		}
	}
	for i, pat := range p.ApprovalPatterns {
		if _, err := regexp.Compile(pat); err != nil {
			return fmt.Errorf("approval_patterns[%d] %q: %w", i, pat, err)
		}
	}
	return nil
}
