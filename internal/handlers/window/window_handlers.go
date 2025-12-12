package window

import (
	"encoding/json"
	"fmt"
	"net/http"

	"MrRSS/internal/handlers/core"
)

// WindowState represents the saved window state
type WindowState struct {
	X         int  `json:"x"`
	Y         int  `json:"y"`
	Width     int  `json:"width"`
	Height    int  `json:"height"`
	Maximized bool `json:"maximized"`
}

// HandleGetWindowState returns the saved window state
func HandleGetWindowState(h *core.Handler, w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	x, err := h.DB.GetSetting("window_x")
	if err != nil {
		http.Error(w, "Failed to get window state", http.StatusInternalServerError)
		return
	}
	y, _ := h.DB.GetSetting("window_y")
	width, _ := h.DB.GetSetting("window_width")
	height, _ := h.DB.GetSetting("window_height")
	maximized, _ := h.DB.GetSetting("window_maximized")

	json.NewEncoder(w).Encode(map[string]string{
		"x":         x,
		"y":         y,
		"width":     width,
		"height":    height,
		"maximized": maximized,
	})
}

// HandleSaveWindowState saves the current window state
func HandleSaveWindowState(h *core.Handler, w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var state WindowState
	if err := json.NewDecoder(r.Body).Decode(&state); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Convert to strings for database storage and check for errors
	if err := h.DB.SetSetting("window_x", fmt.Sprintf("%d", state.X)); err != nil {
		http.Error(w, "Failed to save window state", http.StatusInternalServerError)
		return
	}
	if err := h.DB.SetSetting("window_y", fmt.Sprintf("%d", state.Y)); err != nil {
		http.Error(w, "Failed to save window state", http.StatusInternalServerError)
		return
	}
	if err := h.DB.SetSetting("window_width", fmt.Sprintf("%d", state.Width)); err != nil {
		http.Error(w, "Failed to save window state", http.StatusInternalServerError)
		return
	}
	if err := h.DB.SetSetting("window_height", fmt.Sprintf("%d", state.Height)); err != nil {
		http.Error(w, "Failed to save window state", http.StatusInternalServerError)
		return
	}
	if err := h.DB.SetSetting("window_maximized", fmt.Sprintf("%t", state.Maximized)); err != nil {
		http.Error(w, "Failed to save window state", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}
