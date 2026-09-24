package utils

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRotatingFileKeepsActiveFileWithinLimit(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "debug.log")
	file, err := NewRotatingFile(path, RotatingFileConfig{MaxSize: 10, MaxFiles: 2})
	if err != nil {
		t.Fatalf("NewRotatingFile() error = %v", err)
	}

	if _, err := file.Write([]byte(strings.Repeat("x", 25))); err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	if err := file.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	for _, name := range []string{"debug.log", "debug.log.1", "debug.log.2"} {
		info, err := os.Stat(filepath.Join(dir, name))
		if err != nil {
			t.Fatalf("stat %s: %v", name, err)
		}
		if info.Size() > 10 {
			t.Fatalf("%s size = %d, want <= 10", name, info.Size())
		}
	}
	if _, err := os.Stat(filepath.Join(dir, "debug.log.3")); !os.IsNotExist(err) {
		t.Fatalf("unexpected debug.log.3, err = %v", err)
	}
}

func TestRotatingFileTruncateOnOpen(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "tasks.log")
	if err := os.WriteFile(path, []byte("old content"), 0666); err != nil {
		t.Fatal(err)
	}

	file, err := NewRotatingFile(path, RotatingFileConfig{MaxSize: 100, MaxFiles: 1, TruncateOnOpen: true})
	if err != nil {
		t.Fatalf("NewRotatingFile() error = %v", err)
	}
	defer file.Close()

	if _, err := file.Write([]byte("new")); err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := string(data); got != "new" {
		t.Fatalf("active file = %q, want %q", got, "new")
	}
}
