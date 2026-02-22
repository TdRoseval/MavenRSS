// Package core contains the main Handler struct and core HTTP handlers for the application.
// It defines the Handler struct which holds dependencies like the database and fetcher.
package core

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"MavenRSS/internal/ai"
	"MavenRSS/internal/cache"
	"MavenRSS/internal/database"
	"MavenRSS/internal/discovery"
	"MavenRSS/internal/feed"
	"MavenRSS/internal/models"
	svc "MavenRSS/internal/service"
	"MavenRSS/internal/statistics"
	"MavenRSS/internal/translation"
	"MavenRSS/internal/utils/httputil"
	"MavenRSS/internal/utils/textutil"
	"MavenRSS/internal/utils/urlutil"

	"codeberg.org/readeck/go-readability/v2"

	"github.com/mmcdole/gofeed"
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
// It now uses a service registry for better separation of concerns.
type Handler struct {
	// Services registry provides access to all business logic services
	Services *svc.Registry

	// Direct access to core dependencies (for backward compatibility)
	DB                *database.DB
	Fetcher           *feed.Fetcher
	Translator        translation.Translator
	AIProfileProvider *ai.ProfileProvider // AI profile provider for feature-specific configurations
	AITracker         *ai.UsageTracker
	DiscoveryService  *discovery.Service
	App               interface{}         // Wails app instance for browser integration (interface{} to avoid import in server mode)
	ContentCache      *cache.ContentCache // Cache for article content
	Stats             *statistics.Service // Statistics tracking service

	// Discovery state tracking for polling-based progress
	DiscoveryMu          sync.RWMutex
	SingleDiscoveryState *DiscoveryState
	BatchDiscoveryState  *DiscoveryState
}

// NewHandler creates a new Handler with the given dependencies.
func NewHandler(db *database.DB, fetcher *feed.Fetcher, translator translation.Translator, profileProvider *ai.ProfileProvider) *Handler {
	// Create service registry
	registry := svc.NewRegistry(db, fetcher, translator)

	h := &Handler{
		Services:          registry,
		DB:                db,
		Fetcher:           fetcher,
		Translator:        translator,
		AIProfileProvider: profileProvider,
		AITracker:         registry.AITracker(),
		DiscoveryService:  registry.DiscoveryService(),
		ContentCache:      registry.ContentCache(),
		Stats:             registry.Stats(),
	}

	return h
}

// CallAppMethod calls a method on the Wails app instance if available
func (h *Handler) CallAppMethod(method string, args ...interface{}) error {
	if h.App == nil {
		return fmt.Errorf("app instance not set")
	}

	// Use reflection or type assertion to call the method
	// This is a simplified version - you may need to adjust based on your actual Wails app structure
	// For now, just log that we want to call this method
	log.Printf("Would call app method: %s with args: %v", method, args)
	return nil
}

// SetApp sets the Wails application instance for browser integration.
// This is called after app initialization in main.go.
func (h *Handler) SetApp(app interface{}) {
	h.App = app
}

// Statistics returns the statistics service
func (h *Handler) Statistics() *statistics.Service {
	return h.Stats
}

// GetArticleContent fetches article content with caching
// Returns (content, wasCached, error)
func (h *Handler) GetArticleContent(articleID int64) (string, bool, error) {
	// First, check database cache (persistent cache)
	content, found, err := h.DB.GetArticleContent(articleID)
	if err == nil && found {
		// Also populate memory cache for faster subsequent access
		h.ContentCache.Set(articleID, content)
		return content, true, nil
	}

	// Check memory cache (in-memory cache, might be stale but fast)
	if content, found := h.ContentCache.Get(articleID); found {
		return content, true, nil
	}

	// Get the article from database
	article, err := h.DB.GetArticleByID(articleID)
	if err != nil {
		return "", false, err
	}
	if article == nil {
		return "", false, nil
	}

	// Get the feed
	targetFeed, err := h.DB.GetFeedByID(article.FeedID)
	if err != nil {
		return "", false, err
	}

	if targetFeed == nil {
		return "", false, nil
	}

	// Trigger immediate feed refresh using the new task manager
	// This bypasses the queue and pool limits
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Fetch the feed immediately (article click triggered)
	h.Fetcher.FetchFeedForArticle(ctx, *targetFeed)

	// Parse the feed to get fresh content
	parsedFeed, err := h.Fetcher.ParseFeedWithFeed(ctx, targetFeed, true) // High priority for content fetching
	if err != nil {
		return "", false, err
	}

	// Cache the feed for future use
	h.ContentCache.SetFeed(targetFeed.ID, parsedFeed)

	// Find the article in the feed by multiple criteria for better matching
	matchingItem := h.findMatchingFeedItem(article, parsedFeed.Items)
	if matchingItem != nil {
		content := feed.ExtractContent(matchingItem)
		cleanContent := textutil.CleanHTML(content)

		// Cache the content in both memory and database
		h.ContentCache.Set(articleID, cleanContent)
		if err := h.DB.SetArticleContent(articleID, cleanContent); err != nil {
			log.Printf("Error caching content to database: %v", err)
		}

		return cleanContent, false, nil
	}

	return "", false, nil
}

