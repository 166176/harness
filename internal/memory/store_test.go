package memory

import "testing"

func TestPutGetRoundtrip(t *testing.T) {
	s := NewStore(t.TempDir())
	type hint struct{ Note string }
	if err := s.Put("hint:abc", hint{Note: "测试命令已确认"}); err != nil {
		t.Fatal(err)
	}
	var got hint
	ok, err := s.Get("hint:abc", &got)
	if err != nil || !ok {
		t.Fatalf("ok=%v err=%v", ok, err)
	}
	if got.Note != "测试命令已确认" {
		t.Fatalf("got %+v", got)
	}
}

func TestDeleteAndList(t *testing.T) {
	s := NewStore(t.TempDir())
	_ = s.Put("session:1", map[string]any{"x": 1})
	_ = s.Put("session:2", map[string]any{"x": 2})
	ls, err := s.List("session:")
	if err != nil || len(ls) != 2 {
		t.Fatalf("got %v err=%v", ls, err)
	}
	if err := s.Delete("session:1"); err != nil {
		t.Fatal(err)
	}
	ls, _ = s.List("session:")
	if len(ls) != 1 {
		t.Fatalf("删除后应剩 1，got %v", ls)
	}
}
