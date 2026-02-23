package routes

import (
	"net/http"

	article "MavenRSS/internal/handlers/article"
	"MavenRSS/internal/handlers/core"
	summary "MavenRSS/internal/handlers/summary"
	translationhandlers "MavenRSS/internal/handlers/translation"
	"MavenRSS/internal/middleware"
)

// registerArticleRoutes registers all article-related routes
func registerArticleRoutes(mux *http.ServeMux, h *core.Handler, cfg Config) {
	var authMiddleware middleware.Middleware
	if cfg.EnableAuth && cfg.JWTManager != nil {
		authMiddleware = middleware.AuthMiddleware(cfg.JWTManager)
	}

	// Article CRUD and status
	registerProtectedRoute(mux, "/api/articles", authMiddleware, func(w http.ResponseWriter, r *http.Request) { article.HandleArticles(h, w, r) })
	registerProtectedRoute(mux, "/api/articles/images", authMiddleware, func(w http.ResponseWriter, r *http.Request) { article.HandleImageGalleryArticles(h, w, r) })
	registerProtectedRoute(mux, "/api/articles/filter", authMiddleware, func(w http.ResponseWriter, r *http.Request) { article.HandleFilteredArticles(h, w, r) })
	registerProtectedRoute(mux, "/api/articles/read", authMiddleware, func(w http.ResponseWriter, r *http.Request) { article.HandleMarkReadWithImmediateSync(h, w, r) })
	registerProtectedRoute(mux, "/api/articles/favorite", authMiddleware, func(w http.ResponseWriter, r *http.Request) { article.HandleToggleFavoriteWithImmediateSync(h, w, r) })
	registerProtectedRoute(mux, "/api/articles/mark-relative", authMiddleware, func(w http.ResponseWriter, r *http.Request) { article.HandleMarkRelativeToArticle(h, w, r) })
	registerProtectedRoute(mux, "/api/articles/toggle-hide", authMiddleware, func(w http.ResponseWriter, r *http.Request) { article.HandleToggleHideArticle(h, w, r) })
	registerProtectedRoute(mux, "/api/articles/toggle-read-later", authMiddleware, func(w http.ResponseWriter, r *http.Request) { article.HandleToggleReadLater(h, w, r) })
	registerProtectedRoute(mux, "/api/articles/mark-all-read", authMiddleware, func(w http.ResponseWriter, r *http.Request) { article.HandleMarkAllAsRead(h, w, r) })
	registerProtectedRoute(mux, "/api/articles/clear-read-later", authMiddleware, func(w http.ResponseWriter, r *http.Request) { article.HandleClearReadLater(h, w, r) })

	// Article content
	registerProtectedRoute(mux, "/api/articles/content", authMiddleware, func(w http.ResponseWriter, r *http.Request) { article.HandleGetArticleContent(h, w, r) })
	registerProtectedRoute(mux, "/api/articles/fetch-full", authMiddleware, func(w http.ResponseWriter, r *http.Request) { article.HandleFetchFullArticle(h, w, r) })
	registerProtectedRoute(mux, "/api/articles/extract-images", authMiddleware, func(w http.ResponseWriter, r *http.Request) { article.HandleExtractAllImages(h, w, r) })

	// Article statistics
	registerProtectedRoute(mux, "/api/articles/unread-counts", authMiddleware, func(w http.ResponseWriter, r *http.Request) { article.HandleGetUnreadCounts(h, w, r) })
	registerProtectedRoute(mux, "/api/articles/filter-counts", authMiddleware, func(w http.ResponseWriter, r *http.Request) { article.HandleGetFilterCounts(h, w, r) })

	// Article cleanup
	registerProtectedRoute(mux, "/api/articles/cleanup", authMiddleware, func(w http.ResponseWriter, r *http.Request) { article.HandleCleanupArticles(h, w, r) })
	registerProtectedRoute(mux, "/api/articles/cleanup-content", authMiddleware, func(w http.ResponseWriter, r *http.Request) { article.HandleCleanupArticleContent(h, w, r) })
	registerProtectedRoute(mux, "/api/articles/content-cache-info", authMiddleware, func(w http.ResponseWriter, r *http.Request) { article.HandleGetArticleContentCacheInfo(h, w, r) })

	// Translation
	registerProtectedRoute(mux, "/api/articles/translate", authMiddleware, func(w http.ResponseWriter, r *http.Request) { translationhandlers.HandleTranslateArticle(h, w, r) })
	registerProtectedRoute(mux, "/api/articles/translate-text", authMiddleware, func(w http.ResponseWriter, r *http.Request) { translationhandlers.HandleTranslateText(h, w, r) })
	registerProtectedRoute(mux, "/api/articles/clear-translations", authMiddleware, func(w http.ResponseWriter, r *http.Request) { translationhandlers.HandleClearTranslations(h, w, r) })

	// AI usage (translation related)
	registerProtectedRoute(mux, "/api/ai-usage", authMiddleware, func(w http.ResponseWriter, r *http.Request) { translationhandlers.HandleGetAIUsage(h, w, r) })
	registerProtectedRoute(mux, "/api/ai-usage/reset", authMiddleware, func(w http.ResponseWriter, r *http.Request) { translationhandlers.HandleResetAIUsage(h, w, r) })
	registerProtectedRoute(mux, "/api/translation/test-custom", authMiddleware, func(w http.ResponseWriter, r *http.Request) { translationhandlers.HandleTestCustomTranslation(h, w, r) })

	// Summary
	registerProtectedRoute(mux, "/api/articles/summarize", authMiddleware, func(w http.ResponseWriter, r *http.Request) { summary.HandleSummarizeArticle(h, w, r) })
	registerProtectedRoute(mux, "/api/articles/clear-summaries", authMiddleware, func(w http.ResponseWriter, r *http.Request) { summary.HandleClearSummaries(h, w, r) })

	// Export
	registerProtectedRoute(mux, "/api/articles/export/obsidian", authMiddleware, func(w http.ResponseWriter, r *http.Request) { article.HandleExportToObsidian(h, w, r) })
	registerProtectedRoute(mux, "/api/articles/export/notion", authMiddleware, func(w http.ResponseWriter, r *http.Request) { article.HandleExportToNotion(h, w, r) })
}

func registerProtectedRoute(mux *http.ServeMux, pattern string, authMiddleware middleware.Middleware, handler http.HandlerFunc) {
	if authMiddleware != nil {
		mux.Handle(pattern, authMiddleware(http.HandlerFunc(handler)))
	} else {
		mux.HandleFunc(pattern, handler)
	}
}

func registerPublicRoute(mux *http.ServeMux, pattern string, handler http.HandlerFunc) {
	mux.HandleFunc(pattern, handler)
}
