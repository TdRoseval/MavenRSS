package utils

import (
	"os"
	"path/filepath"
	"strings"
)

// GetScriptsDir returns the path to the scripts directory within the data directory
func GetScriptsDir() (string, error) {
	dataDir, err := GetDataDir()
	if err != nil {
		return "", err
	}
	scriptsDir := filepath.Join(dataDir, "scripts")
	// Create the scripts directory if it doesn't exist
	err = os.MkdirAll(scriptsDir, 0755)
	if err != nil {
		return "", err
	}
	return scriptsDir, nil
}

// ValidateScriptPath validates that a script path is within the scripts directory
// and the script file exists
func ValidateScriptPath(scriptPath string) (string, error) {
	scriptsDir, err := GetScriptsDir()
	if err != nil {
		return "", err
	}

	// Get the absolute path of the script
	absScriptPath := filepath.Join(scriptsDir, scriptPath)
	absScriptPath = filepath.Clean(absScriptPath)

	// Clean the scripts directory path for comparison
	cleanScriptsDir := filepath.Clean(scriptsDir) + string(filepath.Separator)

	// Ensure the script path is within the scripts directory (prevent path traversal)
	// Use strings.HasPrefix on cleaned paths with trailing separator to prevent bypasses
	if !strings.HasPrefix(absScriptPath+string(filepath.Separator), cleanScriptsDir) &&
		!strings.HasPrefix(absScriptPath, cleanScriptsDir) {
		return "", os.ErrPermission
	}

	// Check if the file exists
	if _, err := os.Stat(absScriptPath); os.IsNotExist(err) {
		return "", os.ErrNotExist
	}

	return absScriptPath, nil
}
