package auth

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"

	"MavenRSS/internal/auth"
	"MavenRSS/internal/middleware"
	"MavenRSS/internal/models"
)

func (h *Handler) GetPendingRegistrations(w http.ResponseWriter, r *http.Request) {
	page := 1
	pageSize := 20

	if p := r.URL.Query().Get("page"); p != "" {
		if parsed, err := strconv.Atoi(p); err == nil && parsed > 0 {
			page = parsed
		}
	}
	if ps := r.URL.Query().Get("page_size"); ps != "" {
		if parsed, err := strconv.Atoi(ps); err == nil && parsed > 0 {
			pageSize = parsed
		}
	}

	regs, total, err := h.db.ListPendingRegistrationsPaginated(page, pageSize)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, "failed to fetch pending registrations")
		return
	}

	for _, reg := range regs {
		reg.PasswordHash = ""
	}

	jsonResponse(w, http.StatusOK, map[string]interface{}{
		"registrations": regs,
		"total":         total,
		"page":          page,
		"page_size":     pageSize,
	})
}

type ApproveRequest struct {
	RegistrationID int64 `json:"registration_id"`
}

func (h *Handler) ApproveRegistration(w http.ResponseWriter, r *http.Request) {
	var req ApproveRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	reg, err := h.db.GetPendingRegistrationByID(req.RegistrationID)
	if err != nil {
		jsonError(w, http.StatusNotFound, "registration not found")
		return
	}

	user := &models.User{
		Username:     reg.Username,
		Email:        reg.Email,
		PasswordHash: reg.PasswordHash,
		Role:         models.RoleUser,
		Status:       "active",
	}

	userID, err := h.db.CreateUser(user)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, "failed to create user")
		return
	}

	quota := &models.UserQuota{
		UserID:                   userID,
		MaxFeeds:                 100,
		MaxArticles:              100000,
		MaxAITokens:              1000000,
		MaxAIConcurrency:         5,
		MaxFeedFetchConcurrency:  3,
		MaxDBQueryConcurrency:    5,
		MaxMediaCacheConcurrency: 5,
		MaxRSSDiscoveryConcurrency: 8,
		MaxRSSPathCheckConcurrency: 5,
		MaxTranslationConcurrency: 3,
		MaxStorageMB:             500,
	}
	_, err = h.db.CreateUserQuota(quota)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, "failed to create user quota")
		return
	}

	err = h.db.DeletePendingRegistration(reg.ID)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, "failed to delete pending registration")
		return
	}

	jsonResponse(w, http.StatusOK, map[string]interface{}{
		"message": "User approved successfully",
		"user_id": userID,
	})
}

func (h *Handler) RejectRegistration(w http.ResponseWriter, r *http.Request) {
	var req ApproveRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	err := h.db.DeletePendingRegistration(req.RegistrationID)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, "failed to reject registration")
		return
	}

	jsonResponse(w, http.StatusOK, map[string]string{
		"message": "Registration rejected successfully",
	})
}

func (h *Handler) ListUsers(w http.ResponseWriter, r *http.Request) {
	page := 1
	pageSize := 20

	if p := r.URL.Query().Get("page"); p != "" {
		if parsed, err := strconv.Atoi(p); err == nil && parsed > 0 {
			page = parsed
		}
	}

	if ps := r.URL.Query().Get("page_size"); ps != "" {
		if parsed, err := strconv.Atoi(ps); err == nil && parsed > 0 && parsed <= 100 {
			pageSize = parsed
		}
	}

	users, total, err := h.db.ListUsersPaginated(page, pageSize)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, "failed to fetch users")
		return
	}

	for _, user := range users {
		user.PasswordHash = ""
	}

	jsonResponse(w, http.StatusOK, map[string]interface{}{
		"users":     users,
		"total":     total,
		"page":      page,
		"page_size": pageSize,
	})
}

func (h *Handler) GetUser(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		jsonError(w, http.StatusBadRequest, "invalid user id")
		return
	}

	user, err := h.db.GetUserByID(id)
	if err != nil {
		jsonError(w, http.StatusNotFound, "user not found")
		return
	}

	user.PasswordHash = ""
	jsonResponse(w, http.StatusOK, user)
}

