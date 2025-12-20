// Package core contains the main Handler struct and core HTTP handlers for the application.
// It defines the Handler struct which holds dependencies like the database and fetcher.
package core

import (
	"context"
	"sync"
	"time"

	"MrRSS/internal/aiusage"
	"MrRSS/internal/cache"
	"MrRSS/internal/database"
	"MrRSS/internal/discovery"
	"MrRSS/internal/feed"
	"MrRSS/internal/translation"
	"MrRSS/internal/utils"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// Discovery timeout constants
const (
	// SingleFeedDiscoveryTimeout is the timeout for discovering feeds from a single source
	SingleFeedDiscoveryTimeout = 90 * time.Second
	// BatchDiscoveryTimeout is the timeout for discovering feeds from all sources
	BatchDiscoveryTimeout = 5 * time.Minute
)

// DiscoveryState represents the current state of a discovery operation
type DiscoveryState struct {
	IsRunning  bool                       `json:"is_running"`
	Progress   discovery.Progress         `json:"progress"`
	Feeds      []discovery.DiscoveredBlog `json:"feeds,omitempty"`
	Error      string                     `json:"error,omitempty"`
	IsComplete bool                       `json:"is_complete"`
}

// Handler holds all dependencies for HTTP handlers.
type Handler struct {
	DB               *database.DB
	Fetcher          *feed.Fetcher
	Translator       translation.Translator
	AITracker        *aiusage.Tracker
	DiscoveryService *discovery.Service
	App              *application.App    // Wails app instance for browser integration
	ContentCache     *cache.ContentCache // Cache for article content

	// Discovery state tracking for polling-based progress
	DiscoveryMu          sync.RWMutex
	SingleDiscoveryState *DiscoveryState
	BatchDiscoveryState  *DiscoveryState
}

// NewHandler creates a new Handler with the given dependencies.
func NewHandler(db *database.DB, fetcher *feed.Fetcher, translator translation.Translator) *Handler {
	return &Handler{
		DB:               db,
		Fetcher:          fetcher,
		Translator:       translator,
		AITracker:        aiusage.NewTracker(db),
		DiscoveryService: discovery.NewService(),
		ContentCache:     cache.NewContentCache(100, 30*time.Minute), // Cache up to 100 articles for 30 minutes
	}
}

// SetApp sets the Wails application instance for browser integration.
// This is called after app initialization in main.go.
func (h *Handler) SetApp(app *application.App) {
	h.App = app
}

// GetArticleContent fetches article content with caching
func (h *Handler) GetArticleContent(articleID int64) (string, error) {
	// Check cache first
	if content, found := h.ContentCache.Get(articleID); found {
		return content, nil
	}

	// Get the article from database
	article, err := h.DB.GetArticleByID(articleID)
	if err != nil {
		return "", err
	}

	// Get the feed
	feeds, err := h.DB.GetFeeds()
	if err != nil {
		return "", err
	}

	var targetFeed *struct {
		URL        string
		ScriptPath string
	}
	for _, f := range feeds {
		if f.ID == article.FeedID {
			targetFeed = &struct {
				URL        string
				ScriptPath string
			}{
				URL:        f.URL,
				ScriptPath: f.ScriptPath,
			}
			break
		}
	}

	if targetFeed == nil {
		return "", nil
	}

	// Parse the feed to get fresh content
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	parsedFeed, err := h.Fetcher.ParseFeedWithScript(ctx, targetFeed.URL, targetFeed.ScriptPath, true) // High priority for content fetching
	if err != nil {
		return "", err
	}

	// Find the article in the feed by URL
	for _, item := range parsedFeed.Items {
		if utils.URLsMatch(item.Link, article.URL) {
			content := feed.ExtractContent(item)
			cleanContent := utils.CleanHTML(content)

			// Cache the content
			h.ContentCache.Set(articleID, cleanContent)

			return cleanContent, nil
		}
	}

	return "", nil
}
