//go:build server

// Package browser provides HTTP handlers for browser-related operations (server mode).
package browser

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"

	handlers "MavenRSS/internal/handlers/core"
	"MavenRSS/internal/handlers/response"
)

// HandleOpenURL handles URL opening requests in server mode.
// In server mode, this returns a redirect response that instructs the client to open the URL.
// @Summary      Open URL in browser (server mode)
// @Description  Returns redirect response for client-side URL opening
// @Tags         browser
// @Accept       json
// @Produce      json
// @Param        url       query     string  false  "URL to open (for GET requests)"
// @Param        request  body      object  true  "Open URL request (url) (for POST requests)"
// @Success      200  {object}  map[string]string  "Redirect URL (redirect)"
// @Failure      400  {object}  map[string]string  "Bad request (invalid URL)"
// @Router       /browser/open [post]
// @Router       /browser/open [get]
func HandleOpenURL(h *handlers.Handler, w http.ResponseWriter, r *http.Request) {
	var targetURL string

	// Handle both GET and POST requests
	switch r.Method {
	case http.MethodGet:
		// Get URL from query parameter (for GET requests from proxied links)
		targetURL = r.URL.Query().Get("url")
		if targetURL == "" {
			response.Error(w, fmt.Errorf("URL is required"), http.StatusBadRequest)
			return
		}
	case http.MethodPost:
		// Parse request body (for POST requests)
		var req struct {
			URL string `json:"url"`
		}

		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			response.Error(w, err, http.StatusBadRequest)
			return
		}
		targetURL = req.URL
	default:
		response.Error(w, nil, http.StatusMethodNotAllowed)
		return
	}

	// Validate URL
	if targetURL == "" {
		response.Error(w, fmt.Errorf("URL is required"), http.StatusBadRequest)
		return
	}

	// Parse and validate URL scheme
	parsedURL, err := url.Parse(targetURL)
	if err != nil {
		log.Printf("Invalid URL format: %v", err)
		response.Error(w, fmt.Errorf("invalid URL format: %w", err), http.StatusBadRequest)
		return
	}

	// Only allow http and https schemes for security
	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		log.Printf("Invalid URL scheme: %s", parsedURL.Scheme)
		response.Error(w, fmt.Errorf("only HTTP and HTTPS URLs are allowed"), http.StatusBadRequest)
		return
	}

	// Server mode: return redirect response for client-side handling
	log.Printf("Server mode detected, instructing client to open URL: %s", targetURL)
	w.WriteHeader(http.StatusOK)
	response.JSON(w, map[string]string{"redirect": targetURL})
}
