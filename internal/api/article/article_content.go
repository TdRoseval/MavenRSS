package article

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"

	"MavenRSS/internal/feed"
	"MavenRSS/internal/api/core"
	"MavenRSS/internal/api/response"
)

// HandleGetArticleContent fetches the article content from RSS feed dynamically.
// @Summary      Get article content
// @Description  Fetch the full HTML content of an article (uses cache if available)
// @Tags         articles
// @Accept       json
// @Produce      json
// @Param        id   query     int64   true  "Article ID"
// @Success      200  {object}  map[string]string  "Article content (content, feed_url)"
// @Failure      400  {object}  map[string]string  "Bad request (invalid article ID)"
// @Failure      404  {object}  map[string]string  "Article not found"
// @Failure      500  {object}  map[string]string  "Internal server error"
// @Router       /articles/content [get]
func HandleGetArticleContent(h *core.Handler, w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		response.Error(w, nil, http.StatusMethodNotAllowed)
		return
	}

	articleIDStr := r.URL.Query().Get("id")
	articleID, err := strconv.ParseInt(articleIDStr, 10, 64)
	if err != nil {
		response.Error(w, nil, http.StatusBadRequest)
		return
	}

	// Get the article from database to access feed_id
	article, err := h.DB.GetArticleByID(articleID)
	if err != nil {
		log.Printf("Error getting article: %v", err)
		response.Error(w, err, http.StatusInternalServerError)
		return
	}
	if article == nil {
		response.Error(w, nil, http.StatusNotFound)
		return
	}

	// Use the cached content fetching method
	content, wasCached, err := h.GetArticleContent(articleID)
	if err != nil {
		log.Printf("Error getting article content: %v", err)
		response.Error(w, err, http.StatusInternalServerError)
		return
	}

	// Track article view
	_ = h.DB.IncrementStat("article_view")

	// Get feed URL to use as referer for image proxying
	feed, err := h.DB.GetFeedByID(article.FeedID)
	var feedURL string
	if err == nil && feed != nil {
		feedURL = feed.URL
	}

	response.JSON(w, map[string]interface{}{
		"content":  content,
		"feed_url": feedURL,
		"cached":   wasCached,
	})
}

// HandleFetchFullArticle fetches the full article content from the original URL using readability.
// @Summary      Fetch full article content
// @Description  Fetch the full article content from the original URL using readability extraction (requires full_text_fetch_enabled setting)
// @Tags         articles
// @Accept       json
// @Produce      json
// @Param        id   query     int64   true  "Article ID"
// @Success      200  {object}  map[string]string  "Full article content (content, feed_url)"
// @Failure      400  {object}  map[string]string  "Bad request (invalid ID or missing URL)"
// @Failure      403  {object}  map[string]string  "Full-text fetching disabled"
// @Failure      404  {object}  map[string]string  "Article not found"
// @Failure      500  {object}  map[string]string  "Internal server error"
// @Router       /articles/fetch-full [post]
func HandleFetchFullArticle(h *core.Handler, w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		response.Error(w, nil, http.StatusMethodNotAllowed)
		return
	}

	articleIDStr := r.URL.Query().Get("id")
	articleID, err := strconv.ParseInt(articleIDStr, 10, 64)
	if err != nil {
		response.Error(w, nil, http.StatusBadRequest)
		return
	}

	// Get the article from database
	article, err := h.DB.GetArticleByID(articleID)
	if err != nil {
		log.Printf("Error getting article: %v", err)
		response.Error(w, err, http.StatusInternalServerError)
		return
	}
	if article == nil {
		response.Error(w, nil, http.StatusNotFound)
		return
	}

	if article.URL == "" {
		response.Error(w, nil, http.StatusBadRequest)
		return
	}

	// Check if full-text fetching is enabled (global setting only)
	// auto_expand_content only affects auto-expansion behavior, not manual button clicks
	fullTextEnabledStr, _ := h.DB.GetSetting("full_text_fetch_enabled")
	if fullTextEnabledStr != "true" {
		response.Error(w, nil, http.StatusForbidden)
		return
	}

	// Fetch full content
	fullContent, err := h.FetchFullArticleContent(article.URL)
	if err != nil {
		log.Printf("Error fetching full article content: %v", err)
		response.Error(w, err, http.StatusInternalServerError)
		return
	}

	// Get feed URL to use as referer for image proxying
	feed, err := h.DB.GetFeedByID(article.FeedID)
	var feedURL string
	if err == nil && feed != nil {
		feedURL = feed.URL
	}

	response.JSON(w, map[string]string{
		"content":  fullContent,
		"feed_url": feedURL,
	})
}

