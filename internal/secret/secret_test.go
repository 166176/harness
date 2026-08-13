package secret

import (
	"path/filepath"
	"testing"
)

type fakeProv struct {
	v    string
	name string
}

func (f *fakeProv) Name() string { return f.name }
func (f *fakeProv) Get() (string, error) {
	if f.v == "" {
		return "", ErrNotFound
	}
	return f.v, nil
}
func (f *fakeProv) Set(k string) error { f.v = k; return nil }
func (f *fakeProv) Clear() error       { f.v = ""; return nil }

func TestChainFallsBack(t *testing.T) {
	c := Chain(&fakeProv{name: "a"}, &fakeProv{name: "b", v: "sk-1234567890abcd"})
	got, err := c.Get()
	if err != nil || got != "sk-1234567890abcd" {
		t.Fatalf("masked=%q err=%v", Mask(got), err)
	}
}

func TestMaskNeverShowsPlaintext(t *testing.T) {
	m := Mask("sk-1234567890abcdef")
	if m == "sk-1234567890abcdef" || len(m) > 12 {
		t.Fatalf("mask=%q", m)
	}
	if Fingerprint("sk-abc") == "" || len(Fingerprint("sk-abc")) != 8 {
		t.Fatal("指纹应为 8 位 hex")
	}
}

func TestDotenvRoundtrip(t *testing.T) {
	p := filepath.Join(t.TempDir(), ".env")
	d := DotenvProvider(p)
	if err := d.Set("sk-secret-value"); err != nil {
		t.Fatal(err)
	}
	got, err := d.Get()
	if err != nil || got != "sk-secret-value" {
		t.Fatalf("masked=%q err=%v", Mask(got), err)
	}
	if err := d.Clear(); err != nil {
		t.Fatal(err)
	}
	if _, err := d.Get(); err != ErrNotFound {
		t.Fatal("清除后应 ErrNotFound")
	}
}
