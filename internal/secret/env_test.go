package secret

import (
	"testing"
)

func TestEnvProviderReadsVar(t *testing.T) {
	t.Setenv("GAVEL_API_KEY", "sk-env-1234567890abcd")
	p := EnvProvider("GAVEL_API_KEY")
	v, err := p.Get()
	if err != nil || v != "sk-env-1234567890abcd" {
		t.Fatalf("got %q err=%v", v, err)
	}
	if p.Name() != "env:GAVEL_API_KEY" {
		t.Fatalf("name=%s", p.Name())
	}
}

func TestEnvProviderMissingReturnsNotFound(t *testing.T) {
	p := EnvProvider("GAVEL_NOT_SET_VAR_X")
	if _, err := p.Get(); err != ErrNotFound {
		t.Fatalf("got %v", err)
	}
}

func TestEnvProviderReadOnly(t *testing.T) {
	p := EnvProvider("GAVEL_API_KEY")
	if err := p.Set("x"); err == nil {
		t.Fatal("env provider 应只读")
	}
	if err := p.Clear(); err == nil {
		t.Fatal("env provider 应只读")
	}
}

func TestChainKeyringDotenvEnvFallback(t *testing.T) {
	t.Setenv("GAVEL_API_KEY", "sk-env-only")
	// keyring 与 .env 均不可用时应回退到环境变量
	ch := Chain(&fakeProv{name: "a"}, EnvProvider("GAVEL_API_KEY"))
	v, err := ch.Get()
	if err != nil || v != "sk-env-only" {
		t.Fatalf("got %q err=%v", v, err)
	}
}
