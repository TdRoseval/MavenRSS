package article

import (
	"net/http"
	"strconv"

	"MavenRSS/internal/api/core"
	"MavenRSS/internal/api/response"
)

func convertIntMapToInt64(m map[int64]int) map[int64]int64 {
	result := make(map[int64]int64, len(m))
	for k, v := range m {
		result[k] = int64(v)
	}
	return result
}

// HandleGetUnreadCounts returns unread counts for all feeds.
// @Summary      Get unread counts
// @Description  Get total unread count and per-feed unread counts
// @Tags         articles
// @Accept       json
// @Produce       json
// @Success      200  {object}  map[string]interface{}  "Unread counts (total + feed_counts map)"
// @Failure      500  {object}  map[string]string  "Internal server error"
// @Router       /articles/unread-counts [get]
func HandleGetUnreadCounts(h *core.Handler, w http.ResponseWriter, r *http.Request) {
	userID, _ := core.GetUserIDFromRequest(r)
	useClusterCounts := r.URL.Query().Get("view") == "clusters"

	var totalCount int64
	var feedCounts map[int64]int64

	if useClusterCounts {
		totalCountInt, err := h.DB.GetTotalUnreadClusterCount(userID)
		if err != nil {
			response.Error(w, err, http.StatusInternalServerError)
			return
		}
		totalCount = int64(totalCountInt)

		countsInt, err := h.DB.GetUnreadClusterCountsForAllFeeds(userID)
		if err != nil {
			response.Error(w, err, http.StatusInternalServerError)
			return
		}
		feedCounts = convertIntMapToInt64(countsInt)
	} else {
		totalCountInt, err := h.DB.GetTotalUnreadCount(userID)
		if err != nil {
			response.Error(w, err, http.StatusInternalServerError)
			return
		}
		totalCount = int64(totalCountInt)

		countsInt, err := h.DB.GetUnreadCountsForAllFeeds(userID)
		if err != nil {
			response.Error(w, err, http.StatusInternalServerError)
			return
		}
		feedCounts = convertIntMapToInt64(countsInt)
	}

	resp := map[string]interface{}{
		"total":       totalCount,
		"feed_counts": feedCounts,
	}

	response.JSON(w, resp)
}

// HandleGetFilterCounts returns article counts for different filters (unread, favorites, read_later, images).
// @Summary      Get filter-specific feed counts
// @Description  Get per-feed counts for different filter types (unread, favorites, read_later, images)
// @Tags         articles
// @Accept       json
// @Produce       json
// @Success      200  {object}  map[string]interface{}  "Filter counts for all filter types"
// @Failure      500  {object}  map[string]string  "Internal server error"
// @Router       /articles/filter-counts [get]
func HandleGetFilterCounts(h *core.Handler, w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		response.Error(w, nil, http.StatusMethodNotAllowed)
		return
	}

	userID, _ := core.GetUserIDFromRequest(r)
	useClusterCounts := r.URL.Query().Get("view") == "clusters"

	var unreadCounts map[int64]int64
	var favoriteCounts map[int64]int64
	var favoriteUnreadCounts map[int64]int64
	var readLaterCounts map[int64]int64
	var readLaterUnreadCounts map[int64]int64
	var imageCounts map[int64]int64
	var imageUnreadCounts map[int64]int64

	if useClusterCounts {
		clusterCounts, err := h.DB.GetAllClusterFilterCountsForAllFeeds(userID)
		if err != nil {
			response.Error(w, err, http.StatusInternalServerError)
			return
		}
		unreadCounts = convertIntMapToInt64(clusterCounts.Unread)
		favoriteCounts = convertIntMapToInt64(clusterCounts.Favorites)
		favoriteUnreadCounts = convertIntMapToInt64(clusterCounts.FavoritesUnread)
		readLaterCounts = convertIntMapToInt64(clusterCounts.ReadLater)
		readLaterUnreadCounts = convertIntMapToInt64(clusterCounts.ReadLaterUnread)
		imageCounts = convertIntMapToInt64(clusterCounts.Images)
		imageUnreadCounts = convertIntMapToInt64(clusterCounts.ImagesUnread)
	} else {
		countsInt, err := h.DB.GetUnreadCountsForAllFeeds(userID)
		if err != nil {
			response.Error(w, err, http.StatusInternalServerError)
			return
		}
		unreadCounts = convertIntMapToInt64(countsInt)

		countsInt, err = h.DB.GetFavoriteCountsForAllFeeds(userID)
		if err != nil {
			response.Error(w, err, http.StatusInternalServerError)
			return
		}
		favoriteCounts = convertIntMapToInt64(countsInt)

		countsInt, err = h.DB.GetFavoriteUnreadCountsForAllFeeds(userID)
		if err != nil {
			response.Error(w, err, http.StatusInternalServerError)
			return
		}
		favoriteUnreadCounts = convertIntMapToInt64(countsInt)

		countsInt, err = h.DB.GetReadLaterCountsForAllFeeds(userID)
		if err != nil {
			response.Error(w, err, http.StatusInternalServerError)
			return
		}
		readLaterCounts = convertIntMapToInt64(countsInt)

		countsInt, err = h.DB.GetReadLaterUnreadCountsForAllFeeds(userID)
		if err != nil {
			response.Error(w, err, http.StatusInternalServerError)
			return
		}
		readLaterUnreadCounts = convertIntMapToInt64(countsInt)

		countsInt, err = h.DB.GetImageModeCountsForAllFeeds(userID)
		if err != nil {
			response.Error(w, err, http.StatusInternalServerError)
			return
		}
		imageCounts = convertIntMapToInt64(countsInt)

		countsInt, err = h.DB.GetImageUnreadCountsForAllFeeds(userID)
		if err != nil {
			response.Error(w, err, http.StatusInternalServerError)
			return
		}
		imageUnreadCounts = convertIntMapToInt64(countsInt)
	}

	resp := map[string]interface{}{
		"unread":            unreadCounts,
		"favorites":         favoriteCounts,
		"favorites_unread":  favoriteUnreadCounts,
		"read_later":        readLaterCounts,
		"read_later_unread": readLaterUnreadCounts,
		"images":            imageCounts,
		"images_unread":     imageUnreadCounts,
	}

	response.JSON(w, resp)
}