type UpdateUserRequest struct {
	Username string `json:"username"`
	Email    string `json:"email"`
	Role     string `json:"role"`
	Status   string `json:"status"`
}

func (h *Handler) UpdateUser(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		jsonError(w, http.StatusBadRequest, "invalid user id")
		return
	}

	var req UpdateUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	user, err := h.db.GetUserByID(id)
	if err != nil {
		jsonError(w, http.StatusNotFound, "user not found")
		return
	}

	if req.Username != "" {
		user.Username = req.Username
	}
	if req.Email != "" {
		user.Email = req.Email
	}
	if req.Role != "" {
		user.Role = models.UserRole(req.Role)
	}
	if req.Status != "" {
		user.Status = req.Status
	}

	err = h.db.UpdateUser(user)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, "failed to update user")
		return
	}

	user.PasswordHash = ""
	jsonResponse(w, http.StatusOK, user)
}

func (h *Handler) DeleteUser(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		jsonError(w, http.StatusBadRequest, "invalid user id")
		return
	}

	claims, _ := middleware.GetUserFromContext(r.Context())
	if claims.UserID == id {
		jsonError(w, http.StatusBadRequest, "cannot delete yourself")
		return
	}

	err = h.db.DeleteUser(id)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, "failed to delete user")
		return
	}

	jsonResponse(w, http.StatusOK, map[string]string{
		"message": "User deleted successfully",
	})
}

func (h *Handler) GetUserQuota(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		jsonError(w, http.StatusBadRequest, "invalid user id")
		return
	}

	quota, err := h.db.GetUserQuota(id)
	if err != nil {
		if err == sql.ErrNoRows {
			jsonError(w, http.StatusNotFound, "quota not found")
			return
		}
		jsonError(w, http.StatusInternalServerError, "failed to fetch quota")
		return
	}

	jsonResponse(w, http.StatusOK, quota)
}

type UpdateQuotaRequest struct {
	MaxFeeds                   int   `json:"max_feeds"`
	MaxArticles                int64 `json:"max_articles"`
	MaxAITokens                int64 `json:"max_ai_tokens"`
	MaxAIConcurrency           int   `json:"max_ai_concurrency"`
	MaxFeedFetchConcurrency    int   `json:"max_feed_fetch_concurrency"`
	MaxDBQueryConcurrency      int   `json:"max_db_query_concurrency"`
	MaxMediaCacheConcurrency   int   `json:"max_media_cache_concurrency"`
	MaxRSSDiscoveryConcurrency int   `json:"max_rss_discovery_concurrency"`
	MaxRSSPathCheckConcurrency int  `json:"max_rss_path_check_concurrency"`
	MaxTranslationConcurrency  int   `json:"max_translation_concurrency"`
	MaxStorageMB               int   `json:"max_storage_mb"`
}

