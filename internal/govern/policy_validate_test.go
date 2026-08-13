package govern

import "testing"

func TestPolicyValidateRejectsBadRegex(t *testing.T) {
	p := Policy{ApprovalPatterns: []string{`(?i)\brm\b`, `([a-z`}}
	if err := p.Validate(); err == nil {
		t.Fatal("非法正则必须报错（fail-closed）")
	}
}

func TestPolicyValidateAcceptsDefault(t *testing.T) {
	if err := DefaultPolicy().Validate(); err != nil {
		t.Fatalf("默认策略应合法：%v", err)
	}
}
