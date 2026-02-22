package middleware

import (
	"context"
	"net/http"
	"strings"

	"MavenRSS/internal/auth"
)

type contextKey string

const (
	UserContextKey contextKey = "user"
)

func AuthMiddleware(jwtManager *auth.JWTManager) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				http.Error(w, "authorization header required", http.StatusUnauthorized)
				return
			}

			token := strings.TrimPrefix(authHeader, "Bearer ")
			if token == authHeader {
				http.Error(w, "invalid authorization format", http.StatusUnauthorized)
				return
			}

			claims, err := jwtManager.ValidateToken(token)
			if err != nil {
				if err == auth.ErrExpiredToken {
					http.Error(w, "token expired", http.StatusUnauthorized)
				} else {
					http.Error(w, "invalid token", http.StatusUnauthorized)
				}
				return
			}

			ctx := context.WithValue(r.Context(), UserContextKey, claims)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func AdminMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		claims, ok := r.Context().Value(UserContextKey).(*auth.Claims)
		if !ok {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		if claims.Role != "admin" {
			http.Error(w, "admin access required", http.StatusForbidden)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func GetUserFromContext(ctx context.Context) (*auth.Claims, bool) {
	claims, ok := ctx.Value(UserContextKey).(*auth.Claims)
	return claims, ok
}
