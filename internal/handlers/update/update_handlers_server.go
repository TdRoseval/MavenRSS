//go:build server

// Package update provides HTTP handlers for update operations (server mode).
package update

import (
	"log"
	"net/http"

	"MrRSS/internal/handlers/core"
	"MrRSS/internal/handlers/response"
	"MrRSS/internal/version"
)

// HandleCheckUpdates is not available in server mode.
// @Summary      Check for updates (not available in server mode)
// @Description  Auto-update is not available in server mode
// @Tags         update
// @Accept       json
// @Produce      json
// @Success      200  {object}  map[string]interface{}  "Version info (no update available)"
// @Router       /check-updates [get]
func HandleCheckUpdates(h *core.Handler, w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		response.Error(w, nil, http.StatusMethodNotAllowed)
		return
	}

	log.Printf("HandleCheckUpdates: Server mode - auto-update not available")
	response.JSON(w, map[string]interface{}{
		"current_version": version.Version,
		"has_update":      false,
		"server_mode":     true,
	})
}

// HandleDownloadUpdate is not available in server mode.
// @Summary      Download update (not available in server mode)
// @Description  Auto-update is not available in server mode
// @Tags         update
// @Accept       json
// @Produce      json
// @Success      501  {object}  map[string]interface{}  "Not implemented"
// @Router       /download-update [post]
func HandleDownloadUpdate(h *core.Handler, w http.ResponseWriter, r *http.Request) {
	log.Printf("HandleDownloadUpdate: Server mode - auto-update not available")
	w.WriteHeader(http.StatusNotImplemented)
	response.JSON(w, map[string]interface{}{
		"error": "Auto-update is not available in server mode",
	})
}

// HandleInstallUpdate is not available in server mode.
// @Summary      Install update (not available in server mode)
// @Description  Auto-update is not available in server mode
// @Tags         update
// @Accept       json
// @Produce      json
// @Success      501  {object}  map[string]interface{}  "Not implemented"
// @Router       /install-update [post]
func HandleInstallUpdate(h *core.Handler, w http.ResponseWriter, r *http.Request) {
	log.Printf("HandleInstallUpdate: Server mode - auto-update not available")
	w.WriteHeader(http.StatusNotImplemented)
	response.JSON(w, map[string]interface{}{
		"error": "Auto-update is not available in server mode",
	})
}

// HandleVersion returns the current version.
// @Summary      Get current version
// @Description  Returns the current application version
// @Tags         update
// @Accept       json
// @Produce      json
// @Success      200  {object}  map[string]string  "Version info (version)"
// @Router       /version [get]
func HandleVersion(h *core.Handler, w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		response.Error(w, nil, http.StatusMethodNotAllowed)
		return
	}

	response.JSON(w, map[string]string{
		"version":     version.Version,
		"server_mode": "true",
	})
}
