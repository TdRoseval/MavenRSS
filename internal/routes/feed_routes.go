// Package routes provides centralized route registration for the MrRSS API.
package routes

import (
	"net/http"

	"MrRSS/internal/handlers/core"
	discovery "MrRSS/internal/handlers/discovery"
	feedhandlers "MrRSS/internal/handlers/feed"
	filter_category "MrRSS/internal/handlers/filter_category"
	rsshubHandler "MrRSS/internal/handlers/rsshub"
	taghandlers "MrRSS/internal/handlers/tags"
	"MrRSS/internal/middleware"
)

// registerFeedRoutes registers all feed-related routes
func registerFeedRoutes(mux *http.ServeMux, h *core.Handler, cfg Config) {
	var authMiddleware middleware.Middleware
	if cfg.EnableAuth && cfg.JWTManager != nil {
		authMiddleware = middleware.AuthMiddleware(cfg.JWTManager)
	}

	registerProtectedRoute(mux, "/api/feeds", authMiddleware, func(w http.ResponseWriter, r *http.Request) { feedhandlers.HandleFeeds(h, w, r) })
	registerProtectedRoute(mux, "/api/feeds/add", authMiddleware, func(w http.ResponseWriter, r *http.Request) { feedhandlers.HandleAddFeed(h, w, r) })
	registerProtectedRoute(mux, "/api/feeds/delete", authMiddleware, func(w http.ResponseWriter, r *http.Request) { feedhandlers.HandleDeleteFeed(h, w, r) })
	registerProtectedRoute(mux, "/api/feeds/update", authMiddleware, func(w http.ResponseWriter, r *http.Request) { feedhandlers.HandleUpdateFeed(h, w, r) })
	registerProtectedRoute(mux, "/api/feeds/refresh", authMiddleware, func(w http.ResponseWriter, r *http.Request) { feedhandlers.HandleRefreshFeed(h, w, r) })
	registerProtectedRoute(mux, "/api/feeds/reorder", authMiddleware, func(w http.ResponseWriter, r *http.Request) { feedhandlers.HandleReorderFeed(h, w, r) })
	registerProtectedRoute(mux, "/api/feeds/test-imap", authMiddleware, func(w http.ResponseWriter, r *http.Request) { feedhandlers.HandleTestIMAPConnection(h, w, r) })

	// Discovery routes
	registerProtectedRoute(mux, "/api/feeds/discover", authMiddleware, func(w http.ResponseWriter, r *http.Request) { discovery.HandleDiscoverBlogs(h, w, r) })
	registerProtectedRoute(mux, "/api/feeds/discover-all", authMiddleware, func(w http.ResponseWriter, r *http.Request) { discovery.HandleDiscoverAllFeeds(h, w, r) })
	registerProtectedRoute(mux, "/api/feeds/discover/start", authMiddleware, func(w http.ResponseWriter, r *http.Request) { discovery.HandleStartSingleDiscovery(h, w, r) })
	registerProtectedRoute(mux, "/api/feeds/discover/progress", authMiddleware, func(w http.ResponseWriter, r *http.Request) { discovery.HandleGetSingleDiscoveryProgress(h, w, r) })
	registerProtectedRoute(mux, "/api/feeds/discover/clear", authMiddleware, func(w http.ResponseWriter, r *http.Request) { discovery.HandleClearSingleDiscovery(h, w, r) })
	registerProtectedRoute(mux, "/api/feeds/discover-all/start", authMiddleware, func(w http.ResponseWriter, r *http.Request) { discovery.HandleStartBatchDiscovery(h, w, r) })
	registerProtectedRoute(mux, "/api/feeds/discover-all/progress", authMiddleware, func(w http.ResponseWriter, r *http.Request) { discovery.HandleGetBatchDiscoveryProgress(h, w, r) })
	registerProtectedRoute(mux, "/api/feeds/discover-all/clear", authMiddleware, func(w http.ResponseWriter, r *http.Request) { discovery.HandleClearBatchDiscovery(h, w, r) })

	// Tag routes
	registerProtectedRoute(mux, "/api/tags", authMiddleware, func(w http.ResponseWriter, r *http.Request) { taghandlers.HandleTags(h, w, r) })
	registerProtectedRoute(mux, "/api/tags/update", authMiddleware, func(w http.ResponseWriter, r *http.Request) { taghandlers.HandleTagUpdate(h, w, r) })
	registerProtectedRoute(mux, "/api/tags/delete", authMiddleware, func(w http.ResponseWriter, r *http.Request) { taghandlers.HandleTagDelete(h, w, r) })
	registerProtectedRoute(mux, "/api/tags/reorder", authMiddleware, func(w http.ResponseWriter, r *http.Request) { taghandlers.HandleTagReorder(h, w, r) })

	// Saved filters routes
	registerProtectedRoute(mux, "/api/saved-filters", authMiddleware, func(w http.ResponseWriter, r *http.Request) {
		filter_category.HandleSavedFilters(h, w, r)
	})
	registerProtectedRoute(mux, "/api/saved-filters/reorder", authMiddleware, func(w http.ResponseWriter, r *http.Request) {
		filter_category.HandleReorderSavedFilters(h, w, r)
	})
	registerProtectedRoute(mux, "/api/saved-filters/filter", authMiddleware, func(w http.ResponseWriter, r *http.Request) {
		filter_category.HandleSavedFilter(h, w, r)
	})

	// RSSHub routes
	registerProtectedRoute(mux, "/api/rsshub/add", authMiddleware, func(w http.ResponseWriter, r *http.Request) { rsshubHandler.HandleAddFeed(h, w, r) })
	registerProtectedRoute(mux, "/api/rsshub/test-connection", authMiddleware, func(w http.ResponseWriter, r *http.Request) { rsshubHandler.HandleTestConnection(h, w, r) })
	registerProtectedRoute(mux, "/api/rsshub/validate-route", authMiddleware, func(w http.ResponseWriter, r *http.Request) { rsshubHandler.HandleValidateRoute(h, w, r) })
	registerProtectedRoute(mux, "/api/rsshub/transform-url", authMiddleware, func(w http.ResponseWriter, r *http.Request) { rsshubHandler.HandleTransformURL(h, w, r) })
}
