// Package routes provides centralized route registration for the MrRSS API.
// This eliminates code duplication between main.go and main-core.go.
package routes

import (
	"net/http"

	"MrRSS/internal/auth"
	"MrRSS/internal/handlers/core"
	"MrRSS/internal/middleware"
)

// Config contains options for route registration.
type Config struct {
	// EnableLogging enables request logging middleware
	EnableLogging bool
	// EnableRecovery enables panic recovery middleware
	EnableRecovery bool
	// EnableCORS enables CORS middleware (useful for server mode)
	EnableCORS bool
	// EnableCompression enables gzip/brotli compression
	EnableCompression bool
	// CORSOrigins specifies allowed origins for CORS
	CORSOrigins []string
	// EnableAuth enables authentication middleware
	EnableAuth bool
	// JWTManager is the JWT manager for authentication
	JWTManager *auth.JWTManager
}

// DefaultConfig returns the default route configuration.
func DefaultConfig() Config {
	return Config{
		EnableLogging:     false,
		EnableRecovery:    true,
		EnableCORS:        false,
		EnableCompression: false,
		EnableAuth:        false,
		CORSOrigins:       []string{"*"},
		JWTManager:        nil,
	}
}

// ServerConfig returns a configuration suitable for server mode.
func ServerConfig(jwtManager *auth.JWTManager) Config {
	return Config{
		EnableLogging:     true,
		EnableRecovery:    true,
		EnableCORS:        true,
		EnableCompression: true,
		EnableAuth:        true,
		CORSOrigins:       []string{"*"},
		JWTManager:        jwtManager,
	}
}

// RegisterAPIRoutes registers all API routes to the provided mux.
// This function is called by both main.go (desktop mode) and main-core.go (server mode).
func RegisterAPIRoutes(mux *http.ServeMux, h *core.Handler) {
	RegisterAPIRoutesWithConfig(mux, h, DefaultConfig())
}

// RegisterAPIRoutesWithConfig registers all API routes with the specified configuration.
func RegisterAPIRoutesWithConfig(mux *http.ServeMux, h *core.Handler, cfg Config) {
	// Register all route groups
	registerFeedRoutes(mux, h, cfg)
	registerArticleRoutes(mux, h, cfg)
	registerAIRoutes(mux, h, cfg)
	registerSettingsRoutes(mux, h, cfg)
	registerOtherRoutes(mux, h, cfg)
}

// WrapWithMiddleware wraps an http.Handler with the standard middleware chain.
func WrapWithMiddleware(handler http.Handler, cfg Config) http.Handler {
	var middlewares []middleware.Middleware

	if cfg.EnableRecovery {
		middlewares = append(middlewares, middleware.Recovery())
	}

	if cfg.EnableLogging {
		middlewares = append(middlewares, middleware.Logger())
	}

	if cfg.EnableCORS {
		corsConfig := middleware.CORSConfig{
			AllowedOrigins: cfg.CORSOrigins,
			AllowedMethods: []string{"GET", "POST", "PUT", "DELETE", "OPTIONS", "PATCH"},
			AllowedHeaders: []string{"Content-Type", "Authorization", "X-Requested-With", "Accept", "Origin", "Cache-Control"},
			ExposedHeaders: []string{"Content-Length", "Content-Type", "Content-Disposition"},
			MaxAge:         86400,
		}
		middlewares = append(middlewares, middleware.CORSWithConfig(corsConfig))
	}

	if cfg.EnableCompression {
		middlewares = append(middlewares, middleware.Compress())
	}

	return middleware.Apply(handler, middlewares...)
}
