package routes

import (
	"net/http"

	cluster "MavenRSS/internal/api/cluster"
	"MavenRSS/internal/api/core"
	"MavenRSS/internal/middleware"
)

// registerClusterRoutes registers all cluster-related routes for AI deduplication
func registerClusterRoutes(mux *http.ServeMux, h *core.Handler, cfg Config) {
	var authMiddleware middleware.Middleware
	if cfg.EnableAuth && cfg.JWTManager != nil {
		authMiddleware = middleware.AuthMiddleware(cfg.JWTManager)
	}

	registerProtectedRoute(mux, "/api/clusters", authMiddleware, func(w http.ResponseWriter, r *http.Request) { cluster.HandleClusters(h, w, r) })
	registerProtectedRoute(mux, "/api/clusters/detail", authMiddleware, func(w http.ResponseWriter, r *http.Request) { cluster.HandleClusterDetail(h, w, r) })
	registerProtectedRoute(mux, "/api/clusters/read", authMiddleware, func(w http.ResponseWriter, r *http.Request) { cluster.HandleClusterRead(h, w, r) })
	registerProtectedRoute(mux, "/api/clusters/favorite", authMiddleware, func(w http.ResponseWriter, r *http.Request) { cluster.HandleClusterFavorite(h, w, r) })
	registerProtectedRoute(mux, "/api/clusters/read-later", authMiddleware, func(w http.ResponseWriter, r *http.Request) { cluster.HandleClusterReadLater(h, w, r) })
}
