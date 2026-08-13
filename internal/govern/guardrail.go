package govern

import (
	"path"
	"regexp"
	"strings"
)

type Decision string

const (
	Allow    Decision = "allow"
	Deny     Decision = "deny"
	Approval Decision = "approval"
)

type Action struct {
	Tool string
	Args map[string]any
}

type Verdict struct {
	Decision Decision
	Rule     string
}

// GuardContext 携带判定所需上下文；SecretKey 仅用于泄露比对，绝不落盘/落日志。
type GuardContext struct {
	RepoRoot  string
	SecretKey string
}

type Policy struct {
	ApprovalPatterns       []string `yaml:"approval_patterns"`
	DenyPatterns           []string `yaml:"deny_patterns"`
	ApprovalTimeoutSeconds int      `yaml:"approval_timeout_seconds"`
}

// Check 对动作做确定性护栏判定（§A.4-B：机制是代码，不是提示词）。
// 判定顺序：密钥泄露 deny → 越界围栏 deny → deny 规则 → approval 规则 → allow。
func Check(p Policy, gc GuardContext, a Action) Verdict {
	// ① 密钥泄露（最高优先）：动作参数序列化后与已存 key 比对命中 → deny
	if gc.SecretKey != "" {
		for _, v := range a.Args {
			if s, ok := v.(string); ok && strings.Contains(s, gc.SecretKey) {
				return Verdict{Decision: Deny, Rule: "secret-leak"}
			}
		}
	}

	// ② 范围围栏：文件类工具的 path 经 path.Clean 判断越界 → deny
	if pv, ok := a.Args["path"].(string); ok && pv != "" {
		cleaned := path.Clean(pv)
		if path.IsAbs(cleaned) || cleaned == ".." || strings.HasPrefix(cleaned, "../") {
			return Verdict{Decision: Deny, Rule: "fence"}
		}
	}

	// 汇总命令字符串用于规则匹配
	if cmd, ok := a.Args["command"].(string); ok && cmd != "" {
		// ③ 依次匹配 deny_patterns → deny
		for _, pat := range p.DenyPatterns {
			if match(pat, cmd) {
				return Verdict{Decision: Deny, Rule: pat}
			}
		}
		// ③ 依次匹配 approval_patterns → approval
		for _, pat := range p.ApprovalPatterns {
			if match(pat, cmd) {
				return Verdict{Decision: Approval, Rule: pat}
			}
		}
	}

	// ④ 其余 → allow
	return Verdict{Decision: Allow, Rule: ""}
}

func match(pat, s string) bool {
	re, err := regexp.Compile(pat)
	if err != nil {
		return false
	}
	return re.MatchString(s)
}
