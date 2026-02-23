package feed

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"MavenRSS/internal/handlers/core"
	"MavenRSS/internal/handlers/response"
	"MavenRSS/internal/models"
	"MavenRSS/internal/rsshub"
	"MavenRSS/internal/utils/httputil"
	"MavenRSS/internal/utils/urlutil"
)

// HandleFeeds returns all feeds.
// @Summary      Get all feeds
// @Description  Retrieve all RSS feed subscriptions (passwords are cleared)
// @Tags         feeds
// @Accept       json
// @Produce      json
// @Success      200  {array}   models.Feed  "List of feeds"
// @Failure      500  {object}  map[string]string  "Internal server error"
// @Router       /feeds [get]
func HandleFeeds(h *core.Handler, w http.ResponseWriter, r *http.Request) {
	userID, ok := core.GetUserIDFromRequest(r)
	var feeds []models.Feed
	var err error
	if ok {
		feeds, err = h.DB.GetFeedsForUser(userID)
	} else {
		feeds, err = h.DB.GetFeeds()
	}
	if err != nil {
		response.Error(w, err, http.StatusInternalServerError)
		return
	}

	// Populate tags for each feed
	for i := range feeds {
		tags, _ := h.DB.GetFeedTags(feeds[i].ID)
		feeds[i].Tags = tags
		// Clear sensitive password fields before sending to frontend
		feeds[i].EmailPassword = ""
	}

	response.JSON(w, feeds)
}

