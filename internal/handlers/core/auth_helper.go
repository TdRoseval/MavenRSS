package core

import (
	"net/http"

	"MavenRSS/internal/auth"
	"MavenRSS/internal/middleware"
)

// GetUserIDFromRequest extracts the user ID from the request context
func GetUserIDFromRequest(r *http.Request) (int64, bool) {
	claims, ok := middleware.GetUserFromContext(r.Context())
	if !ok {
		return 0, false
	}
	return claims.UserID, true
}

// GetUserFromRequest extracts the full user claims from the request context
func GetUserFromRequest(r *http.Request) (*auth.Claims, bool) {
	return middleware.GetUserFromContext(r.Context())
}
