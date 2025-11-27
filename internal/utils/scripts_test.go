package utils

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGetScriptsDir(t *testing.T) {
	dir, err := GetScriptsDir()
	if err != nil {
		t.Fatalf("GetScriptsDir() error: %v", err)
	}

	// Check that it's within the data directory
	dataDir, err := GetDataDir()
	if err != nil {
		t.Fatalf("GetDataDir() error: %v", err)
	}

	expectedDir := filepath.Join(dataDir, "scripts")
	if dir != expectedDir {
		t.Errorf("GetScriptsDir() = %v, want %v", dir, expectedDir)
	}

	// Check that the directory was created
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		t.Errorf("Scripts directory was not created: %v", dir)
	}
}

func TestValidateScriptPath_Valid(t *testing.T) {
	scriptsDir, err := GetScriptsDir()
	if err != nil {
		t.Fatalf("GetScriptsDir() error: %v", err)
	}

	// Create a test script file
	testScriptPath := filepath.Join(scriptsDir, "test_script.py")
	if err := os.WriteFile(testScriptPath, []byte("#!/usr/bin/env python3\nprint('test')"), 0755); err != nil {
		t.Fatalf("Failed to create test script: %v", err)
	}
	defer os.Remove(testScriptPath)

	// Validate the script path
	absPath, err := ValidateScriptPath("test_script.py")
	if err != nil {
		t.Errorf("ValidateScriptPath() error: %v", err)
	}

	if absPath != testScriptPath {
		t.Errorf("ValidateScriptPath() = %v, want %v", absPath, testScriptPath)
	}
}

func TestValidateScriptPath_NotExists(t *testing.T) {
	_, err := ValidateScriptPath("nonexistent_script.py")
	if !os.IsNotExist(err) {
		t.Errorf("ValidateScriptPath() should return os.ErrNotExist for non-existent script, got: %v", err)
	}
}

func TestValidateScriptPath_PathTraversal(t *testing.T) {
	_, err := ValidateScriptPath("../../../etc/passwd")
	if err != os.ErrPermission {
		t.Errorf("ValidateScriptPath() should return os.ErrPermission for path traversal, got: %v", err)
	}
}
