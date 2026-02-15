package settings

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"

	"MrRSS/internal/handlers/core"
	"MrRSS/internal/handlers/response"
	"MrRSS/internal/translation"
	"MrRSS/internal/utils/httputil"
)

// safeGetEncryptedSetting safely retrieves an encrypted setting, returning empty string on error.
// This prevents JSON encoding errors when encrypted data is corrupted or cannot be decrypted.
func safeGetEncryptedSetting(h *core.Handler, key string) string {
	value, err := h.DB.GetEncryptedSetting(key)
	if err != nil {
		log.Printf("Warning: Failed to decrypt setting %s: %v. Returning empty string.", key, err)
		return ""
	}
	return sanitizeValue(value)
}

// safeGetSetting safely retrieves a setting, returning empty string on error.
func safeGetSetting(h *core.Handler, key string) string {
	value, err := h.DB.GetSetting(key)
	if err != nil {
		log.Printf("Warning: Failed to retrieve setting %s: %v. Returning empty string.", key, err)
		return ""
	}
	return sanitizeValue(value)
}

// sanitizeValue removes control characters that could break JSON encoding.
func sanitizeValue(value string) string {
	// Remove control characters that could break JSON
	return strings.Map(func(r rune) rune {
		if r < 32 && r != '\t' && r != '\n' && r != '\r' {
			return -1 // Remove control characters except tab, newline, carriage return
		}
		return r
	}, value)
}

// HandleSettings handles GET and POST requests for application settings.
// Uses the definition-driven approach from settings_base.go for cleaner code.
func HandleSettings(h *core.Handler, w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		// Get all settings using the definition-driven approach
		settings := GetAllSettings(h)
		response.JSON(w, settings)

	case http.MethodPost:
		// Parse request body as a generic map
		var req map[string]string
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			response.Error(w, err, http.StatusBadRequest)
			return
		}

		// Check if we're disabling FreshRSS
		if newEnabled, ok := req["freshrss_enabled"]; ok {
			oldEnabled, _ := h.DB.GetSetting("freshrss_enabled")
			if oldEnabled == "true" && newEnabled != "true" {
				// Cleanup FreshRSS data when disabling
				log.Printf("[HandleSettings] FreshRSS disabled, cleaning up data...")
				if err := h.DB.CleanupFreshRSSData(); err != nil {
					log.Printf("[HandleSettings] Failed to cleanup FreshRSS data: %v", err)
				}
			}
		}

		// Check if proxy settings are changing
		oldProxyType, _ := h.DB.GetSetting("proxy_type")
		oldProxyHost, _ := h.DB.GetSetting("proxy_host")
		oldProxyPort, _ := h.DB.GetSetting("proxy_port")
		oldProxyUsername, _ := h.DB.GetEncryptedSetting("proxy_username")
		oldProxyPassword, _ := h.DB.GetEncryptedSetting("proxy_password")
		oldProxyURL := httputil.BuildProxyURL(oldProxyType, oldProxyHost, oldProxyPort, oldProxyUsername, oldProxyPassword)

		// Save settings using the definition-driven approach
		if err := SaveSettings(h, req); err != nil {
			log.Printf("Failed to save settings: %v", err)
			response.Error(w, err, http.StatusInternalServerError)
			return
		}

		// Check if proxy settings changed and refresh connection pool
		newProxyEnabled, _ := h.DB.GetSetting("proxy_enabled")
		var newProxyURL string
		if newProxyEnabled == "true" {
			newProxyType, _ := h.DB.GetSetting("proxy_type")
			newProxyHost, _ := h.DB.GetSetting("proxy_host")
			newProxyPort, _ := h.DB.GetSetting("proxy_port")
			newProxyUsername, _ := h.DB.GetEncryptedSetting("proxy_username")
			newProxyPassword, _ := h.DB.GetEncryptedSetting("proxy_password")
			newProxyURL = httputil.BuildProxyURL(newProxyType, newProxyHost, newProxyPort, newProxyUsername, newProxyPassword)
		}

		if oldProxyURL != newProxyURL {
			log.Printf("[HandleSettings] Proxy settings changed, refreshing connection pool...")
			httputil.RefreshProxyClients(newProxyURL)

			// Refresh translator proxy
			if refresher, ok := h.Translator.(translation.ProxyRefresher); ok {
				refresher.InvalidateCache()
				log.Printf("[HandleSettings] Translator cache invalidated for proxy refresh")
			}

			// Also call RefreshProxy if available to refresh individual translator HTTP clients
			if refresherWithProxy, ok := h.Translator.(interface{ RefreshProxy() }); ok {
				refresherWithProxy.RefreshProxy()
				log.Printf("[HandleSettings] Translator proxy refreshed via RefreshProxy()")
			}

			// Refresh discovery service proxy
			if h.DiscoveryService != nil {
				h.DiscoveryService.SetProxy(newProxyURL)
				log.Printf("[HandleSettings] Discovery service proxy refreshed")
			}
		}

		// Re-fetch all settings after save to return updated values
		settings := GetAllSettings(h)
		response.JSON(w, settings)

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}