func (h *Handler) UpdateUserQuota(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		jsonError(w, http.StatusBadRequest, "invalid user id")
		return
	}

	var req UpdateQuotaRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	quota, err := h.db.GetUserQuota(id)
	if err != nil {
		if err == sql.ErrNoRows {
			jsonError(w, http.StatusNotFound, "quota not found")
			return
		}
		jsonError(w, http.StatusInternalServerError, "failed to fetch quota")
		return
	}

	if req.MaxFeeds > 0 {
		quota.MaxFeeds = req.MaxFeeds
	}
	if req.MaxArticles > 0 {
		quota.MaxArticles = req.MaxArticles
	}
	if req.MaxAITokens > 0 {
		quota.MaxAITokens = req.MaxAITokens
	}
	if req.MaxAIConcurrency > 0 {
		quota.MaxAIConcurrency = req.MaxAIConcurrency
	}
	if req.MaxFeedFetchConcurrency > 0 {
		quota.MaxFeedFetchConcurrency = req.MaxFeedFetchConcurrency
	}
	if req.MaxDBQueryConcurrency > 0 {
		quota.MaxDBQueryConcurrency = req.MaxDBQueryConcurrency
	}
	if req.MaxMediaCacheConcurrency > 0 {
		quota.MaxMediaCacheConcurrency = req.MaxMediaCacheConcurrency
	}
	if req.MaxRSSDiscoveryConcurrency > 0 {
		quota.MaxRSSDiscoveryConcurrency = req.MaxRSSDiscoveryConcurrency
	}
	if req.MaxRSSPathCheckConcurrency > 0 {
		quota.MaxRSSPathCheckConcurrency = req.MaxRSSPathCheckConcurrency
	}
	if req.MaxTranslationConcurrency > 0 {
		quota.MaxTranslationConcurrency = req.MaxTranslationConcurrency
	}
	oldMaxStorageMB := quota.MaxStorageMB
	if req.MaxStorageMB > 0 {
		quota.MaxStorageMB = req.MaxStorageMB
	}

	err = h.db.UpdateUserQuota(quota)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, "failed to update quota")
		return
	}

	// If storage quota was lowered, auto-adjust user's max_cache_size_mb setting
	if req.MaxStorageMB > 0 && req.MaxStorageMB < oldMaxStorageMB {
		log.Printf("[UpdateQuota] Storage quota lowered from %d to %d, checking user settings...", oldMaxStorageMB, req.MaxStorageMB)
		currentVal, err := h.db.GetSettingForUser(id, "max_cache_size_mb")
		if err == nil && currentVal != "" {
			var currentSize int
			if _, err = fmt.Sscanf(currentVal, "%d", &currentSize); err == nil {
				if currentSize > req.MaxStorageMB {
					log.Printf("[UpdateQuota] Auto-adjusting user %d's max_cache_size_mb from %d to %d", id, currentSize, req.MaxStorageMB)
					_ = h.db.SetSettingForUser(id, "max_cache_size_mb", fmt.Sprintf("%d", req.MaxStorageMB))
				}
			}
		}
	}

	jsonResponse(w, http.StatusOK, quota)
}

func (h *Handler) CreateTemplateUser(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username string `json:"username"`
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Username == "" || req.Email == "" || req.Password == "" {
		jsonError(w, http.StatusBadRequest, "username, email and password are required")
		return
	}

	hashedPassword, err := auth.HashPassword(req.Password)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, "failed to hash password")
		return
	}

	user := &models.User{
		Username:     req.Username,
		Email:        req.Email,
		PasswordHash: hashedPassword,
		Role:         models.RoleTemplate,
		Status:       "active",
	}

	userID, err := h.db.CreateUser(user)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, "failed to create template user")
		return
	}

	quota := &models.UserQuota{
		UserID:                   userID,
		MaxFeeds:                 1000,
		MaxArticles:              1000000,
		MaxAITokens:              10000000,
		MaxAIConcurrency:         20,
		MaxFeedFetchConcurrency:  10,
		MaxDBQueryConcurrency:    10,
		MaxMediaCacheConcurrency: 10,
		MaxRSSDiscoveryConcurrency: 16,
		MaxRSSPathCheckConcurrency: 10,
		MaxTranslationConcurrency: 10,
		MaxStorageMB:             5000,
	}
	_, err = h.db.CreateUserQuota(quota)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, "failed to create template quota")
		return
	}

	jsonResponse(w, http.StatusOK, map[string]interface{}{
		"message": "Template user created successfully",
		"user_id": userID,
	})
}

func (h *Handler) GetTemplateUser(w http.ResponseWriter, r *http.Request) {
	user, err := h.db.GetTemplateUser()
	if err != nil {
		jsonError(w, http.StatusInternalServerError, "failed to fetch template user")
		return
	}
	if user == nil {
		jsonError(w, http.StatusNotFound, "template user not found")
		return
	}

	user.PasswordHash = ""
	jsonResponse(w, http.StatusOK, user)
}

// isSensitiveSetting checks if a setting is a sensitive configuration
func isSensitiveSetting(key string) bool {
	sensitivePrefixes := []string{"ai_", "proxy_", "rsshub_"}
	for _, prefix := range sensitivePrefixes {
		if len(key) >= len(prefix) && key[:len(prefix)] == prefix {
			return true
		}
	}
	return false
}