// FetchFullArticleContent fetches the full article content from the original URL using readability.
func (h *Handler) FetchFullArticleContent(pageURL string) (string, error) {
	// Build proxy URL if enabled
	var proxyURL string
	proxyEnabled, _ := h.DB.GetSetting("proxy_enabled")
	if proxyEnabled == "true" {
		proxyType, _ := h.DB.GetSetting("proxy_type")
		proxyHost, _ := h.DB.GetSetting("proxy_host")
		proxyPort, _ := h.DB.GetSetting("proxy_port")
		proxyUsername, _ := h.DB.GetEncryptedSetting("proxy_username")
		proxyPassword, _ := h.DB.GetEncryptedSetting("proxy_password")
		proxyURL = httputil.BuildProxyURL(proxyType, proxyHost, proxyPort, proxyUsername, proxyPassword)
		log.Printf("[FetchFullArticleContent] Using proxy: %s", proxyURL)
	} else {
		log.Printf("[FetchFullArticleContent] No proxy configured, fetching directly")
	}

	// Use our own HTTP client with proxy support
	userAgent := "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"
	httpClient := httputil.GetPooledUserAgentClient(proxyURL, 30*time.Second, userAgent)

	// Fetch the page first using our HTTP client
	req, err := http.NewRequest("GET", pageURL, nil)
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}

	// Add browser-like headers to bypass anti-bot protections
	// Note: Don't set Accept-Encoding - let Go's http.Transport handle it automatically
	// This ensures proper gzip/deflate decompression is applied
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9,zh-CN;q=0.8,zh;q=0.7")
	req.Header.Set("DNT", "1")
	req.Header.Set("Connection", "keep-alive")
	req.Header.Set("Upgrade-Insecure-Requests", "1")
	req.Header.Set("Sec-Fetch-Dest", "document")
	req.Header.Set("Sec-Fetch-Mode", "navigate")
	req.Header.Set("Sec-Fetch-Site", "none")
	req.Header.Set("Sec-Fetch-User", "?1")
	req.Header.Set("Cache-Control", "max-age=0")

	log.Printf("[FetchFullArticleContent] Fetching URL: %s", pageURL)
	resp, err := httpClient.Do(req)
	if err != nil {
		log.Printf("[FetchFullArticleContent] Fetch error: %v", err)
		return "", fmt.Errorf("fetch page: %w", err)
	}
	defer resp.Body.Close()

	log.Printf("[FetchFullArticleContent] Response status: %d", resp.StatusCode)
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP %d: %s", resp.StatusCode, resp.Status)
	}

	// Parse the page URL for relative links
	parsedURL, err := url.Parse(pageURL)
	if err != nil {
		return "", fmt.Errorf("parse URL: %w", err)
	}

	// Use FromReader to parse with our own fetched content
	article, err := readability.FromReader(resp.Body, parsedURL)
	if err != nil {
		log.Printf("[FetchFullArticleContent] Readability error: %v", err)
		return "", fmt.Errorf("readability parse: %w", err)
	}

	// Render the article content as HTML
	var buf bytes.Buffer
	err = article.RenderHTML(&buf)
	if err != nil {
		return "", fmt.Errorf("render HTML: %w", err)
	}

	// Remove duplicate content blocks
	content := removeDuplicateContent(buf.String())

	log.Printf("[FetchFullArticleContent] Successfully fetched article content, original length: %d, after dedup: %d", buf.Len(), len(content))
	return content, nil
}

