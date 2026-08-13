package tools

import (
	"context"
	"testing"
)

func TestWriteFileDeniesOutsideRoot(t *testing.T) {
	root := t.TempDir()
	tools := FileTools(root)
	for _, tl := range tools {
		if tl.Name() == "write_file" {
			_, err := tl.Execute(context.Background(), map[string]any{"path": "../escape.txt", "content": "x"})
			if err == nil {
				t.Fatal("越界写必须报错")
			}
		}
	}
}

func TestWriteAndReadRoundtrip(t *testing.T) {
	root := t.TempDir()
	tools := FileTools(root)
	write, read := pick(tools, "write_file"), pick(tools, "read_file")
	if _, err := write.Execute(context.Background(), map[string]any{"path": "a.txt", "content": "hello"}); err != nil {
		t.Fatal(err)
	}
	out, err := read.Execute(context.Background(), map[string]any{"path": "a.txt"})
	if err != nil {
		t.Fatal(err)
	}
	if out != "hello" {
		t.Fatalf("got %q", out)
	}
}

func pick(ts []Tool, name string) Tool {
	for _, tl := range ts {
		if tl.Name() == name {
			return tl
		}
	}
	return nil
}