// HandleAddFeed adds a new feed subscription and immediately fetches its articles.
// @Summary      Add a new feed
// @Description  Add a new RSS/Atom/Email/Script/XPath feed subscription
// @Tags         feeds
// @Accept       json
// @Produce      json
// @Param        request  body      object  true  "Feed details"
// @Success      200  {string}  string  "Feed added successfully"
// @Failure      400  {object}  map[string]string  "Bad request"
// @Failure      409  {object}  map[string]string  "Feed URL already exists"
// @Failure      500  {object}  map[string]string  "Internal server error"
// @Router       /feeds/add [post]
func HandleAddFeed(h *core.Handler, w http.ResponseWriter, r *http.Request) {
	userID, ok := core.GetUserIDFromRequest(r)
	if !ok {
		response.Error(w, nil, http.StatusUnauthorized)
		return
	}

	var req struct {
		URL              string `json:"url"`
		Category         string `json:"category"`
		Title            string `json:"title"`
		ScriptPath       string `json:"script_path"`
		HideFromTimeline bool   `json:"hide_from_timeline"`
		ProxyURL         string `json:"proxy_url"`
		ProxyEnabled     bool   `json:"proxy_enabled"`
		RefreshInterval  int    `json:"refresh_interval"`
		IsImageMode      bool   `json:"is_image_mode"`
		// XPath fields
		Type                string `json:"type"`
		XPathItem           string `json:"xpath_item"`
		XPathItemTitle      string `json:"xpath_item_title"`
		XPathItemContent    string `json:"xpath_item_content"`
		XPathItemUri        string `json:"xpath_item_uri"`
		XPathItemAuthor     string `json:"xpath_item_author"`
		XPathItemTimestamp  string `json:"xpath_item_timestamp"`
		XPathItemTimeFormat string `json:"xpath_item_time_format"`
		XPathItemThumbnail  string `json:"xpath_item_thumbnail"`
		XPathItemCategories string `json:"xpath_item_categories"`
		XPathItemUid        string `json:"xpath_item_uid"`
		ArticleViewMode     string `json:"article_view_mode"`
		AutoExpandContent   string `json:"auto_expand_content"`
		// Email/Newsletter fields
		EmailAddress    string `json:"email_address"`
		EmailIMAPServer string `json:"email_imap_server"`
		EmailIMAPPort   int    `json:"email_imap_port"`
		EmailUsername   string `json:"email_username"`
		EmailPassword   string `json:"email_password"`
		EmailFolder     string `json:"email_folder"`
		// Tags
		Tags []int64 `json:"tags"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, err, http.StatusBadRequest)
		return
	}

	// Normalize the URL to ensure it has a protocol
	req.URL = urlutil.NormalizeFeedURL(req.URL)

	// Determine the feed URL to check for duplicates
	feedURL := req.URL
	if req.ScriptPath != "" {
		feedURL = "script://" + req.ScriptPath
	} else if req.Type == "email" {
		feedURL = "email://" + req.EmailAddress
	}

	// Check if feed with this URL already exists (excluding FreshRSS feeds) for this user
	var existingID int64
	var existingIsFreshRSS bool
	err := h.DB.QueryRow("SELECT id, is_freshrss_source FROM feeds WHERE user_id = ? AND url = ?", userID, feedURL).Scan(&existingID, &existingIsFreshRSS)
	if err == nil && !existingIsFreshRSS {
		// Feed exists and is not a FreshRSS feed - return conflict error
		response.Error(w, err, http.StatusConflict)
		return
	}

	var feedID int64
	
	// Get user proxy settings as fallback
	var userProxyURL string
	proxyEnabled := req.ProxyEnabled
	if proxyEnabled {
		proxyEnabledStr, _ := h.DB.GetSettingWithFallback(userID, "proxy_enabled")
		if proxyEnabledStr == "true" {
			proxyEnabled = true
			proxyType, _ := h.DB.GetSettingWithFallback(userID, "proxy_type")
			proxyHost, _ := h.DB.GetSettingWithFallback(userID, "proxy_host")
			proxyPort, _ := h.DB.GetSettingWithFallback(userID, "proxy_port")
			proxyUsername, _ := h.DB.GetEncryptedSettingWithFallback(userID, "proxy_username")
			proxyPassword, _ := h.DB.GetEncryptedSettingWithFallback(userID, "proxy_password")
			userProxyURL = httputil.BuildProxyURL(proxyType, proxyHost, proxyPort, proxyUsername, proxyPassword)
		}
	} else {
		proxyEnabledStr, _ := h.DB.GetSettingWithFallback(userID, "proxy_enabled")
		if proxyEnabledStr == "true" {
			proxyEnabled = true
			proxyType, _ := h.DB.GetSettingWithFallback(userID, "proxy_type")
			proxyHost, _ := h.DB.GetSettingWithFallback(userID, "proxy_host")
			proxyPort, _ := h.DB.GetSettingWithFallback(userID, "proxy_port")
			proxyUsername, _ := h.DB.GetEncryptedSettingWithFallback(userID, "proxy_username")
			proxyPassword, _ := h.DB.GetEncryptedSettingWithFallback(userID, "proxy_password")
			userProxyURL = httputil.BuildProxyURL(proxyType, proxyHost, proxyPort, proxyUsername, proxyPassword)
		}
	}
	
	// Use request proxy URL if provided, otherwise use user proxy settings
	finalProxyURL := req.ProxyURL
	if finalProxyURL == "" && userProxyURL != "" {
		finalProxyURL = userProxyURL
	}
	
	// First, create a basic feed with user_id
	feed := &models.Feed{
		Title:               req.Title,
		URL:                 feedURL,
		Category:            req.Category,
		ScriptPath:          req.ScriptPath,
		HideFromTimeline:    req.HideFromTimeline,
		ProxyURL:            finalProxyURL,
		ProxyEnabled:        proxyEnabled,
		RefreshInterval:     req.RefreshInterval,
		IsImageMode:         req.IsImageMode,
		Type:                req.Type,
		XPathItem:           req.XPathItem,
		XPathItemTitle:      req.XPathItemTitle,
		XPathItemContent:    req.XPathItemContent,
		XPathItemUri:        req.XPathItemUri,
		XPathItemAuthor:     req.XPathItemAuthor,
		XPathItemTimestamp:  req.XPathItemTimestamp,
		XPathItemTimeFormat: req.XPathItemTimeFormat,
		XPathItemThumbnail:  req.XPathItemThumbnail,
		XPathItemCategories: req.XPathItemCategories,
		XPathItemUid:        req.XPathItemUid,
		ArticleViewMode:     req.ArticleViewMode,
		AutoExpandContent:   req.AutoExpandContent,
		EmailAddress:        req.EmailAddress,
		EmailIMAPServer:     req.EmailIMAPServer,
		EmailIMAPPort:       req.EmailIMAPPort,
		EmailUsername:       req.EmailUsername,
		EmailPassword:       req.EmailPassword,
		EmailFolder:         req.EmailFolder,
		UserID:              userID,
	}

	// Add feed using AddFeedForUser
	feedID, err = h.DB.AddFeedForUser(userID, feed)
	if err != nil {
		response.Error(w, err, http.StatusInternalServerError)
		return
	}

	// Set tags for the feed
	if len(req.Tags) > 0 {
		if err := h.DB.SetFeedTags(feedID, req.Tags); err != nil {
			// Log error but don't fail - feed was created successfully
			// Tags can be set later via edit
		}
	}

	// Immediately fetch articles for the newly added feed in background
	go func() {
		feed, err := h.DB.GetFeedByIDForUser(userID, feedID)
		if err != nil || feed == nil {
			return
		}
		// Use manual refresh (queue head) for newly added feed
		h.Fetcher.FetchSingleFeed(context.Background(), *feed, true)
	}()

	w.WriteHeader(http.StatusOK)
}

// HandleDeleteFeed deletes a feed subscription.
// @Summary      Delete a feed
// @Description  Delete a feed subscription by ID
// @Tags         feeds
// @Accept       json
// @Produce      json
// @Param        id   query      int64  true  "Feed ID"
// @Success      200  {string}  string  "Feed deleted successfully"
// @Failure      500  {object}  map[string]string  "Internal server error"
// @Router       /feeds/delete [post]
func HandleDeleteFeed(h *core.Handler, w http.ResponseWriter, r *http.Request) {
	userID, ok := core.GetUserIDFromRequest(r)
	if !ok {
		response.Error(w, nil, http.StatusUnauthorized)
		return
	}

	idStr := r.URL.Query().Get("id")
	id, _ := strconv.ParseInt(idStr, 10, 64)

	// Verify that the feed belongs to this user
	feed, err := h.DB.GetFeedByIDForUser(userID, id)
	if err != nil {
		response.Error(w, err, http.StatusInternalServerError)
		return
	}
	if feed == nil {
		response.Error(w, nil, http.StatusNotFound)
		return
	}

	if err := h.DB.DeleteFeed(id); err != nil {
		response.Error(w, err, http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}

// HandleUpdateFeed updates a feed's properties.
// @Summary      Update a feed
// @Description  Update properties of an existing feed subscription
// @Tags         feeds
// @Accept       json
// @Produce      json
// @Param        request  body      object  true  "Feed update details"
// @Success      200  {string}  string  "Feed updated successfully"
// @Failure      400  {object}  map[string]string  "Bad request"
// @Failure      409  {object}  map[string]string  "Feed URL already exists"
// @Failure      500  {object}  map[string]string  "Internal server error"
// @Router       /feeds/update [post]
func HandleUpdateFeed(h *core.Handler, w http.ResponseWriter, r *http.Request) {
	userID, ok := core.GetUserIDFromRequest(r)
	if !ok {
		response.Error(w, nil, http.StatusUnauthorized)
		return
	}

	var req struct {
		ID               int64  `json:"id"`
		Title            string `json:"title"`
		URL              string `json:"url"`
		Category         string `json:"category"`
		ScriptPath       string `json:"script_path"`
		HideFromTimeline bool   `json:"hide_from_timeline"`
		ProxyURL         string `json:"proxy_url"`
		ProxyEnabled     bool   `json:"proxy_enabled"`
		RefreshInterval  int    `json:"refresh_interval"`
		IsImageMode      bool   `json:"is_image_mode"`
		// XPath fields
		Type                string `json:"type"`
		XPathItem           string `json:"xpath_item"`
		XPathItemTitle      string `json:"xpath_item_title"`
		XPathItemContent    string `json:"xpath_item_content"`
		XPathItemUri        string `json:"xpath_item_uri"`
		XPathItemAuthor     string `json:"xpath_item_author"`
		XPathItemTimestamp  string `json:"xpath_item_timestamp"`
		XPathItemTimeFormat string `json:"xpath_item_time_format"`
		XPathItemThumbnail  string `json:"xpath_item_thumbnail"`
		XPathItemCategories string `json:"xpath_item_categories"`
		XPathItemUid        string `json:"xpath_item_uid"`
		ArticleViewMode     string `json:"article_view_mode"`
		AutoExpandContent   string `json:"auto_expand_content"`
		// Email/Newsletter fields
		EmailAddress    string `json:"email_address"`
		EmailIMAPServer string `json:"email_imap_server"`
		EmailIMAPPort   int    `json:"email_imap_port"`
		EmailUsername   string `json:"email_username"`
		EmailPassword   string `json:"email_password"`
		EmailFolder     string `json:"email_folder"`
		// Tags
		Tags []int64 `json:"tags"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, err, http.StatusBadRequest)
		return
	}

	// Normalize the URL to ensure it has a protocol
	req.URL = urlutil.NormalizeFeedURL(req.URL)

	// Verify that the feed belongs to this user
	currentFeed, err := h.DB.GetFeedByIDForUser(userID, req.ID)
	if err != nil {
		response.Error(w, err, http.StatusInternalServerError)
		return
	}
	if currentFeed == nil {
		response.Error(w, nil, http.StatusNotFound)
		return
	}

	// Validate RSSHub URL if provided
	if req.URL != "" && rsshub.IsRSSHubURL(req.URL) {
		// Check if RSSHub is enabled
		enabledStr, _ := h.DB.GetSettingForUser(userID, "rsshub_enabled")
		if enabledStr != "true" {
			response.Error(w, nil, http.StatusBadRequest)
			return
		}

		endpoint, _ := h.DB.GetSettingForUser(userID, "rsshub_endpoint")
		if endpoint == "" {
			endpoint = "https://rsshub.app"
		}
		apiKey, _ := h.DB.GetEncryptedSettingForUser(userID, "rsshub_api_key")

		// Skip validation if API key is empty (public rsshub.app instance with Cloudflare protection)
		if apiKey != "" {
			route := rsshub.ExtractRoute(req.URL)

			// Build proxy URL if enabled
			var proxyURL string
			proxyEnabled, _ := h.DB.GetSettingForUser(userID, "proxy_enabled")
			if proxyEnabled == "true" {
				proxyType, _ := h.DB.GetSettingForUser(userID, "proxy_type")
				proxyHost, _ := h.DB.GetSettingForUser(userID, "proxy_host")
				proxyPort, _ := h.DB.GetSettingForUser(userID, "proxy_port")
				proxyUsername, _ := h.DB.GetEncryptedSettingForUser(userID, "proxy_username")
				proxyPassword, _ := h.DB.GetEncryptedSettingForUser(userID, "proxy_password")
				proxyURL = httputil.BuildProxyURL(proxyType, proxyHost, proxyPort, proxyUsername, proxyPassword)
			}

			client := rsshub.NewClientWithProxy(endpoint, apiKey, proxyURL)
			if err := client.ValidateRoute(route); err != nil {
				response.Error(w, err, http.StatusBadRequest)
				return
			}
		}
	}

	// Determine the feed URL to check for duplicates
	feedURL := req.URL
	if req.ScriptPath != "" {
		feedURL = "script://" + req.ScriptPath
	} else if req.Type == "email" {
		feedURL = "email://" + req.EmailAddress
	}

	// Check if another feed with this URL already exists (excluding FreshRSS feeds and current feed) for this user
	var existingID int64
	var existingIsFreshRSS bool
	err = h.DB.QueryRow("SELECT id, is_freshrss_source FROM feeds WHERE user_id = ? AND url = ? AND id != ?", userID, feedURL, req.ID).Scan(&existingID, &existingIsFreshRSS)
	if err == nil && !existingIsFreshRSS {
		// Another feed exists with this URL and is not a FreshRSS feed - return conflict error
		response.Error(w, err, http.StatusConflict)
		return
	}

	// If title is empty, fetch the default title from the feed
	finalTitle := req.Title
	if finalTitle == "" {
		// Fetch default title based on feed type
		if req.ScriptPath != "" || currentFeed.ScriptPath != "" {
			// Script-based feed
			scriptPathToUse := req.ScriptPath
			if scriptPathToUse == "" {
				scriptPathToUse = currentFeed.ScriptPath
			}
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			parsedFeed, err := h.Fetcher.ParseFeedWithScript(ctx, "", scriptPathToUse, false)
			if err == nil && parsedFeed.Title != "" {
				finalTitle = parsedFeed.Title
			} else {
				finalTitle = scriptPathToUse
			}
		} else if req.XPathItem != "" || currentFeed.XPathItem != "" {
			// XPath-based feed: use default title
			finalTitle = "XPath Feed"
		} else if req.Type == "email" || currentFeed.Type == "email" {
			// Email-based feed: use email address as title
			emailAddr := req.EmailAddress
			if emailAddr == "" {
				emailAddr = currentFeed.EmailAddress
			}
			finalTitle = emailAddr
		} else {
			// URL-based feed: fetch and parse to get title
			urlToFetch := req.URL
			if urlToFetch == "" {
				urlToFetch = currentFeed.URL
			}
			if urlToFetch != "" {
				ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
				defer cancel()
				parsedFeed, err := h.Fetcher.ParseFeedWithUserID(ctx, urlToFetch, userID)
				if err == nil && parsedFeed.Title != "" {
					finalTitle = parsedFeed.Title
				} else {
					// Fallback to URL as title
					finalTitle = urlToFetch
				}
			}
		}
	}

	// Get user proxy settings as fallback
	var userProxyURL string
	proxyEnabled := req.ProxyEnabled
	if proxyEnabled {
		proxyEnabledStr, _ := h.DB.GetSettingWithFallback(userID, "proxy_enabled")
		if proxyEnabledStr == "true" {
			proxyEnabled = true
			proxyType, _ := h.DB.GetSettingWithFallback(userID, "proxy_type")
			proxyHost, _ := h.DB.GetSettingWithFallback(userID, "proxy_host")
			proxyPort, _ := h.DB.GetSettingWithFallback(userID, "proxy_port")
			proxyUsername, _ := h.DB.GetEncryptedSettingWithFallback(userID, "proxy_username")
			proxyPassword, _ := h.DB.GetEncryptedSettingWithFallback(userID, "proxy_password")
			userProxyURL = httputil.BuildProxyURL(proxyType, proxyHost, proxyPort, proxyUsername, proxyPassword)
		}
	} else {
		proxyEnabledStr, _ := h.DB.GetSettingWithFallback(userID, "proxy_enabled")
		if proxyEnabledStr == "true" {
			proxyEnabled = true
			proxyType, _ := h.DB.GetSettingWithFallback(userID, "proxy_type")
			proxyHost, _ := h.DB.GetSettingWithFallback(userID, "proxy_host")
			proxyPort, _ := h.DB.GetSettingWithFallback(userID, "proxy_port")
			proxyUsername, _ := h.DB.GetEncryptedSettingWithFallback(userID, "proxy_username")
			proxyPassword, _ := h.DB.GetEncryptedSettingWithFallback(userID, "proxy_password")
			userProxyURL = httputil.BuildProxyURL(proxyType, proxyHost, proxyPort, proxyUsername, proxyPassword)
		}
	}
	
	// Use request proxy URL if provided, otherwise use user proxy settings
	finalProxyURL := req.ProxyURL
	if finalProxyURL == "" && userProxyURL != "" {
		finalProxyURL = userProxyURL
	}
	
	if err := h.DB.UpdateFeed(req.ID, finalTitle, req.URL, req.Category, req.ScriptPath, req.HideFromTimeline, finalProxyURL, proxyEnabled, req.RefreshInterval, req.IsImageMode, req.Type, req.XPathItem, req.XPathItemTitle, req.XPathItemContent, req.XPathItemUri, req.XPathItemAuthor, req.XPathItemTimestamp, req.XPathItemTimeFormat, req.XPathItemThumbnail, req.XPathItemCategories, req.XPathItemUid, req.ArticleViewMode, req.AutoExpandContent, req.EmailAddress, req.EmailIMAPServer, req.EmailUsername, req.EmailPassword, req.EmailFolder, req.EmailIMAPPort); err != nil {
		response.Error(w, err, http.StatusInternalServerError)
		return
	}

	// Update tags for the feed
	if req.Tags != nil {
		if err := h.DB.SetFeedTags(req.ID, req.Tags); err != nil {
			response.Error(w, err, http.StatusInternalServerError)
			return
		}
	}

	// Immediately fetch articles for the updated feed in background
	go func() {
		feed, err := h.DB.GetFeedByIDForUser(userID, req.ID)
		if err != nil || feed == nil {
			return
		}
		// Use manual refresh (queue head) for updated feed
		h.Fetcher.FetchSingleFeed(context.Background(), *feed, true)
	}()

	w.WriteHeader(http.StatusOK)
}

