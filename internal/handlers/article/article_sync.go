package article

import (
	"context"
	"log"
	"net/http"
	"strconv"

	"MavenRSS/internal/database"
	"MavenRSS/internal/freshrss"
	"MavenRSS/internal/handlers/core"
	"MavenRSS/internal/handlers/response"
	"MavenRSS/internal/utils/httputil"
)

// HandleMarkReadWithImmediateSync marks an article as read/unread and immediately syncs to FreshRSS
// @Summary      Mark article as read/unread with immediate FreshRSS sync
// @Description  Mark a specific article as read or unread and immediately sync to FreshRSS if configured
// @Tags         articles
// @Accept       json
// @Produce      json
// @Param        id   query     int64   true  "Article ID"
// @Param        read query     string  true  "Read status: 'true', '1', 'false', or '0'"  Enums(true, 1, false, 0)
// @Success      200  {string}  string  "Article marked and sync triggered successfully"
// @Failure      500  {object}  map[string]string  "Internal server error"
// @Router       /articles/mark-read-sync [post]
func HandleMarkReadWithImmediateSync(h *core.Handler, w http.ResponseWriter, r *http.Request) {
	idStr := r.URL.Query().Get("id")
	id, _ := strconv.ParseInt(idStr, 10, 64)

	readStr := r.URL.Query().Get("read")
	read := true
	if readStr == "false" || readStr == "0" {
		read = false
	}

	// Mark as read and get sync request
	syncReq, err := h.DB.MarkArticleReadWithSync(id, read)
	if err != nil {
		response.Error(w, err, http.StatusInternalServerError)
		return
	}

	response.JSON(w, map[string]bool{"success": true})

	// Immediately sync to FreshRSS if needed
	if syncReq != nil {
		go performImmediateSync(h, syncReq)
	}
}

// HandleToggleFavoriteWithImmediateSync toggles favorite and immediately syncs to FreshRSS
// @Summary      Toggle article favorite status with immediate FreshRSS sync
// @Description  Toggle the favorite/starred status of an article and immediately sync to FreshRSS if configured
// @Tags         articles
// @Accept       json
// @Produce      json
// @Param        id   query     int64   true  "Article ID"
// @Success      200  {string}  string  "Favorite toggled and sync triggered successfully"
// @Failure      500  {object}  map[string]string  "Internal server error"
// @Router       /articles/toggle-favorite-sync [post]
func HandleToggleFavoriteWithImmediateSync(h *core.Handler, w http.ResponseWriter, r *http.Request) {
	idStr := r.URL.Query().Get("id")
	id, _ := strconv.ParseInt(idStr, 10, 64)

	// Toggle favorite and get sync request
	syncReq, err := h.DB.ToggleFavoriteWithSync(id)
	if err != nil {
		response.Error(w, err, http.StatusInternalServerError)
		return
	}

	response.JSON(w, map[string]bool{"success": true})

	// Immediately sync to FreshRSS if needed
	if syncReq != nil {
		go performImmediateSync(h, syncReq)
	}
}

// performImmediateSync performs an immediate sync to FreshRSS in a background goroutine
func performImmediateSync(h *core.Handler, syncReq *database.SyncRequest) {
	// Check if FreshRSS is enabled and configured
	enabled, _ := h.DB.GetSetting("freshrss_enabled")
	if enabled != "true" {
		return
	}

	serverURL, username, password, err := h.DB.GetFreshRSSConfig()
	if err != nil || serverURL == "" || username == "" || password == "" {
		log.Printf("[Immediate Sync] FreshRSS not configured, skipping sync")
		return
	}

	// Build proxy URL if enabled
	var proxyURL string
	proxyEnabled, _ := h.DB.GetSetting("proxy_enabled")
	if proxyEnabled == "true" {
		proxyType, _ := h.DB.GetSetting("proxy_type")
		proxyHost, _ := h.DB.GetSetting("proxy_host")
		proxyPort, _ := h.DB.GetSetting("proxy_port")
		proxyUsername, _ := h.DB.GetEncryptedSetting("proxy_username")
		proxyPassword, _ := h.DB.GetEncryptedSetting("proxy_password")
		proxyURL = buildProxyURL(proxyType, proxyHost, proxyPort, proxyUsername, proxyPassword)
	}

	// Create sync service with proxy support
	syncService := freshrss.NewBidirectionalSyncServiceWithProxy(serverURL, username, password, proxyURL, h.DB)

	// Perform immediate sync
	ctx := context.Background()
	err = syncService.SyncArticleStatus(ctx, syncReq.ArticleID, syncReq.ArticleURL, syncReq.Action)
	if err != nil {
		log.Printf("[Immediate Sync] Failed for article %d: %v", syncReq.ArticleID, err)
		// Enqueue for retry during next global sync
		_ = h.DB.EnqueueSyncChange(syncReq.ArticleID, syncReq.ArticleURL, syncReq.Action)
		log.Printf("[Immediate Sync] Enqueued article %d for retry", syncReq.ArticleID)
	} else {
		log.Printf("[Immediate Sync] Success for article %d: %s", syncReq.ArticleID, syncReq.Action)
	}
}

func buildProxyURL(proxyType, proxyHost, proxyPort, proxyUsername, proxyPassword string) string {
	return httputil.BuildProxyURL(proxyType, proxyHost, proxyPort, proxyUsername, proxyPassword)
}
