package freshrss

import (
	"context"
	"encoding/json"
	"log"
	"net/http"

	"MrRSS/internal/freshrss"
	"MrRSS/internal/handlers/core"
)

// HandleSync performs synchronization with FreshRSS server
func HandleSync(h *core.Handler, w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get FreshRSS settings
	enabled, err := h.DB.GetSetting("freshrss_enabled")
	if err != nil {
		log.Printf("Error getting freshrss_enabled: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	if enabled != "true" {
		http.Error(w, "FreshRSS sync is disabled", http.StatusBadRequest)
		return
	}

	serverURL, _ := h.DB.GetSetting("freshrss_server_url")
	username, _ := h.DB.GetSetting("freshrss_username")
	password, _ := h.DB.GetEncryptedSetting("freshrss_api_password")

	if serverURL == "" || username == "" || password == "" {
		http.Error(w, "FreshRSS settings incomplete", http.StatusBadRequest)
		return
	}

	// Create sync service
	syncService := freshrss.NewSyncService(serverURL, username, password, h.DB)

	// Perform sync in background
	go func() {
		ctx := context.Background()
		if err := syncService.Sync(ctx); err != nil {
			log.Printf("FreshRSS sync failed: %v", err)
		} else {
			log.Printf("FreshRSS sync completed successfully")
			// Trigger a refresh of all feeds to update the article list
			go h.Fetcher.FetchAll(context.Background())
		}
	}()

	// Return success response
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status":  "sync_started",
		"message": "FreshRSS synchronization started",
	})
}

// HandleTestConnection tests the connection to FreshRSS server
func HandleTestConnection(h *core.Handler, w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse request body
	var req struct {
		ServerURL   string `json:"server_url"`
		Username    string `json:"username"`
		APIPassword string `json:"api_password"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   "Invalid request body",
		})
		return
	}

	// Validate required fields
	if req.ServerURL == "" || req.Username == "" || req.APIPassword == "" {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   "FreshRSS settings incomplete",
		})
		return
	}

	serverURL := req.ServerURL
	username := req.Username
	password := req.APIPassword

	// Test connection
	client := freshrss.NewClient(serverURL, username, password)
	ctx := context.Background()

	err := client.Login(ctx)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	// Get subscription count
	subscriptions, err := client.GetSubscriptions(ctx)
	subscriptionCount := 0
	if err == nil {
		subscriptionCount = len(subscriptions)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success":           true,
		"subscriptionCount": subscriptionCount,
		"message":           "Connection successful",
	})
}
