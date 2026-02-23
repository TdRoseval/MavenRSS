package routes

import (
	"net/http"
	"time"

	"MavenRSS/internal/auth"
	auth_handlers "MavenRSS/internal/handlers/auth"
	"MavenRSS/internal/middleware"
)

func RegisterAuthRoutes(mux *http.ServeMux, authHandler *auth_handlers.Handler, jwtManager *auth.JWTManager) {
	authMiddleware := middleware.AuthMiddleware(jwtManager)

	strictRateLimitConfig := middleware.RateLimiterConfig{
		RequestsPerSecond: 2,
		BurstSize:         5,
		CleanupInterval:   time.Minute,
	}
	strictRateLimiter := middleware.RateLimiter(strictRateLimitConfig)

	mux.Handle("POST /api/auth/register", strictRateLimiter(http.HandlerFunc(authHandler.Register)))
	mux.Handle("POST /api/auth/login", strictRateLimiter(http.HandlerFunc(authHandler.Login)))
	mux.HandleFunc("POST /api/auth/refresh", authHandler.Refresh)
	mux.HandleFunc("POST /api/auth/logout", authHandler.Logout)

	mux.Handle("GET /api/auth/me", authMiddleware(http.HandlerFunc(authHandler.GetMe)))
	mux.Handle("GET /api/auth/template-available", authMiddleware(http.HandlerFunc(authHandler.CheckTemplateAvailable)))
	mux.Handle("POST /api/auth/inherit-template", authMiddleware(http.HandlerFunc(authHandler.InheritTemplate)))

	adminRoutes := http.NewServeMux()
	adminRoutes.HandleFunc("GET /pending-registrations", authHandler.GetPendingRegistrations)
	adminRoutes.HandleFunc("POST /approve-registration", authHandler.ApproveRegistration)
	adminRoutes.HandleFunc("POST /reject-registration", authHandler.RejectRegistration)
	adminRoutes.HandleFunc("GET /users", authHandler.ListUsers)
	adminRoutes.HandleFunc("GET /users/{id}", authHandler.GetUser)
	adminRoutes.HandleFunc("PUT /users/{id}", authHandler.UpdateUser)
	adminRoutes.HandleFunc("DELETE /users/{id}", authHandler.DeleteUser)
	adminRoutes.HandleFunc("GET /users/{id}/quota", authHandler.GetUserQuota)
	adminRoutes.HandleFunc("PUT /users/{id}/quota", authHandler.UpdateUserQuota)
	adminRoutes.HandleFunc("POST /create-template", authHandler.CreateTemplateUser)
	adminRoutes.HandleFunc("GET /template", authHandler.GetTemplateUser)

	mux.Handle("/api/admin/", http.StripPrefix("/api/admin", authMiddleware(middleware.AdminMiddleware(adminRoutes))))
}
