package utils

import (
	"os"
	"path/filepath"
	"runtime"
	"sync"
)

var (
	isPortableMode   bool
	portableModeOnce sync.Once
	isServerMode     bool
)

// SetServerMode sets the server mode flag
func SetServerMode(v bool) {
	isServerMode = v
}

// IsServerMode returns true if running in server mode
func IsServerMode() bool {
	return isServerMode
}

// IsPortableMode checks if the application is running in portable mode
// Portable mode is enabled if a "portable.txt" file exists in the executable's directory
func IsPortableMode() bool {
	portableModeOnce.Do(func() {
		exePath, err := os.Executable()
		if err != nil {
			isPortableMode = false
			return
		}

		exeDir := filepath.Dir(exePath)
		portableMarker := filepath.Join(exeDir, "portable.txt")

		_, err = os.Stat(portableMarker)
		isPortableMode = err == nil
	})
	return isPortableMode
}

// GetDataDir returns the platform-specific user data directory for MrRSS
// In portable mode, returns "data" directory next to the executable
// In normal mode, returns system-specific user data directory
func GetDataDir() (string, error) {
	var dataDir string
	var err error

	// Check if server mode is enabled - this takes precedence
	if IsServerMode() {
		// In server mode, use ./data directory (relative to working directory)
		return "./data", nil
	}

	// Check if portable mode is enabled
	if IsPortableMode() {
		// Portable mode: use "data" directory next to executable
		exePath, err := os.Executable()
		if err != nil {
			return "", err
		}
		exeDir := filepath.Dir(exePath)
		dataDir = filepath.Join(exeDir, "data")
	} else {
		// Normal mode: use platform-specific user data directory
		var baseDir string

		switch runtime.GOOS {
		case "windows":
			baseDir = os.Getenv("APPDATA")
			if baseDir == "" {
				baseDir = os.Getenv("USERPROFILE")
				if baseDir != "" {
					baseDir = filepath.Join(baseDir, "AppData", "Roaming")
				}
			}
		case "darwin":
			baseDir = os.Getenv("HOME")
			if baseDir != "" {
				baseDir = filepath.Join(baseDir, "Library", "Application Support")
			}
		case "linux":
			// Follow XDG Base Directory specification
			baseDir = os.Getenv("XDG_DATA_HOME")
			if baseDir == "" {
				homeDir := os.Getenv("HOME")
				if homeDir != "" {
					baseDir = filepath.Join(homeDir, ".local", "share")
				}
			}
		default:
			// Fallback for other platforms
			baseDir = os.Getenv("HOME")
			if baseDir != "" {
				baseDir = filepath.Join(baseDir, ".config")
			}
		}

		if baseDir == "" {
			// Last resort: use current directory
			baseDir, err = os.Getwd()
			if err != nil {
				return "", err
			}
		}

		// Create MrRSS subdirectory
		dataDir = filepath.Join(baseDir, "MrRSS")
	}

	// Create the data directory if it doesn't exist
	err = os.MkdirAll(dataDir, 0755)
	if err != nil {
		return "", err
	}

	return dataDir, nil
}

// GetDBPath returns the full path to the database file
func GetDBPath() (string, error) {
	dataDir, err := GetDataDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dataDir, "rss.db"), nil
}

// GetLogPath returns the full path to the debug log file
func GetLogPath() (string, error) {
	dataDir, err := GetDataDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dataDir, "debug.log"), nil
}

// GetMediaCacheDir returns the full path to the media cache directory
func GetMediaCacheDir() (string, error) {
	dataDir, err := GetDataDir()
	if err != nil {
		return "", err
	}
	cacheDir := filepath.Join(dataDir, "media_cache")
	err = os.MkdirAll(cacheDir, 0755)
	if err != nil {
		return "", err
	}
	return cacheDir, nil
}

// IsWindows returns true if the current platform is Windows
func IsWindows() bool {
	return runtime.GOOS == "windows"
}

// IsMacOS returns true if the current platform is MacOS (Darwin)
func IsMacOS() bool {
	return runtime.GOOS == "darwin"
}
