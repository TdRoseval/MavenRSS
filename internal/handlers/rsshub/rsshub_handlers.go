package rsshub

import (
	"encoding/json"
	"fmt"
	"net/http"

	"MrRSS/internal/handlers/core"
	"MrRSS/internal/handlers/response"
	"MrRSS/internal/rsshub"
	"MrRSS/internal/utils/httputil"
)

// HandleAddFeed adds a new RSSHub feed subscription
//
//	@Summary		Add RSSHub feed
//	@Description	Adds a new RSSHub feed subscription with the specified route, category, and title
//	@Tags			feeds
//	@Accept			json
//	@Produce		json
//	@Param			request	body		object{route=string,category=string,title=string}	true	"RSSHub feed details"
//	@Success		200		{object}	object{success=bool,feed_id=int64}				"Feed added successfully"
//	@Failure		400		{object}	object{error=string}								"Invalid request"
//	@Failure		500		{object}	object{error=string}								"Server error"
//	@Router			/api/rsshub/add [post]
func HandleAddFeed(h *core.Handler, w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		response.Error(w, nil, http.StatusMethodNotAllowed)
		return
	}

	userID, _ := core.GetUserIDFromRequest(r)

	var req struct {
		Route    string `json:"route"`
		Category string `json:"category"`
		Title    string `json:"title"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, err, http.StatusBadRequest)
		return
	}

	// Validate route
	if req.Route == "" {
		response.Error(w, fmt.Errorf("route is required"), http.StatusBadRequest)
		return
	}

	// Add RSSHub subscription using specialized handler
	feedID, err := h.Fetcher.AddRSSHubSubscriptionWithUserID(req.Route, req.Category, req.Title, userID)
	if err != nil {
		response.Error(w, err, http.StatusInternalServerError)
		return
	}

	response.JSON(w, map[string]interface{}{
		"success": true,
		"feed_id": feedID,
	})
}

// HandleTestConnection tests the RSSHub endpoint and API key
//
//	@Summary		Test RSSHub connection
//	@Description	Tests the connection to RSSHub endpoint with the provided API key by validating the endpoint
//	@Tags			rsshub
//	@Accept			json
//	@Produce		json
//	@Param			request	body		object{endpoint=string,api_key=string}	true	"RSSHub connection details"
//	@Success		200		{object}	object{success=bool,message=string}	"Connection successful"
//	@Failure		200		{object}	object{success=bool,error=string}		"Connection failed"
//	@Router			/api/rsshub/test-connection [post]
func HandleTestConnection(h *core.Handler, w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		response.Error(w, nil, http.StatusMethodNotAllowed)
		return
	}

	userID, _ := core.GetUserIDFromRequest(r)

	var req struct {
		Endpoint string `json:"endpoint"`
		APIKey   string `json:"api_key"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.JSON(w, map[string]interface{}{
			"success": false,
			"error":   "Invalid request body",
		})
		return
	}

	// Validate endpoint
	if req.Endpoint == "" {
		response.JSON(w, map[string]interface{}{
			"success": false,
			"error":   "Endpoint is required",
		})
		return
	}

	// Build proxy URL if enabled (优先用户设置，回退全局)
	var proxyURL string
	proxyEnabled, _ := h.DB.GetSettingWithFallback(userID, "proxy_enabled")
	if proxyEnabled == "true" {
		proxyType, _ := h.DB.GetSettingWithFallback(userID, "proxy_type")
		proxyHost, _ := h.DB.GetSettingWithFallback(userID, "proxy_host")
		proxyPort, _ := h.DB.GetSettingWithFallback(userID, "proxy_port")
		proxyUsername, _ := h.DB.GetEncryptedSettingWithFallback(userID, "proxy_username")
		proxyPassword, _ := h.DB.GetEncryptedSettingWithFallback(userID, "proxy_password")
		proxyURL = httputil.BuildProxyURL(proxyType, proxyHost, proxyPort, proxyUsername, proxyPassword)
	}

	client := rsshub.NewClientWithProxy(req.Endpoint, req.APIKey, proxyURL)
	err := client.TestEndpoint()

	if err != nil {
		response.JSON(w, map[string]interface{}{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	response.JSON(w, map[string]interface{}{
		"success": true,
		"message": "Connection successful",
	})
}

// HandleValidateRoute validates a specific RSSHub route
//
//	@Summary		Validate RSSHub route
//	@Description	Validates if a specific RSSHub route exists and is accessible using the configured endpoint and API key
//	@Tags			rsshub
//	@Accept			json
//	@Produce		json
//	@Param			request	body		object{route=string}	true	"Route to validate"
//	@Success		200		{object}	object{valid=bool,message=string}	"Route is valid"
//	@Failure		200		{object}	object{valid=bool,error=string}		"Route is invalid"
//	@Router			/api/rsshub/validate-route [post]
func HandleValidateRoute(h *core.Handler, w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		response.Error(w, nil, http.StatusMethodNotAllowed)
		return
	}

	userID, _ := core.GetUserIDFromRequest(r)

	var req struct {
		Route string `json:"route"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, err, http.StatusBadRequest)
		return
	}

	if req.Route == "" {
		response.Error(w, fmt.Errorf("route is required"), http.StatusBadRequest)
		return
	}

	// Get RSSHub settings (优先用户设置，回退全局)
	endpoint, _ := h.DB.GetSettingWithFallback(userID, "rsshub_endpoint")
	if endpoint == "" {
		endpoint = "https://rsshub.app"
	}
	apiKey, _ := h.DB.GetEncryptedSettingWithFallback(userID, "rsshub_api_key")

	// Build proxy URL if enabled (优先用户设置，回退全局)
	var proxyURL string
	proxyEnabled, _ := h.DB.GetSettingWithFallback(userID, "proxy_enabled")
	if proxyEnabled == "true" {
		proxyType, _ := h.DB.GetSettingWithFallback(userID, "proxy_type")
		proxyHost, _ := h.DB.GetSettingWithFallback(userID, "proxy_host")
		proxyPort, _ := h.DB.GetSettingWithFallback(userID, "proxy_port")
		proxyUsername, _ := h.DB.GetEncryptedSettingWithFallback(userID, "proxy_username")
		proxyPassword, _ := h.DB.GetEncryptedSettingWithFallback(userID, "proxy_password")
		proxyURL = httputil.BuildProxyURL(proxyType, proxyHost, proxyPort, proxyUsername, proxyPassword)
	}

	client := rsshub.NewClientWithProxy(endpoint, apiKey, proxyURL)
	err := client.ValidateRoute(req.Route)

	if err != nil {
		response.JSON(w, map[string]interface{}{
			"valid": false,
			"error": err.Error(),
		})
		return
	}

	response.JSON(w, map[string]interface{}{
		"valid":   true,
		"message": "Route is valid",
	})
}

// HandleTransformURL transforms a rsshub:// URL to full RSSHub URL
//
//	@Summary		Transform RSSHub URL
//	@Description	Transforms a rsshub:// protocol URL to full RSSHub URL with endpoint and API key
//	@Tags			rsshub
//	@Accept			json
//	@Produce		json
//	@Param			request	body		object{url=string}	true	"RSSHub URL to transform (rsshub:// protocol)"
//	@Success		200		{object}	object{url=string}	"Transformed URL"
//	@Failure		400		{object}	object{error=string}	"Invalid request or URL"
//	@Router			/api/rsshub/transform-url [post]
func HandleTransformURL(h *core.Handler, w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		response.Error(w, nil, http.StatusMethodNotAllowed)
		return
	}

	userID, _ := core.GetUserIDFromRequest(r)

	var req struct {
		URL string `json:"url"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, err, http.StatusBadRequest)
		return
	}

	if req.URL == "" {
		response.Error(w, fmt.Errorf("url is required"), http.StatusBadRequest)
		return
	}

	// Check if it's a RSSHub URL
	if !rsshub.IsRSSHubURL(req.URL) {
		response.JSON(w, map[string]interface{}{
			"url": req.URL,
		})
		return
	}

	// Check if RSSHub is enabled (优先用户设置，回退全局)
	enabledStr, _ := h.DB.GetSettingWithFallback(userID, "rsshub_enabled")
	if enabledStr != "true" {
		response.Error(w, fmt.Errorf("RSSHub integration is disabled"), http.StatusBadRequest)
		return
	}

	// Get RSSHub settings (优先用户设置，回退全局)
	endpoint, _ := h.DB.GetSettingWithFallback(userID, "rsshub_endpoint")
	if endpoint == "" {
		endpoint = "https://rsshub.app"
	}
	apiKey, _ := h.DB.GetEncryptedSettingWithFallback(userID, "rsshub_api_key")

	// Build proxy URL if enabled (优先用户设置，回退全局)
	var proxyURL string
	proxyEnabled, _ := h.DB.GetSettingWithFallback(userID, "proxy_enabled")
	if proxyEnabled == "true" {
		proxyType, _ := h.DB.GetSettingWithFallback(userID, "proxy_type")
		proxyHost, _ := h.DB.GetSettingWithFallback(userID, "proxy_host")
		proxyPort, _ := h.DB.GetSettingWithFallback(userID, "proxy_port")
		proxyUsername, _ := h.DB.GetEncryptedSettingWithFallback(userID, "proxy_username")
		proxyPassword, _ := h.DB.GetEncryptedSettingWithFallback(userID, "proxy_password")
		proxyURL = httputil.BuildProxyURL(proxyType, proxyHost, proxyPort, proxyUsername, proxyPassword)
	}

	// Extract route and build URL
	route := rsshub.ExtractRoute(req.URL)
	client := rsshub.NewClientWithProxy(endpoint, apiKey, proxyURL)
	transformedURL := client.BuildURL(route)

	response.JSON(w, map[string]interface{}{
		"url": transformedURL,
	})
}
