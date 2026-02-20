package auth

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strconv"

	"MrRSS/internal/auth"
	"MrRSS/internal/middleware"
	"MrRSS/internal/models"
)

func (h *Handler) GetPendingRegistrations(w http.ResponseWriter, r *http.Request) {
	regs, err := h.db.ListPendingRegistrations()
	if err != nil {
		jsonError(w, http.StatusInternalServerError, "failed to fetch pending registrations")
		return
	}

	for _, reg := range regs {
		reg.PasswordHash = ""
	}

	jsonResponse(w, http.StatusOK, regs)
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
		UserID:           userID,
		MaxFeeds:         100,
		MaxArticles:      100000,
		MaxAICallsPerDay: 100,
		MaxStorageMB:     500,
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
	users, err := h.db.ListUsers()
	if err != nil {
		jsonError(w, http.StatusInternalServerError, "failed to fetch users")
		return
	}

	for _, user := range users {
		user.PasswordHash = ""
	}

	jsonResponse(w, http.StatusOK, users)
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

	err = h.db.DeleteUserSessions(id)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, "failed to delete user sessions")
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
	MaxFeeds         int   `json:"max_feeds"`
	MaxArticles      int64 `json:"max_articles"`
	MaxAICallsPerDay int   `json:"max_ai_calls_per_day"`
	MaxStorageMB     int   `json:"max_storage_mb"`
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
	if req.MaxAICallsPerDay > 0 {
		quota.MaxAICallsPerDay = req.MaxAICallsPerDay
	}
	if req.MaxStorageMB > 0 {
		quota.MaxStorageMB = req.MaxStorageMB
	}

	err = h.db.UpdateUserQuota(quota)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, "failed to update quota")
		return
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
		UserID:           userID,
		MaxFeeds:         1000,
		MaxArticles:      1000000,
		MaxAICallsPerDay: 1000,
		MaxStorageMB:     5000,
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

	if user.HasInherited {
		jsonError(w, http.StatusBadRequest, "already inherited from template")
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

	// Copy feeds
	query := `INSERT INTO feeds (user_id, title, url, link, description, category, image_url, last_updated, last_error)
	          SELECT ?, title, url, link, description, category, image_url, last_updated, last_error
	          FROM feeds WHERE user_id = ?`
	_, err = tx.Exec(query, user.ID, templateUser.ID)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, "failed to copy feeds")
		return
	}

	// Copy settings (all settings including sensitive ones)
	query = `INSERT INTO user_settings (user_id, key, value)
	          SELECT ?, key, value
	          FROM user_settings WHERE user_id = ?`
	_, err = tx.Exec(query, user.ID, templateUser.ID)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, "failed to copy settings")
		return
	}

	user.InheritedFrom = &templateUser.ID
	user.HasInherited = true
	query = `UPDATE users SET inherited_from = ?, has_inherited = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`
	_, err = tx.Exec(query, templateUser.ID, true, user.ID)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, "failed to update user")
		return
	}

	err = tx.Commit()
	if err != nil {
		jsonError(w, http.StatusInternalServerError, "failed to commit transaction")
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
	templateAvailable := templateUser != nil && !user.HasInherited

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

	jsonResponse(w, http.StatusOK, map[string]interface{}{
		"available":    templateUser != nil && !user.HasInherited,
		"inherited":    user.HasInherited,
		"has_template": templateUser != nil,
	})
}
