package routes

import (
	"MavenRSS/internal/handlers/article"
	browser "MavenRSS/internal/handlers/browser"
	"MavenRSS/internal/handlers/core"
	customcss "MavenRSS/internal/handlers/custom_css"
	freshrssHandler "MavenRSS/internal/handlers/freshrss"
	media "MavenRSS/internal/handlers/media"
	networkhandlers "MavenRSS/internal/handlers/network"
	opml "MavenRSS/internal/handlers/opml"
	rules "MavenRSS/internal/handlers/rules"
	script "MavenRSS/internal/handlers/script"
	update "MavenRSS/internal/handlers/update"
	window "MavenRSS/internal/handlers/window"
	"MavenRSS/internal/middleware"
	"net/http"
)

// registerOtherRoutes registers all other miscellaneous routes
func registerOtherRoutes(mux *http.ServeMux, h *core.Handler, cfg Config) {
	var authMiddleware middleware.Middleware
	if cfg.EnableAuth && cfg.JWTManager != nil {
		authMiddleware = middleware.AuthMiddleware(cfg.JWTManager)
	}

	// Refresh and progress
	registerProtectedRoute(mux, "/api/refresh", authMiddleware, func(w http.ResponseWriter, r *http.Request) { article.HandleRefresh(h, w, r) })
	registerProtectedRoute(mux, "/api/refresh/stop", authMiddleware, func(w http.ResponseWriter, r *http.Request) { article.HandleStopRefresh(h, w, r) })
	registerProtectedRoute(mux, "/api/progress", authMiddleware, func(w http.ResponseWriter, r *http.Request) { article.HandleProgress(h, w, r) })
	registerProtectedRoute(mux, "/api/progress/task-details", authMiddleware, func(w http.ResponseWriter, r *http.Request) { article.HandleTaskDetails(h, w, r) })

	// OPML
	registerProtectedRoute(mux, "/api/opml/import", authMiddleware, func(w http.ResponseWriter, r *http.Request) { opml.HandleOPMLImport(h, w, r) })
	registerProtectedRoute(mux, "/api/opml/export", authMiddleware, func(w http.ResponseWriter, r *http.Request) { opml.HandleOPMLExport(h, w, r) })
	registerProtectedRoute(mux, "/api/opml/import-dialog", authMiddleware, func(w http.ResponseWriter, r *http.Request) { opml.HandleOPMLImportDialog(h, w, r) })
	registerProtectedRoute(mux, "/api/opml/export-dialog", authMiddleware, func(w http.ResponseWriter, r *http.Request) { opml.HandleOPMLExportDialog(h, w, r) })

	// Update
	registerPublicRoute(mux, "/api/check-updates", func(w http.ResponseWriter, r *http.Request) { update.HandleCheckUpdates(h, w, r) })
	registerProtectedRoute(mux, "/api/download-update", authMiddleware, func(w http.ResponseWriter, r *http.Request) { update.HandleDownloadUpdate(h, w, r) })
	registerProtectedRoute(mux, "/api/install-update", authMiddleware, func(w http.ResponseWriter, r *http.Request) { update.HandleInstallUpdate(h, w, r) })
	registerPublicRoute(mux, "/api/version", func(w http.ResponseWriter, r *http.Request) { update.HandleVersion(h, w, r) })

	// Rules
	registerProtectedRoute(mux, "/api/rules/apply", authMiddleware, func(w http.ResponseWriter, r *http.Request) { rules.HandleApplyRule(h, w, r) })

	// Scripts
	registerProtectedRoute(mux, "/api/scripts/dir", authMiddleware, func(w http.ResponseWriter, r *http.Request) { script.HandleGetScriptsDir(h, w, r) })
	registerProtectedRoute(mux, "/api/scripts/open", authMiddleware, func(w http.ResponseWriter, r *http.Request) { script.HandleOpenScriptsDir(h, w, r) })
	registerProtectedRoute(mux, "/api/scripts/list", authMiddleware, func(w http.ResponseWriter, r *http.Request) { script.HandleListScripts(h, w, r) })

	// Media
	registerProtectedRoute(mux, "/api/media/proxy", authMiddleware, func(w http.ResponseWriter, r *http.Request) { media.HandleMediaProxy(h, w, r) })
	registerProtectedRoute(mux, "/api/media/cleanup", authMiddleware, func(w http.ResponseWriter, r *http.Request) { media.HandleMediaCacheCleanup(h, w, r) })
	// Media cache info is public for display in settings
	registerPublicRoute(mux, "/api/media/info", func(w http.ResponseWriter, r *http.Request) { media.HandleMediaCacheInfo(h, w, r) })
	registerProtectedRoute(mux, "/api/webpage/proxy", authMiddleware, func(w http.ResponseWriter, r *http.Request) { media.HandleWebpageProxy(h, w, r) })
	registerProtectedRoute(mux, "/api/webpage/resource", authMiddleware, func(w http.ResponseWriter, r *http.Request) { media.HandleWebpageResource(h, w, r) })

	// Network
	registerProtectedRoute(mux, "/api/network/detect", authMiddleware, func(w http.ResponseWriter, r *http.Request) { networkhandlers.HandleDetectNetwork(h, w, r) })
	registerProtectedRoute(mux, "/api/network/info", authMiddleware, func(w http.ResponseWriter, r *http.Request) { networkhandlers.HandleGetNetworkInfo(h, w, r) })
	registerProtectedRoute(mux, "/api/network/test-proxy", authMiddleware, func(w http.ResponseWriter, r *http.Request) { networkhandlers.HandleTestProxy(h, w, r) })
	registerProtectedRoute(mux, "/api/network/test-custom-proxy", authMiddleware, func(w http.ResponseWriter, r *http.Request) { networkhandlers.HandleTestCustomProxy(h, w, r) })

	// Browser
	registerProtectedRoute(mux, "/api/browser/open", authMiddleware, func(w http.ResponseWriter, r *http.Request) { browser.HandleOpenURL(h, w, r) })

	// Custom CSS
	registerProtectedRoute(mux, "/api/custom-css/upload-dialog", authMiddleware, func(w http.ResponseWriter, r *http.Request) { customcss.HandleUploadCSSDialog(h, w, r) })
	registerProtectedRoute(mux, "/api/custom-css/upload", authMiddleware, func(w http.ResponseWriter, r *http.Request) { customcss.HandleUploadCSS(h, w, r) })
	registerProtectedRoute(mux, "/api/custom-css", authMiddleware, func(w http.ResponseWriter, r *http.Request) { customcss.HandleGetCSS(h, w, r) })
	registerProtectedRoute(mux, "/api/custom-css/delete", authMiddleware, func(w http.ResponseWriter, r *http.Request) { customcss.HandleDeleteCSS(h, w, r) })

	// Window
	registerPublicRoute(mux, "/api/window/state", func(w http.ResponseWriter, r *http.Request) { window.HandleGetWindowState(h, w, r) })
	registerPublicRoute(mux, "/api/window/save", func(w http.ResponseWriter, r *http.Request) { window.HandleSaveWindowState(h, w, r) })

	// FreshRSS
	registerProtectedRoute(mux, "/api/freshrss/sync", authMiddleware, func(w http.ResponseWriter, r *http.Request) { freshrssHandler.HandleSync(h, w, r) })
	registerProtectedRoute(mux, "/api/freshrss/sync-feed", authMiddleware, func(w http.ResponseWriter, r *http.Request) { freshrssHandler.HandleSyncFeed(h, w, r) })
	registerProtectedRoute(mux, "/api/freshrss/status", authMiddleware, func(w http.ResponseWriter, r *http.Request) { freshrssHandler.HandleSyncStatus(h, w, r) })
}
