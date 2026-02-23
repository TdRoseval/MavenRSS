//go:build server

// Package script provides HTTP handlers for script operations (server mode).
package script

import (
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"MavenRSS/internal/handlers/core"
	"MavenRSS/internal/handlers/response"
	"MavenRSS/internal/utils/fileutil"
)

// HandleGetScriptsDir returns the path to the scripts directory.
// @Summary      Get scripts directory path
// @Description  Get the file system path to the scripts directory
// @Tags         scripts
// @Accept       json
// @Produce      json
// @Success      200  {object}  map[string]string  "Scripts directory path (scripts_dir)"
// @Failure      500  {object}  map[string]string  "Internal server error"
// @Router       /scripts/dir [get]
func HandleGetScriptsDir(h *core.Handler, w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		response.Error(w, nil, http.StatusMethodNotAllowed)
		return
	}

	scriptsDir, err := fileutil.GetScriptsDir()
	if err != nil {
		response.Error(w, err, http.StatusInternalServerError)
		return
	}

	response.JSON(w, map[string]string{
		"scripts_dir": scriptsDir,
	})
}

// HandleOpenScriptsDir is not available in server mode.
// @Summary      Open scripts directory (not available)
// @Description  Opening file explorer is not available in server mode
// @Tags         scripts
// @Accept       json
// @Produce      json
// @Success      501  {object}  map[string]string  "Not implemented"
// @Router       /scripts/dir/open [post]
func HandleOpenScriptsDir(h *core.Handler, w http.ResponseWriter, r *http.Request) {
	log.Printf("HandleOpenScriptsDir: Server mode - file explorer not available")
	w.WriteHeader(http.StatusNotImplemented)
	response.JSON(w, map[string]string{
		"error": "Opening file explorer is not available in server mode. Please access scripts directory via the server's file system directly.",
	})
}

// HandleListScripts returns a list of available scripts in the scripts directory.
// @Summary      List available scripts
// @Description  Get a list of all available scripts in the scripts directory
// @Tags         scripts
// @Accept       json
// @Produce      json
// @Success      200  {object}  map[string]interface{}  "List of scripts"
// @Failure      500  {object}  map[string]string  "Internal server error"
// @Router       /scripts/list [get]
func HandleListScripts(h *core.Handler, w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		response.Error(w, nil, http.StatusMethodNotAllowed)
		return
	}

	scriptsDir, err := fileutil.GetScriptsDir()
	if err != nil {
		response.Error(w, err, http.StatusInternalServerError)
		return
	}

	validExtensions := map[string]bool{
		".py":  true,
		".sh":  true,
		".ps1": true,
		".js":  true,
		".rb":  true,
	}

	var scripts []map[string]string

	err = filepath.Walk(scriptsDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		relPath, err := filepath.Rel(scriptsDir, path)
		if err != nil {
			return err
		}

		ext := strings.ToLower(filepath.Ext(path))
		scriptType := ""

		if validExtensions[ext] {
			switch ext {
			case ".py":
				scriptType = "Python"
			case ".sh":
				scriptType = "Shell"
			case ".ps1":
				scriptType = "PowerShell"
			case ".js":
				scriptType = "Node.js"
			case ".rb":
				scriptType = "Ruby"
			}

			scripts = append(scripts, map[string]string{
				"name": info.Name(),
				"path": relPath,
				"type": scriptType,
			})
		}

		return nil
	})

	if err != nil {
		response.Error(w, err, http.StatusInternalServerError)
		return
	}

	if scripts == nil {
		scripts = []map[string]string{}
	}

	response.JSON(w, map[string]interface{}{
		"scripts":     scripts,
		"scripts_dir": scriptsDir,
	})
}
