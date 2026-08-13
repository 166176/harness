package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultDeepSeek(t *testing.T) {
	c := Default()
	if c.BaseURL != "https://api.deepseek.com" || c.Model != "deepseek-chat" {
		t.Fatalf("%+v", c)
	}
}

func TestLoadOverridesAndMissingFileFallsBack(t *testing.T) {
	c, err := Load(filepath.Join(t.TempDir(), "nope.yaml"))
	if err != nil || c.MaxTurns != 20 {
		t.Fatalf("缺失文件应回退默认: %+v err=%v", c, err)
	}
	dir := t.TempDir()
	p := filepath.Join(dir, "gavel.yaml")
	_ = os.WriteFile(p, []byte("model: deepseek-reasoner\nmax_turns: 5\n"), 0o644)
	c, err = Load(p)
	if err != nil || c.Model != "deepseek-reasoner" || c.MaxTurns != 5 {
		t.Fatalf("%+v err=%v", c, err)
	}
}