// HandleRefreshFeed refreshes a single feed by ID with progress tracking.
// @Summary      Refresh a single feed
// @Description  Trigger a refresh for a specific feed (runs in background with progress tracking)
// @Tags         feeds
// @Accept       json
// @Produce      json
// @Param        id   query     int64   true  "Feed ID"
// @Success      200  {string}  string  "Feed refresh started successfully"
// @Failure      400  {object}  map[string]string  "Bad request (invalid feed ID)"
// @Failure      404  {object}  map[string]string  "Feed not found"
// @Router       /feeds/refresh [post]
func HandleRefreshFeed(h *core.Handler, w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		response.Error(w, nil, http.StatusMethodNotAllowed)
		return
	}

	userID, ok := core.GetUserIDFromRequest(r)
	if !ok {
		response.Error(w, nil, http.StatusUnauthorized)
		return
	}

	idStr := r.URL.Query().Get("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		response.Error(w, err, http.StatusBadRequest)
		return
	}

	feed, err := h.DB.GetFeedByIDForUser(userID, id)
	if err != nil {
		response.Error(w, err, http.StatusInternalServerError)
		return
	}
	if feed == nil {
		response.Error(w, nil, http.StatusNotFound)
		return
	}

	// Refresh the feed in background with progress tracking (manual = queue head)
	go h.Fetcher.FetchSingleFeed(context.Background(), *feed, true)

	// Return success response
	response.JSON(w, map[string]string{"status": "refreshing"})
}

