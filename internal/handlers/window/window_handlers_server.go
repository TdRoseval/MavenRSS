//go:build server

// Package window provides HTTP handlers for window operations (server mode).
package window

import (
	"log"
	"net/http"

	"MrRSS/internal/handlers/core"
	"MrRSS/internal/handlers/response"
)

// HandleGetWindowState returns empty state in server mode.
// @Summary      Get window state (server mode)
// @Description  Window state is not applicable in server mode
// @Tags         window
// @Accept       json
// @Produce      json
// @Success      200  {object}  map[string]string  "Empty window state"
// @Router       /window/state [get]
func HandleGetWindowState(h *core.Handler, w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		response.Error(w, nil, http.StatusMethodNotAllowed)
		return
	}

	log.Printf("HandleGetWindowState: Server mode - window state not applicable")
	// Return empty state
	response.JSON(w, map[string]string{
		"x":         "",
		"y":         "",
		"width":     "",
		"height":    "",
		"maximized": "",
	})
}

// HandleSaveWindowState is not available in server mode.
// @Summary      Save window state (not available)
// @Description  Window state saving is not applicable in server mode
// @Tags         window
// @Accept       json
// @Produce      json
// @Success      200  {object}  map[string]string  "Success (no-op)"
// @Router       /window/save [post]
func HandleSaveWindowState(h *core.Handler, w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		response.Error(w, nil, http.StatusMethodNotAllowed)
		return
	}

	log.Printf("HandleSaveWindowState: Server mode - window state not applicable")
	// Return success as a no-op
	response.JSON(w, map[string]string{"status": "ok"})
}
