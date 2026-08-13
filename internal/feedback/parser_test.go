package feedback

import "testing"

const goTestOut = `--- FAIL: TestAdd (0.00s)
    calc_test.go:12: Add(1,2) = 2, want 3
FAIL
exit status 1`

func TestParseGoTest(t *testing.T) {
	fs := Parse("gotest", goTestOut, 1)
	if len(fs) != 1 {
		t.Fatalf("应解析出 1 个失败，got %d", len(fs))
	}
	f := fs[0]
	if f.File != "calc_test.go" || f.Line != 12 {
		t.Fatalf("got %s:%d", f.File, f.Line)
	}
	if f.Kind != KindAssert {
		t.Fatalf("kind=%s", f.Kind)
	}
	if !contains(f.Message, "want 3") {
		t.Fatalf("message=%q", f.Message)
	}
}

func TestParsePytest(t *testing.T) {
	out := "E   assert 2 == 3\npath/to/test_calc.py:7: AssertionError"
	fs := Parse("pytest", out, 1)
	if len(fs) != 1 || fs[0].File != "path/to/test_calc.py" || fs[0].Line != 7 || fs[0].Kind != KindAssert {
		t.Fatalf("got %+v", fs)
	}
}

func TestGreenOutputNoFailure(t *testing.T) {
	fs := Parse("gotest", "ok  \tgithub.com/x/y\t0.001s", 0)
	if len(fs) != 0 {
		t.Fatalf("全绿应无失败，got %+v", fs)
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(s) > 0 && (index(s, sub) >= 0))
}
func index(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