// HandleReorderFeed reorders a feed within or across categories.
// @Summary      Reorder a feed
// @Description  Change the position and optionally the category of a feed
// @Tags         feeds
// @Accept       json
// @Produce      json
// @Param        request  body      object  true  "Reorder details (feed_id, category, position)"
// @Success      200  {object}  map[string]string  "Reorder status"
// @Failure      400  {object}  map[string]string  "Bad request"
// @Failure      500  {object}  map[string]string  "Internal server error"
// @Router       /feeds/reorder [post]
func HandleReorderFeed(h *core.Handler, w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		response.Error(w, nil, http.StatusMethodNotAllowed)
		return
	}

	userID, ok := core.GetUserIDFromRequest(r)
	if !ok {
		response.Error(w, nil, http.StatusUnauthorized)
		return
	}

	var req struct {
		FeedID   int64  `json:"feed_id"`
		Category string `json:"category"`
		Position int    `json:"position"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, err, http.StatusBadRequest)
		return
	}

	// Verify that the feed belongs to this user
	feed, err := h.DB.GetFeedByIDForUser(userID, req.FeedID)
	if err != nil {
		response.Error(w, err, http.StatusInternalServerError)
		return
	}
	if feed == nil {
		response.Error(w, nil, http.StatusNotFound)
		return
	}

	if err := h.DB.ReorderFeed(req.FeedID, req.Category, req.Position); err != nil {
		response.Error(w, err, http.StatusInternalServerError)
		return
	}

	response.JSON(w, map[string]string{"status": "ok"})
}
