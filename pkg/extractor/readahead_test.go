package extractor

import (
	"os"
	"path/filepath"
	"testing"
)

// More files than the window, read in walk order: every one comes back intact, and a path the walk
// never listed still reads (plain fallback).
func TestReadAheadReturnsEveryFileInOrder(t *testing.T) {
	dir := t.TempDir()
	var order []string
	for i := 0; i < readAheadWindow*3; i++ {
		p := filepath.Join(dir, string(rune('a'+i%26))+string(rune('a'+i/26))+".ts")
		if err := os.WriteFile(p, []byte(p), 0o644); err != nil {
			t.Fatal(err)
		}
		order = append(order, p)
	}
	other := filepath.Join(dir, "notes.txt")
	os.WriteFile(other, []byte("x"), 0o644)
	read := newReadAhead(dir)
	filepath.WalkDir(dir, func(p string, d os.DirEntry, err error) error {
		if d.IsDir() || filepath.Ext(p) != ".ts" {
			return nil
		}
		b, err := read(p)
		if err != nil || string(b) != p {
			t.Fatalf("%s: got %q, %v", p, b, err)
		}
		return nil
	})
	if b, err := read(other); err != nil || string(b) != "x" {
		t.Fatalf("fallback read failed: %q %v", b, err)
	}
	_ = order
}