// HandleExtractAllImages extracts all image URLs from article content
// @Summary      Extract all images from article
// @Description  Extract all image URLs from article content (including relative URLs resolved to absolute)
// @Tags         articles
// @Accept       json
// @Produce      json
// @Param        id   query     int64   true  "Article ID"
// @Success      200  {object}  map[string]interface{}  "List of image URLs (images, feed_url)"
// @Failure      400  {object}  map[string]string  "Bad request (invalid article ID)"
// @Failure      404  {object}  map[string]string  "Article not found"
// @Failure      500  {object}  map[string]string  "Internal server error"
// @Router       /articles/extract-images [get]
func HandleExtractAllImages(h *core.Handler, w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		response.Error(w, nil, http.StatusMethodNotAllowed)
		return
	}

	articleIDStr := r.URL.Query().Get("id")
	articleID, err := strconv.ParseInt(articleIDStr, 10, 64)
	if err != nil {
		response.Error(w, nil, http.StatusBadRequest)
		return
	}

	// Get the article from database
	article, err := h.DB.GetArticleByID(articleID)
	if err != nil {
		log.Printf("Error getting article: %v", err)
		response.Error(w, err, http.StatusInternalServerError)
		return
	}
	if article == nil {
		response.Error(w, nil, http.StatusNotFound)
		return
	}

	// Get feed URL to use as base for resolving relative URLs
	feedObj, err := h.DB.GetFeedByID(article.FeedID)
	var feedURL string
	if err == nil && feedObj != nil {
		feedURL = feedObj.URL
	}

	// Get article content
	content, _, err := h.GetArticleContent(articleID)
	if err != nil {
		log.Printf("Error getting article content: %v", err)
		response.Error(w, err, http.StatusInternalServerError)
		return
	}

	// Extract all images from content
	rawImageURLs := feed.ExtractAllImageURLsFromHTML(content)

	// Resolve all relative URLs to absolute
	var resolvedImageURLs []string
	for _, imgURL := range rawImageURLs {
		resolvedURL := feed.ResolveRelativeURL(imgURL, feedURL)
		if resolvedURL != "" {
			resolvedImageURLs = append(resolvedImageURLs, resolvedURL)
		}
	}

	response.JSON(w, map[string]interface{}{
		"images":   resolvedImageURLs,
		"feed_url": feedURL,
	})
}

// HandleGetArticleTranslatedContent retrieves cached translated article content.
// @Summary      Get translated article content
// @Description  Get cached translated article content from database
// @Tags         articles
// @Accept       json
// @Produce      json
// @Param        id           query     int64   true  "Article ID"
// @Param        target_lang  query     string  true  "Target language"
// @Success      200  {object}  map[string]interface{}  "Translated content (content, provider, cached)"
// @Failure      400  {object}  map[string]string  "Bad request"
// @Router       /articles/translated-content [get]
func HandleGetArticleTranslatedContent(h *core.Handler, w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		response.Error(w, nil, http.StatusMethodNotAllowed)
		return
	}

	articleIDStr := r.URL.Query().Get("id")
	articleID, err := strconv.ParseInt(articleIDStr, 10, 64)
	if err != nil {
		response.Error(w, nil, http.StatusBadRequest)
		return
	}

	targetLang := r.URL.Query().Get("target_lang")
	if targetLang == "" {
		response.Error(w, nil, http.StatusBadRequest)
		return
	}

	content, provider, found, err := h.DB.GetArticleTranslatedContent(articleID, targetLang)
	if err != nil {
		log.Printf("Error getting translated content: %v", err)
		response.Error(w, err, http.StatusInternalServerError)
		return
	}

	response.JSON(w, map[string]interface{}{
		"content":  content,
		"provider": provider,
		"cached":   found,
	})
}

// HandleSetArticleTranslatedContent saves translated article content to database.
// @Summary      Save translated article content
// @Description  Save translated article content to database for caching
// @Tags         articles
// @Accept       json
// @Produce      json
// @Param        request  body      object  true  "Translated content request (article_id, content, target_lang, provider)"
// @Success      200  {object}  map[string]bool  "Success status"
// @Failure      400  {object}  map[string]string  "Bad request"
// @Router       /articles/translated-content [post]
func HandleSetArticleTranslatedContent(h *core.Handler, w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		response.Error(w, nil, http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		ArticleID  int64  `json:"article_id"`
		Content    string `json:"content"`
		TargetLang string `json:"target_lang"`
		Provider   string `json:"provider"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, err, http.StatusBadRequest)
		return
	}

	if req.ArticleID == 0 || req.Content == "" || req.TargetLang == "" {
		response.Error(w, nil, http.StatusBadRequest)
		return
	}

	if req.Provider == "" {
		req.Provider = "unknown"
	}

	err := h.DB.SetArticleTranslatedContent(req.ArticleID, req.Content, req.TargetLang, req.Provider)
	if err != nil {
		log.Printf("Error saving translated content: %v", err)
		response.Error(w, err, http.StatusInternalServerError)
		return
	}

	response.JSON(w, map[string]bool{"success": true})
}

// HandleClearArticleTranslatedContents clears all translated article contents.
// @Summary      Clear translated contents
// @Description  Clear all cached translated article contents
// @Tags         articles
// @Accept       json
// @Produce      json
// @Success      200  {object}  map[string]bool  "Success status"
// @Router       /articles/clear-translated-contents [post]
func HandleClearArticleTranslatedContents(h *core.Handler, w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		response.Error(w, nil, http.StatusMethodNotAllowed)
		return
	}

	if err := h.DB.ClearAllArticleTranslatedContents(); err != nil {
		log.Printf("Error clearing translated contents: %v", err)
		response.Error(w, err, http.StatusInternalServerError)
		return
	}

	response.JSON(w, map[string]bool{"success": true})
}
