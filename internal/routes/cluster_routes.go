package routes

import (
	"net/http"

	cluster "MavenRSS/internal/api/cluster"
	"MavenRSS/internal/api/core"
	"MavenRSS/internal/middleware"
)

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
	registerProtectedRoute(mux, "/api/clusters/mark-all-read", authMiddleware, func(w http.ResponseWriter, r *http.Request) { cluster.HandleMarkAllClustersRead(h, w, r) })
	registerProtectedRoute(mux, "/api/clusters/feed", authMiddleware, func(w http.ResponseWriter, r *http.Request) { cluster.HandleClustersFeed(h, w, r) })
	registerProtectedRoute(mux, "/api/clusters/click", authMiddleware, func(w http.ResponseWriter, r *http.Request) { cluster.HandleClusterClick(h, w, r) })
	registerProtectedRoute(mux, "/api/clusters/read-time", authMiddleware, func(w http.ResponseWriter, r *http.Request) { cluster.HandleClusterReadTime(h, w, r) })
	registerProtectedRoute(mux, "/api/clusters/daily-recommendations", authMiddleware, func(w http.ResponseWriter, r *http.Request) { cluster.HandleDailyRecommendations(h, w, r) })
	registerProtectedRoute(mux, "/api/clusters/daily-recommendations/dates", authMiddleware, func(w http.ResponseWriter, r *http.Request) { cluster.HandleDailyRecommendationDates(h, w, r) })
	registerProtectedRoute(mux, "/api/clusters/daily-recommendations/regenerate", authMiddleware, func(w http.ResponseWriter, r *http.Request) { cluster.HandleRegenerateDailyRecommendations(h, w, r) })
	registerProtectedRoute(mux, "/api/clusters/ai-processing-status", authMiddleware, func(w http.ResponseWriter, r *http.Request) { cluster.HandleAIProcessingStatus(h, w, r) })
}
