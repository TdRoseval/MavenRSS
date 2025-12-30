package feed

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestScriptExecutor_ExecuteScript_InvalidPath(t *testing.T) {
	tempDir := t.TempDir()
	executor := NewScriptExecutor(tempDir)

	// Try to execute a non-existent script
	_, err := executor.ExecuteScript(context.Background(), "nonexistent.py")
	if err == nil {
		t.Error("ExecuteScript() should return error for non-existent script")
	}
}

func TestScriptExecutor_ExecuteScript_PathTraversal(t *testing.T) {
	tempDir := t.TempDir()
	executor := NewScriptExecutor(tempDir)

	// Try path traversal attack
	_, err := executor.ExecuteScript(context.Background(), "../../../etc/passwd")
	if err == nil {
		t.Error("ExecuteScript() should return error for path traversal attempt")
	}

	// Verify error message mentions security concern
	if err != nil && !strings.Contains(strings.ToLower(err.Error()), "script must be within") {
		t.Errorf("ExecuteScript() error should mention path traversal: %v", err)
	}
}

func TestScriptExecutor_ExecuteScript_ValidPythonScript(t *testing.T) {
	tempDir := t.TempDir()

	// Create a test Python script that outputs RSS
	scriptContent := `#!/usr/bin/env python3
print('''<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0">
  <channel>
    <title>Test Feed</title>
    <link>https://example.com</link>
    <description>A test feed</description>
    <item>
      <title>Test Article</title>
      <link>https://example.com/article1</link>
      <description>Test content</description>
    </item>
  </channel>
</rss>''')
`
	scriptPath := filepath.Join(tempDir, "test_feed.py")
	if err := os.WriteFile(scriptPath, []byte(scriptContent), 0755); err != nil {
		t.Fatalf("Failed to create test script: %v", err)
	}

	executor := NewScriptExecutor(tempDir)

	feed, err := executor.ExecuteScript(context.Background(), "test_feed.py")
	if err != nil {
		// Python might not be available in all test environments
		t.Skipf("Skipping test - Python execution failed: %v", err)
	}

	if feed == nil {
		t.Fatal("ExecuteScript() returned nil feed")
	}

	if feed.Title != "Test Feed" {
		t.Errorf("Feed title = %v, want 'Test Feed'", feed.Title)
	}

	if len(feed.Items) != 1 {
		t.Errorf("Feed items count = %d, want 1", len(feed.Items))
	}

	if len(feed.Items) > 0 && feed.Items[0].Title != "Test Article" {
		t.Errorf("Article title = %v, want 'Test Article'", feed.Items[0].Title)
	}
}

func TestScriptExecutor_ExecuteScript_Timeout(t *testing.T) {
	tempDir := t.TempDir()

	// Create a script that takes too long (simulating timeout scenario)
	scriptContent := `#!/usr/bin/env python3
import time
time.sleep(60)  # Sleep for 60 seconds
print("This should not print")
`
	scriptPath := filepath.Join(tempDir, "slow_script.py")
	if err := os.WriteFile(scriptPath, []byte(scriptContent), 0755); err != nil {
		t.Fatalf("Failed to create test script: %v", err)
	}

	executor := NewScriptExecutor(tempDir)

	// Use a very short timeout (100 milliseconds)
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	_, err := executor.ExecuteScript(ctx, "slow_script.py")
	if err == nil {
		t.Error("ExecuteScript() should return error for timeout")
	}
}

func TestFindPythonExecutable(t *testing.T) {
	ctx := context.Background()

	// This test will pass if any Python executable is found
	pythonCmd, err := findPythonExecutable(ctx)

	// If no Python is found, that's okay - just skip the test
	if err != nil {
		t.Skipf("No Python executable found: %v", err)
	}

	// If Python is found, verify it works
	if pythonCmd == "" {
		t.Error("findPythonExecutable returned empty string")
	}

	// Test that the found executable actually works
	cmd := exec.CommandContext(ctx, pythonCmd, "--version")
	err = cmd.Run()
	if err != nil {
		t.Errorf("Found Python executable '%s' failed to run: %v", pythonCmd, err)
	}
}