// HandleMarkAllAsRead marks all articles as read.
// @Summary      Mark all articles as read
// @Description  Mark all articles as read globally, by feed, or by category
// @Tags         articles
// @Accept       json
// @Produce      json
// @Param        feed_id   query     int64   false  "Mark all as read for specific feed ID"
// @Param        category  query     string  false  "Mark all as read for specific category"
// @Success      200  {string}  string  "Articles marked as read successfully"
// @Failure      400  {object}  map[string]string  "Bad request (invalid feed_id)"
// @Failure      500  {object}  map[string]string  "Internal server error"
// @Router       /articles/mark-all-read [post]
func HandleMarkAllAsRead(h *core.Handler, w http.ResponseWriter, r *http.Request) {
	feedIDStr := r.URL.Query().Get("feed_id")
	category := r.URL.Query().Get("category")

	var err error

	if feedIDStr != "" {
		feedID, parseErr := strconv.ParseInt(feedIDStr, 10, 64)
		if parseErr != nil {
			response.Error(w, parseErr, http.StatusBadRequest)
			return
		}
		err = h.DB.MarkAllAsReadForFeed(feedID)
	} else if category != "" {
		err = h.DB.MarkAllAsReadForCategory(category)
	} else {
		err = h.DB.MarkAllAsRead()
	}

	if err != nil {
		response.Error(w, err, http.StatusInternalServerError)
		return
	}

	response.JSON(w, map[string]string{"message": "success"})
}

// HandleCleanupOldArticles triggers layered article cleanup with user context.
func HandleCleanupOldArticles(h *core.Handler, w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		response.Error(w, nil, http.StatusMethodNotAllowed)
		return
	}

	userID, _ := core.GetUserIDFromRequest(r)

	count, err := h.DB.CleanupOldArticles(userID)
	if err != nil {
		response.Error(w, err, http.StatusInternalServerError)
		return
	}

	response.JSON(w, map[string]interface{}{
		"deleted": count,
		"message": "Cleanup completed",
	})
}

