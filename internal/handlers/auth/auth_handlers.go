package auth

import (
	"encoding/json"
	"net/http"
	"time"

	"MrRSS/internal/auth"
	"MrRSS/internal/database"
	"MrRSS/internal/models"
)

type Handler struct {
	db         *database.DB
	jwtManager *auth.JWTManager
}

func NewHandler(db *database.DB, jwtSecret string) *Handler {
	return &Handler{
		db:         db,
		jwtManager: auth.NewJWTManager(jwtSecret),
	}
}

type RegisterRequest struct {
	Username string `json:"username"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type AuthResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	User         *models.User `json:"user"`
}

func (h *Handler) Register(w http.ResponseWriter, r *http.Request) {
	var req RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Username == "" || req.Email == "" || req.Password == "" {
		jsonError(w, http.StatusBadRequest, "username, email and password are required")
		return
	}

	_, err := h.db.GetUserByUsername(req.Username)
	if err == nil {
		jsonError(w, http.StatusConflict, "username already exists")
		return
	}

	_, err = h.db.GetUserByEmail(req.Email)
	if err == nil {
		jsonError(w, http.StatusConflict, "email already exists")
		return
	}

	hashedPassword, err := auth.HashPassword(req.Password)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, "failed to hash password")
		return
	}

	pendingReg := &models.PendingRegistration{
		Username:     req.Username,
		Email:        req.Email,
		PasswordHash: hashedPassword,
	}

	_, err = h.db.CreatePendingRegistration(pendingReg)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, "failed to submit registration")
		return
	}

	jsonResponse(w, http.StatusOK, map[string]string{
		"message": "Registration submitted successfully. Please wait for admin approval.",
	})
}

func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	user, err := h.db.GetUserByUsername(req.Username)
	if err != nil {
		jsonError(w, http.StatusUnauthorized, "invalid credentials")
		return
	}

	if user.Status != "active" {
		jsonError(w, http.StatusUnauthorized, "account not active")
		return
	}

	if !auth.CheckPassword(req.Password, user.PasswordHash) {
		jsonError(w, http.StatusUnauthorized, "invalid credentials")
		return
	}

	accessToken, refreshToken, err := h.jwtManager.GenerateTokens(user.ID, user.Username, string(user.Role))
	if err != nil {
		jsonError(w, http.StatusInternalServerError, "failed to generate tokens")
		return
	}

	session := &models.UserSession{
		UserID:       user.ID,
		RefreshToken: refreshToken,
		UserAgent:    r.UserAgent(),
		IPAddress:    r.RemoteAddr,
		ExpiresAt:    time.Now().Add(30 * 24 * time.Hour),
	}
	_, err = h.db.CreateUserSession(session)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, "failed to create session")
		return
	}

	user.PasswordHash = ""
	jsonResponse(w, http.StatusOK, AuthResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		User:         user,
	})
}

type RefreshRequest struct {
	RefreshToken string `json:"refresh_token"`
}

func (h *Handler) Refresh(w http.ResponseWriter, r *http.Request) {
	var req RefreshRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	session, err := h.db.GetUserSessionByToken(req.RefreshToken)
	if err != nil {
		jsonError(w, http.StatusUnauthorized, "invalid refresh token")
		return
	}

	if time.Now().After(session.ExpiresAt) {
		h.db.DeleteUserSession(session.ID)
		jsonError(w, http.StatusUnauthorized, "refresh token expired")
		return
	}

	accessToken, newRefreshToken, err := h.jwtManager.RefreshToken(req.RefreshToken)
	if err != nil {
		jsonError(w, http.StatusUnauthorized, "invalid refresh token")
		return
	}

	session.RefreshToken = newRefreshToken
	_, err = h.db.CreateUserSession(session)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, "failed to update session")
		return
	}

	user, err := h.db.GetUserByID(session.UserID)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, "failed to get user")
		return
	}

	user.PasswordHash = ""
	jsonResponse(w, http.StatusOK, AuthResponse{
		AccessToken:  accessToken,
		RefreshToken: newRefreshToken,
		User:         user,
	})
}

func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
	var req RefreshRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	session, err := h.db.GetUserSessionByToken(req.RefreshToken)
	if err == nil {
		h.db.DeleteUserSession(session.ID)
	}

	jsonResponse(w, http.StatusOK, map[string]string{
		"message": "Logged out successfully",
	})
}