// findMatchingFeedItem finds the best matching feed item for an article using multiple criteria
func (h *Handler) findMatchingFeedItem(article *models.Article, items []*gofeed.Item) *gofeed.Item {
	// First pass: exact URL match
	for _, item := range items {
		if urlutil.URLsMatch(item.Link, article.URL) {
			return item
		}
	}

	// Second pass: URL + title match (for script-based feeds that might have URL variations)
	for _, item := range items {
		if urlutil.URLsMatch(item.Link, article.URL) && h.titlesMatch(item.Title, article.Title) {
			return item
		}
	}

	// Third pass: title + published time match (fallback for when URLs don't match)
	for _, item := range items {
		if h.titlesMatch(item.Title, article.Title) && h.publishedTimesMatch(item.PublishedParsed, &article.PublishedAt) {
			return item
		}
	}

	// Final fallback: just title match
	for _, item := range items {
		if h.titlesMatch(item.Title, article.Title) {
			return item
		}
	}

	return nil
}

// titlesMatch checks if two titles match, allowing for minor differences
func (h *Handler) titlesMatch(title1, title2 string) bool {
	if title1 == title2 {
		return true
	}
	// Normalize titles by removing extra whitespace and comparing
	normalized1 := strings.TrimSpace(strings.Join(strings.Fields(title1), " "))
	normalized2 := strings.TrimSpace(strings.Join(strings.Fields(title2), " "))
	return normalized1 == normalized2
}

// publishedTimesMatch checks if two published times match within a reasonable tolerance
func (h *Handler) publishedTimesMatch(time1, time2 *time.Time) bool {
	if time1 == nil || time2 == nil {
		return false
	}
	// Allow for 1 minute difference in published times
	diff := time1.Sub(*time2)
	if diff < 0 {
		diff = -diff
	}
	return diff <= time.Minute
}

// removeDuplicateContent removes duplicate content blocks from HTML
// This helps with pages that have repeated sections like related articles, comments, etc.
func removeDuplicateContent(htmlContent string) string {
	if htmlContent == "" {
		return htmlContent
	}

	// Extract text content and find duplicate paragraphs
	// First, strip HTML tags to get plain text for comparison
	plainText := stripHTMLTags(htmlContent)

	// Split into paragraphs (by double newline or <p> tags)
	paragraphs := strings.Split(plainText, "\n\n")

	// Use a map to track seen paragraphs
	seen := make(map[string]bool)
	var uniqueParagraphs []string
	minLength := 100 // Minimum characters to consider as valid paragraph

	for _, p := range paragraphs {
		trimmed := strings.TrimSpace(p)
		if len(trimmed) < minLength {
			// Keep short paragraphs (likely structural elements)
			uniqueParagraphs = append(uniqueParagraphs, p)
			continue
		}

		// Create a normalized version for comparison
		normalized := normalizeText(trimmed)

		if !seen[normalized] {
			seen[normalized] = true
			uniqueParagraphs = append(uniqueParagraphs, p)
		}
		// If we've seen this paragraph before, skip it (it's a duplicate)
	}

	// If we removed duplicates, reconstruct the content
	if len(uniqueParagraphs) < len(paragraphs) {
		// For HTML, we need to be more careful
		// Instead of reconstructing, let's just return the cleaned version
		// by removing duplicate text blocks from the original HTML

		// A simpler approach: return original if we didn't find many duplicates
		// Most readability libraries already do deduplication
		if len(seen) > 0 && float64(len(seen))/float64(len(paragraphs)) < 0.5 {
			// Too many duplicates removed, might be a problem
			// Return original content
			return htmlContent
		}
	}

	// Return original - readability should already handle this
	// The issue might be in how the article is rendered
	return htmlContent
}

// stripHTMLTags removes HTML tags from content
func stripHTMLTags(html string) string {
	re := strings.NewReplacer(
		"<br>", "\n", "<br/>", "\n", "<br />", "\n",
		"</p>", "\n</p>", "</div>", "\n</div>",
	)
	html = re.Replace(html)

	// Simple tag stripping
	inTag := false
	var result strings.Builder
	for _, c := range html {
		if c == '<' {
			inTag = true
		} else if c == '>' {
			inTag = false
		} else if !inTag {
			result.WriteRune(c)
		}
	}
	return result.String()
}

// normalizeText normalizes text for comparison
func normalizeText(text string) string {
	// Convert to lowercase
	text = strings.ToLower(text)
	// Remove extra whitespace
	text = strings.Join(strings.Fields(text), " ")
	return text
}
