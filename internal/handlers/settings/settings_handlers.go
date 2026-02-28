package settings

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"

	"MavenRSS/internal/handlers/core"
	"MavenRSS/internal/handlers/response"
	"MavenRSS/internal/middleware"
	"MavenRSS/internal/translation"
	"MavenRSS/internal/utils/httputil"
)

// isSensitiveSetting checks if a setting is a sensitive configuration
// that should be hidden from non-admin users when inherited from template
func isSensitiveSetting(key string) bool {
	sensitivePrefixes := []string{"ai_", "proxy_", "rsshub_"}
	for _, prefix := range sensitivePrefixes {
		if strings.HasPrefix(key, prefix) {
			return true
		}
	}
	return false
}

// isAdmin checks if the user is an admin
func isAdmin(r *http.Request) bool {
	claims, ok := middleware.GetUserFromContext(r.Context())
	return ok && claims.Role == "admin"
}

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

// safeGetEncryptedSettingForUser safely retrieves an encrypted setting for a specific user, returning empty string on error.
func safeGetEncryptedSettingForUser(h *core.Handler, userID int64, key string) string {
	value, err := h.DB.GetEncryptedSettingForUser(userID, key)
	if err != nil {
		log.Printf("Warning: Failed to decrypt setting %s for user %d: %v. Returning empty string.", key, userID, err)
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

// safeGetSettingForUser safely retrieves a setting for a specific user, returning empty string on error.
func safeGetSettingForUser(h *core.Handler, userID int64, key string) string {
	value, err := h.DB.GetSettingForUser(userID, key)
	if err != nil {
		log.Printf("Warning: Failed to retrieve setting %s for user %d: %v. Returning empty string.", key, userID, err)
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

// GetAllSettingsForUser reads all settings for a specific user from the database and returns them as a map.
// Encrypted settings are automatically decrypted.
// If a setting is empty for the user, falls back to global setting.
// If global setting is also empty, falls back to default value from config package.
func GetAllSettingsForUser(h *core.Handler, userID int64, isAdmin bool, hasInherited bool) map[string]string {
	result := make(map[string]string, len(AllSettings)+1)
	globalSettings := GetAllSettings(h)

	// Add hasInherited flag to result so frontend knows
	if hasInherited {
		result["_has_inherited"] = "true"
	} else {
		result["_has_inherited"] = "false"
	}

	for _, def := range AllSettings {
		var finalValue string

		// Regular setting - check user setting first, then fall back to global
		var userValue string
		if def.Encrypted {
			userValue = safeGetEncryptedSettingForUser(h, userID, def.Key)
		} else {
			userValue = safeGetSettingForUser(h, userID, def.Key)
		}

		if userValue == "" {
			finalValue = globalSettings[def.Key]
		} else {
			finalValue = userValue
		}

		result[def.Key] = finalValue
	}

	return result
}

// SaveSettingsForUser saves settings for a specific user from a map to the database.
// Empty string values are skipped (to allow partial updates).
// Encrypted settings are automatically encrypted.
func SaveSettingsForUser(h *core.Handler, userID int64, settings map[string]string) error {
	// Create a lookup for encrypted keys
	encryptedKeys := make(map[string]bool, len(AllSettings))
	for _, def := range AllSettings {
		if def.Encrypted {
			encryptedKeys[def.Key] = true
		}
	}

	// Check if ai_usage_limit is being updated
	if userLimitStr, hasUserLimit := settings["ai_usage_limit"]; hasUserLimit {
		// Get the hard limit for this user
		hardLimitStr, _ := h.DB.GetSettingForUser(userID, "ai_usage_hard_limit")
		if hardLimitStr == "" {
			hardLimitStr, _ = h.DB.GetSetting("ai_usage_hard_limit")
		}
		
		// Parse hard limit (0 means no limit)
		var hardLimit int64 = 0
		if hardLimitStr != "" {
			fmt.Sscanf(hardLimitStr, "%d", &hardLimit)
		}
		
		// Parse user limit
		var userLimit int64 = 0
		if userLimitStr != "" {
			fmt.Sscanf(userLimitStr, "%d", &userLimit)
		}
		
		// Only enforce hard limit if it's set (not 0)
		if hardLimit > 0 {
			// Check if user is trying to set 0 (unlimited) or exceeding hard limit
			if userLimit == 0 {
				return fmt.Errorf("无法设置为无限制，管理员已设置硬上限为 %d tokens", hardLimit)
			}
			if userLimit > hardLimit {
				return fmt.Errorf("设置值 %d 超过了管理员设置的硬上限 %d", userLimit, hardLimit)
			}
		}
	}

	// Save each setting
	for key, value := range settings {
		if encryptedKeys[key] {
			if err := h.DB.SetEncryptedSettingForUser(userID, key, value); err != nil {
				return err
			}
		} else if value != "" {
			h.DB.SetSettingForUser(userID, key, value)
		}
	}

	return nil
}

// HandleSettings handles GET and POST requests for application settings.
// Uses the definition-driven approach from settings_base.go for cleaner code.
func HandleSettings(h *core.Handler, w http.ResponseWriter, r *http.Request) {
	userID, ok := core.GetUserIDFromRequest(r)

	// Get user information for sensitive settings filtering
	isAdmin := isAdmin(r)
	hasInherited := false
	if ok {
		user, err := h.DB.GetUserByID(userID)
		if err == nil {
			hasInherited = user.HasInherited
		}
	}

	switch r.Method {
	case http.MethodGet:
		// Get all settings using the definition-driven approach
		// If we have a user ID, return user settings, otherwise return global settings
		var settings map[string]string
		if ok {
			settings = GetAllSettingsForUser(h, userID, isAdmin, hasInherited)
		} else {
			settings = GetAllSettings(h)
		}
		response.JSON(w, settings)

	case http.MethodPost:
		// Parse request body as a generic map
		var req map[string]string
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			response.Error(w, err, http.StatusBadRequest)
			return
		}

		// Check if we're disabling FreshRSS
		if newEnabled, okFresh := req["freshrss_enabled"]; okFresh {
			var oldEnabled string
			if ok {
				oldEnabled, _ = h.DB.GetSettingForUser(userID, "freshrss_enabled")
			} else {
				oldEnabled, _ = h.DB.GetSetting("freshrss_enabled")
			}
			if oldEnabled == "true" && newEnabled != "true" {
				// Cleanup FreshRSS data when disabling
				log.Printf("[HandleSettings] FreshRSS disabled, cleaning up data...")
				if err := h.DB.CleanupFreshRSSData(); err != nil {
					log.Printf("[HandleSettings] Failed to cleanup FreshRSS data: %v", err)
				}
			}
		}

		// Check if proxy settings are changing
		var oldProxyType, oldProxyHost, oldProxyPort, oldProxyUsername, oldProxyPassword string
		if ok {
			oldProxyType, _ = h.DB.GetSettingForUser(userID, "proxy_type")
			oldProxyHost, _ = h.DB.GetSettingForUser(userID, "proxy_host")
			oldProxyPort, _ = h.DB.GetSettingForUser(userID, "proxy_port")
			oldProxyUsername, _ = h.DB.GetEncryptedSettingForUser(userID, "proxy_username")
			oldProxyPassword, _ = h.DB.GetEncryptedSettingForUser(userID, "proxy_password")
		} else {
			oldProxyType, _ = h.DB.GetSetting("proxy_type")
			oldProxyHost, _ = h.DB.GetSetting("proxy_host")
			oldProxyPort, _ = h.DB.GetSetting("proxy_port")
			oldProxyUsername, _ = h.DB.GetEncryptedSetting("proxy_username")
			oldProxyPassword, _ = h.DB.GetEncryptedSetting("proxy_password")
		}
		oldProxyURL := httputil.BuildProxyURL(oldProxyType, oldProxyHost, oldProxyPort, oldProxyUsername, oldProxyPassword)

		// Save settings - only to user settings if authenticated
		var err error
		if ok {
			// Save to user settings
			err = SaveSettingsForUser(h, userID, req)
			if err != nil {
				log.Printf("Failed to save user settings: %v", err)
				response.Error(w, err, http.StatusInternalServerError)
				return
			}
		} else {
			// If no user, save to global settings
			err = SaveSettings(h, req)
			if err != nil {
				log.Printf("Failed to save global settings: %v", err)
				response.Error(w, err, http.StatusInternalServerError)
				return
			}
		}

		// Check if proxy settings changed and refresh connection pool
		var newProxyEnabled string
		if ok {
			newProxyEnabled, _ = h.DB.GetSettingForUser(userID, "proxy_enabled")
		} else {
			newProxyEnabled, _ = h.DB.GetSetting("proxy_enabled")
		}
		var newProxyURL string
		if newProxyEnabled == "true" {
			var newProxyType, newProxyHost, newProxyPort, newProxyUsername, newProxyPassword string
			if ok {
				newProxyType, _ = h.DB.GetSettingForUser(userID, "proxy_type")
				newProxyHost, _ = h.DB.GetSettingForUser(userID, "proxy_host")
				newProxyPort, _ = h.DB.GetSettingForUser(userID, "proxy_port")
				newProxyUsername, _ = h.DB.GetEncryptedSettingForUser(userID, "proxy_username")
				newProxyPassword, _ = h.DB.GetEncryptedSettingForUser(userID, "proxy_password")
			} else {
				newProxyType, _ = h.DB.GetSetting("proxy_type")
				newProxyHost, _ = h.DB.GetSetting("proxy_host")
				newProxyPort, _ = h.DB.GetSetting("proxy_port")
				newProxyUsername, _ = h.DB.GetEncryptedSetting("proxy_username")
				newProxyPassword, _ = h.DB.GetEncryptedSetting("proxy_password")
			}
			newProxyURL = httputil.BuildProxyURL(newProxyType, newProxyHost, newProxyPort, newProxyUsername, newProxyPassword)
		}

		if oldProxyURL != newProxyURL {
			log.Printf("[HandleSettings] Proxy settings changed, refreshing connection pool...")
			httputil.RefreshProxyClients(newProxyURL)

			// Refresh translator proxy
			if refresher, okRefresh := h.Translator.(translation.ProxyRefresher); okRefresh {
				refresher.InvalidateCache()
				log.Printf("[HandleSettings] Translator cache invalidated for proxy refresh")
			}

			// Also call RefreshProxy if available to refresh individual translator HTTP clients
			if refresherWithProxy, okRefreshProxy := h.Translator.(interface{ RefreshProxy() }); okRefreshProxy {
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
		var settingsUpdated map[string]string
		if ok {
			settingsUpdated = GetAllSettingsForUser(h, userID, isAdmin, hasInherited)
		} else {
			settingsUpdated = GetAllSettings(h)
		}
		response.JSON(w, settingsUpdated)

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}
