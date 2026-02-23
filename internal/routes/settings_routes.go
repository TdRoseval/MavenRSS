package routes

import (
	"context"
	"log"
	"net/http"
	"strings"

	"MavenRSS/internal/auth"
	"MavenRSS/internal/handlers/core"
	settings "MavenRSS/internal/handlers/settings"
	stathandlers "MavenRSS/internal/handlers/statistics"
	"MavenRSS/internal/middleware"
)

// registerSettingsRoutes registers all settings-related routes
func registerSettingsRoutes(mux *http.ServeMux, h *core.Handler, cfg Config) {
	var authMiddleware middleware.Middleware
	if cfg.EnableAuth && cfg.JWTManager != nil {
		authMiddleware = middleware.AuthMiddleware(cfg.JWTManager)
	}

	// Settings handler - GET is public but uses user context if available, POST requires auth
	if authMiddleware != nil {
		// Create a smart handler that checks auth for POST and optionally for GET
		smartHandler := func(w http.ResponseWriter, r *http.Request) {
			// Check if we have an auth header, and if so, validate it and set user context
			authHeader := r.Header.Get("Authorization")
			var ctx context.Context = r.Context()
			var userID int64 = 0

			if authHeader != "" {
				token := strings.TrimPrefix(authHeader, "Bearer ")
				if token != authHeader {
					claims, err := cfg.JWTManager.ValidateToken(token)
					if err == nil {
						// Token is valid, set user in context
						ctx = context.WithValue(r.Context(), middleware.UserContextKey, claims)
						userID = claims.UserID
						log.Printf("[HandleSettings] Token valid for user %d, method=%s", userID, r.Method)
					} else {
						log.Printf("[HandleSettings] Token validation failed: %v, method=%s", err, r.Method)
					}
				}
			} else {
				log.Printf("[HandleSettings] No auth header, method=%s", r.Method)
			}

			if r.Method == http.MethodGet {
				// GET is always public, but uses user context if available
				log.Printf("[HandleSettings] GET request, userID=%d", userID)
				settings.HandleSettings(h, w, r.WithContext(ctx))
			} else if r.Method == http.MethodPost {
				// POST requires auth
				if authHeader == "" {
					http.Error(w, "authorization header required", http.StatusUnauthorized)
					return
				}

				token := strings.TrimPrefix(authHeader, "Bearer ")
				if token == authHeader {
					http.Error(w, "invalid authorization format", http.StatusUnauthorized)
					return
				}

				claims, err := cfg.JWTManager.ValidateToken(token)
				if err != nil {
					if err == auth.ErrExpiredToken {
						http.Error(w, "token expired", http.StatusUnauthorized)
					} else {
						http.Error(w, "invalid token", http.StatusUnauthorized)
					}
					return
				}

				// Set user in context and handle
				postCtx := context.WithValue(r.Context(), middleware.UserContextKey, claims)
				log.Printf("[HandleSettings] POST request, userID=%d", claims.UserID)
				settings.HandleSettings(h, w, r.WithContext(postCtx))
			} else {
				http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			}
		}
		mux.HandleFunc("/api/settings", smartHandler)
	} else {
		// If auth is disabled, just register the normal handler for all methods
		mux.HandleFunc("/api/settings", func(w http.ResponseWriter, r *http.Request) {
			settings.HandleSettings(h, w, r)
		})
	}

	// Statistics
	registerProtectedRoute(mux, "/api/statistics", authMiddleware, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodDelete {
			stathandlers.HandleResetStatistics(h, w, r)
		} else {
			stathandlers.HandleGetStatistics(h, w, r)
		}
	})
	registerProtectedRoute(mux, "/api/statistics/all-time", authMiddleware, func(w http.ResponseWriter, r *http.Request) { stathandlers.HandleGetAllTimeStatistics(h, w, r) })
	registerProtectedRoute(mux, "/api/statistics/available-months", authMiddleware, func(w http.ResponseWriter, r *http.Request) { stathandlers.HandleGetAvailableMonths(h, w, r) })
}