func (h *Handler) InheritTemplate(w http.ResponseWriter, r *http.Request) {
	claims, ok := middleware.GetUserFromContext(r.Context())
	if !ok {
		jsonError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	user, err := h.db.GetUserByID(claims.UserID)
	if err != nil {
		jsonError(w, http.StatusNotFound, "user not found")
		return
	}

	templateUser, err := h.db.GetTemplateUser()
	if err != nil {
		jsonError(w, http.StatusInternalServerError, "failed to get template user")
		return
	}
	if templateUser == nil {
		jsonError(w, http.StatusNotFound, "template user not found")
		return
	}

	tx, err := h.db.Begin()
	if err != nil {
		jsonError(w, http.StatusInternalServerError, "failed to start transaction")
		return
	}
	defer tx.Rollback()

	// Copy AI profiles first (since settings reference them)
	var profileCount int
	err = tx.QueryRow(`SELECT COUNT(*) FROM ai_profiles WHERE user_id = ?`, templateUser.ID).Scan(&profileCount)
	if err != nil {
		profileCount = 0
	}
	
	log.Printf("[InheritTemplate] Template user has %d AI profiles to copy", profileCount)
	
	var profileIDMap map[int64]int64
	if profileCount > 0 {
		profileIDMap, err = h.db.CopyAIProfiles(templateUser.ID, user.ID, tx)
		if err != nil {
			jsonError(w, http.StatusInternalServerError, "failed to copy AI profiles: "+err.Error())
			return
		}
		log.Printf("[InheritTemplate] Copied %d AI profiles, ID map: %v", len(profileIDMap), profileIDMap)
	}

	// Copy feeds
	var feedCount int
	err = tx.QueryRow(`SELECT COUNT(*) FROM feeds WHERE user_id = ?`, templateUser.ID).Scan(&feedCount)
	if err != nil {
		feedCount = 0
	}
	
	if feedCount > 0 {
		_, err = tx.Exec(`DELETE FROM feeds WHERE user_id = ?`, user.ID)
		if err != nil {
		}
		
		query := `INSERT INTO feeds (
			user_id, title, url, link, description, category, image_url, 
			script_path, hide_from_timeline, proxy_url, proxy_enabled, refresh_interval,
			is_image_mode, type,
			xpath_item, xpath_item_title, xpath_item_content, xpath_item_uri,
			xpath_item_author, xpath_item_timestamp, xpath_item_time_format,
			xpath_item_thumbnail, xpath_item_categories, xpath_item_uid,
			article_view_mode, auto_expand_content,
			email_address, email_imap_server, email_imap_port,
			email_username, email_password, email_folder, email_last_uid,
			is_freshrss_source, freshrss_stream_id,
			position, last_updated, last_error
		) SELECT 
			?, title, url, link, description, category, image_url, 
			script_path, hide_from_timeline, proxy_url, proxy_enabled, refresh_interval,
			is_image_mode, type,
			xpath_item, xpath_item_title, xpath_item_content, xpath_item_uri,
			xpath_item_author, xpath_item_timestamp, xpath_item_time_format,
			xpath_item_thumbnail, xpath_item_categories, xpath_item_uid,
			article_view_mode, auto_expand_content,
			email_address, email_imap_server, email_imap_port,
			email_username, email_password, email_folder, email_last_uid,
			is_freshrss_source, freshrss_stream_id,
			position, last_updated, last_error
		FROM feeds WHERE user_id = ?`
		_, err = tx.Exec(query, user.ID, templateUser.ID)
		if err != nil {
			jsonError(w, http.StatusInternalServerError, "failed to copy feeds: "+err.Error())
			return
		}
	}

	// Copy settings (all settings including sensitive ones)
	var count int
	err = tx.QueryRow(`SELECT COUNT(*) FROM user_settings WHERE user_id = ?`, templateUser.ID).Scan(&count)
	if err != nil {
		count = 0
	}
	
	log.Printf("[InheritTemplate] Template user has %d settings to copy", count)
	
	if count > 0 {
		err = h.db.CopyUserSettings(templateUser.ID, user.ID, tx)
		if err != nil {
			jsonError(w, http.StatusInternalServerError, "failed to copy settings: "+err.Error())
			return
		}
		
		// Update profile ID references in settings if we have a map
		if profileIDMap != nil && len(profileIDMap) > 0 {
			profileSettingKeys := []string{"ai_chat_profile_id", "ai_search_profile_id", "ai_summary_profile_id", "ai_translation_profile_id"}
			for _, key := range profileSettingKeys {
				var oldIDStr string
				err = tx.QueryRow(`SELECT value FROM user_settings WHERE user_id = ? AND key = ?`, user.ID, key).Scan(&oldIDStr)
				if err == nil && oldIDStr != "" {
					var oldID int64
					_, err = fmt.Sscanf(oldIDStr, "%d", &oldID)
					if err == nil {
						if newID, ok := profileIDMap[oldID]; ok {
							newIDStr := fmt.Sprintf("%d", newID)
							_, err = tx.Exec(`UPDATE user_settings SET value = ? WHERE user_id = ? AND key = ?`, newIDStr, user.ID, key)
							log.Printf("[InheritTemplate] Updated %s from %d to %d", key, oldID, newID)
						}
					}
				}
			}
		}
		
		var copiedCount int
		err = tx.QueryRow(`SELECT COUNT(*) FROM user_settings WHERE user_id = ?`, user.ID).Scan(&copiedCount)
		log.Printf("[InheritTemplate] Copied %d settings to user %d", copiedCount, user.ID)

		// Auto-adjust user settings to not exceed admin quota
		log.Printf("[InheritTemplate] Auto-adjusting user settings to not exceed admin quota...")
		quota, qErr := h.db.GetUserQuota(user.ID)
		if qErr == nil && quota != nil {
			// Adjust max_cache_size_mb if needed
			if quota.MaxStorageMB > 0 {
				var currentVal string
				err = tx.QueryRow(`SELECT value FROM user_settings WHERE user_id = ? AND key = ?`, user.ID, "max_cache_size_mb").Scan(&currentVal)
				if err == nil && currentVal != "" {
					var currentSize int
					if _, err = fmt.Sscanf(currentVal, "%d", &currentSize); err == nil {
						if currentSize > quota.MaxStorageMB {
							log.Printf("[InheritTemplate] Adjusting max_cache_size_mb from %d to %d (quota limit)", currentSize, quota.MaxStorageMB)
							_, err = tx.Exec(`UPDATE user_settings SET value = ? WHERE user_id = ? AND key = ?`, fmt.Sprintf("%d", quota.MaxStorageMB), user.ID, "max_cache_size_mb")
						}
					}
				}
			}
		}
	}

	user.InheritedFrom = &templateUser.ID
	user.HasInherited = true
	query := `UPDATE users SET inherited_from = ?, has_inherited = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`
	_, err = tx.Exec(query, templateUser.ID, true, user.ID)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, "failed to update user: "+err.Error())
		return
	}

	err = tx.Commit()
	if err != nil {
		jsonError(w, http.StatusInternalServerError, "failed to commit transaction: "+err.Error())
		return
	}

	jsonResponse(w, http.StatusOK, map[string]string{
		"message": "Successfully inherited from template",
	})
}

