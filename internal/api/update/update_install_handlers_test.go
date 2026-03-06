package update

import (
	"os"
	"path/filepath"
	"testing"
)

// Test copyFile function
func TestCopyFile(t *testing.T) {
	// Create a temporary source file
	tmpDir := t.TempDir()
	srcPath := filepath.Join(tmpDir, "source.txt")
	dstPath := filepath.Join(tmpDir, "dest.txt")

	// Write test content
	testContent := []byte("test content")
	if err := os.WriteFile(srcPath, testContent, 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Test copy
	if err := copyFile(srcPath, dstPath); err != nil {
		t.Fatalf("copyFile failed: %v", err)
	}

	// Verify content
	content, err := os.ReadFile(dstPath)
	if err != nil {
		t.Fatalf("Failed to read destination file: %v", err)
	}

	if string(content) != string(testContent) {
		t.Errorf("Content mismatch: got %s, want %s", content, testContent)
	}
}