// HandleCleanupUnimportantArticles removes all unimportant articles (not read, not favorited, not read later).
func HandleCleanupUnimportantArticles(h *core.Handler, w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		response.Error(w, nil, http.StatusMethodNotAllowed)
		return
	}

	userID, _ := core.GetUserIDFromRequest(r)

	count, err := h.DB.CleanupUnimportantArticles(userID)
	if err != nil {
		response.Error(w, err, http.StatusInternalServerError)
		return
	}

	response.JSON(w, map[string]interface{}{
		"deleted": count,
		"message": "Unimportant articles cleaned up",
	})
}

// HandleCleanupArticleContents removes old article content cache entries.
func HandleCleanupArticleContents(h *core.Handler, w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		response.Error(w, nil, http.StatusMethodNotAllowed)
		return
	}

	userID, _ := core.GetUserIDFromRequest(r)

	if h.Fetcher != nil {
		if manager := h.Fetcher.GetAIEnhancedManager(); manager != nil {
			manager.InterruptUserWork(userID)
		}
	}

	stats, err := h.DB.CleanupArticleCachePreservingFavorites(userID)
	if err != nil {
		response.Error(w, err, http.StatusInternalServerError)
		return
	}

	if h.ContentCache != nil {
		h.ContentCache.Clear()
	}

	response.JSON(w, map[string]interface{}{
		"deleted":           stats.DeletedArticles,
		"deleted_articles":  stats.DeletedArticles,
		"deleted_clusters":  stats.DeletedClusters,
		"retained_articles": stats.RetainedArticles,
		"retained_clusters": stats.RetainedClusters,
		"entries_cleaned":   stats.DeletedArticles,
		"message":           "Articles cleaned up while preserving favorites",
	})
}

// HandleDeleteAllArticles removes all articles for the current user.
func HandleDeleteAllArticles(h *core.Handler, w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		response.Error(w, nil, http.StatusMethodNotAllowed)
		return
	}

	userID, _ := core.GetUserIDFromRequest(r)

	count, err := h.DB.DeleteAllArticles(userID)
	if err != nil {
		response.Error(w, err, http.StatusInternalServerError)
		return
	}

	response.JSON(w, map[string]interface{}{
		"deleted": count,
		"message": "All articles deleted",
	})
}

// HandleGetDatabaseSize returns the current database size in MB.
func HandleGetDatabaseSize(h *core.Handler, w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		response.Error(w, nil, http.StatusMethodNotAllowed)
		return
	}

	sizeMB, err := h.DB.GetDatabaseSizeMB()
	if err != nil {
		response.Error(w, err, http.StatusInternalServerError)
		return
	}

	response.JSON(w, map[string]float64{"size_mb": sizeMB})
}

// HandleMarkRelativeToArticle marks articles relative to a reference article based on published time.
func HandleMarkRelativeToArticle(h *core.Handler, w http.ResponseWriter, r *http.Request) {
	response.JSON(w, map[string]string{"message": "not implemented"})
}

// HandleClearReadLater clears all read later articles.
func HandleClearReadLater(h *core.Handler, w http.ResponseWriter, r *http.Request) {
	response.JSON(w, map[string]string{"message": "not implemented"})
}

// HandleCleanupArticles is an alias for HandleCleanupOldArticles.
func HandleCleanupArticles(h *core.Handler, w http.ResponseWriter, r *http.Request) {
	HandleCleanupOldArticles(h, w, r)
}

// HandleCleanupArticleContent is an alias for HandleCleanupArticleContents.
func HandleCleanupArticleContent(h *core.Handler, w http.ResponseWriter, r *http.Request) {
	HandleCleanupArticleContents(h, w, r)
}

// HandleGetArticleContentCacheInfo returns article content cache information.
func HandleGetArticleContentCacheInfo(h *core.Handler, w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		response.Error(w, nil, http.StatusMethodNotAllowed)
		return
	}

	userID, _ := core.GetUserIDFromRequest(r)

	count, err := h.DB.GetArticleContentCount(userID)
	if err != nil {
		response.Error(w, err, http.StatusInternalServerError)
		return
	}

	response.JSON(w, map[string]interface{}{
		"cached_articles": count,
		"count":           count,
		"size":            0,
	})
}

// HandleClearArticlesForFeed clears all articles for a specific feed.
func HandleClearArticlesForFeed(h *core.Handler, w http.ResponseWriter, r *http.Request) {
	response.JSON(w, map[string]string{"message": "not implemented"})
}