func (h *Handler) GetMe(w http.ResponseWriter, r *http.Request) {
	claims, ok := middleware.GetUserFromContext(r.Context())
	if !ok {
		jsonError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	user, err := h.db.GetUserByID(claims.UserID)
	if err != nil {
		jsonError(w, http.StatusNotFound, "user not found")
		return
	}

	quota, err := h.db.GetUserQuota(claims.UserID)
	if err != nil {
		quota = nil
	}

	templateUser, err := h.db.GetTemplateUser()
	// Allow template available even if already inherited
	templateAvailable := templateUser != nil

	user.PasswordHash = ""
	jsonResponse(w, http.StatusOK, map[string]interface{}{
		"user":              user,
		"quota":             quota,
		"template_available": templateAvailable,
	})
}

func (h *Handler) CheckTemplateAvailable(w http.ResponseWriter, r *http.Request) {
	claims, ok := middleware.GetUserFromContext(r.Context())
	if !ok {
		jsonError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	user, err := h.db.GetUserByID(claims.UserID)
	if err != nil {
		jsonError(w, http.StatusNotFound, "user not found")
		return
	}

	templateUser, err := h.db.GetTemplateUser()
	if err != nil {
		jsonError(w, http.StatusInternalServerError, "failed to check template")
		return
	}

	// Allow re-inheriting even if already inherited
	jsonResponse(w, http.StatusOK, map[string]interface{}{
		"available":    templateUser != nil,
		"inherited":    user.HasInherited,
		"has_template": templateUser != nil,
	})
}
