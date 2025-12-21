// Package browser provides HTTP handlers for browser-related operations using Wails v3 Browser API.
package browser

import (
	"encoding/json"
	"log"
	"net/http"
	"net/url"

	handlers "MrRSS/internal/handlers/core"
	"MrRSS/internal/utils"
)

// HandleOpenURL opens a URL in the user's default web browser using Wails v3 Browser API.
//
// Request: POST /api/browser/open
// Body: {"url": "https://example.com"}
// Response: 200 OK or 400/500 on error
func HandleOpenURL(h *handlers.Handler, w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse request body
	var req struct {
		URL string `json:"url"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validate URL
	if req.URL == "" {
		http.Error(w, "URL is required", http.StatusBadRequest)
		return
	}

	// Parse and validate URL scheme
	parsedURL, err := url.Parse(req.URL)
	if err != nil {
		log.Printf("Invalid URL format: %v", err)
		http.Error(w, "Invalid URL format", http.StatusBadRequest)
		return
	}

	// Only allow http and https schemes for security
	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		log.Printf("Invalid URL scheme: %s", parsedURL.Scheme)
		http.Error(w, "Only HTTP and HTTPS URLs are allowed", http.StatusBadRequest)
		return
	}

	// Check if app instance is available
	if h.App == nil {
		// specific check for server mode to redirect to client side
		if utils.IsServerMode() {
			log.Printf("Server mode detected, instructing client to open URL: %s", req.URL)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]string{"redirect": req.URL})
			return
		}

		log.Printf("App instance not available for browser integration")
		http.Error(w, "Browser integration not available", http.StatusInternalServerError)
		return
	}

	// Open URL using Wails v3 Browser API
	err = h.App.Browser.OpenURL(req.URL)
	if err != nil {
		log.Printf("Failed to open URL in browser: %v", err)
		http.Error(w, "Failed to open URL in browser", http.StatusInternalServerError)
		return
	}

	log.Printf("Opened URL in browser: %s", req.URL)

	// Return success
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "success"})
}
