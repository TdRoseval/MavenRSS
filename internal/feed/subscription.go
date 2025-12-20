package feed

import (
	"MrRSS/internal/models"
	"context"
	"time"

	"github.com/mmcdole/gofeed"
)

// AddSubscription adds a new feed subscription and returns the feed ID.
func (f *Fetcher) AddSubscription(url string, category string, customTitle string) (int64, error) {
	parsedFeed, err := f.fp.ParseURL(url)
	if err != nil {
		return 0, err
	}

	title := parsedFeed.Title
	if customTitle != "" {
		title = customTitle
	}

	feed := &models.Feed{
		Title:       title,
		URL:         url,
		Link:        parsedFeed.Link,
		Description: parsedFeed.Description,
		Category:    category,
	}

	if parsedFeed.Image != nil {
		feed.ImageURL = parsedFeed.Image.URL
	}

	return f.db.AddFeed(feed)
}

// AddScriptSubscription adds a new feed subscription that uses a custom script
// and returns the feed ID.
func (f *Fetcher) AddScriptSubscription(scriptPath string, category string, customTitle string) (int64, error) {
	// Validate script path
	if f.scriptExecutor == nil {
		return 0, &ScriptError{Message: "script executor not initialized"}
	}

	// Execute script to get initial feed info
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	parsedFeed, err := f.scriptExecutor.ExecuteScript(ctx, scriptPath)
	if err != nil {
		return 0, err
	}

	title := parsedFeed.Title
	if customTitle != "" {
		title = customTitle
	}

	// Use a placeholder URL for script-based feeds
	url := "script://" + scriptPath

	feed := &models.Feed{
		Title:       title,
		URL:         url,
		Link:        parsedFeed.Link,
		Description: parsedFeed.Description,
		Category:    category,
		ScriptPath:  scriptPath,
	}

	if parsedFeed.Image != nil {
		feed.ImageURL = parsedFeed.Image.URL
	}

	return f.db.AddFeed(feed)
}

// ImportSubscription imports a feed subscription and returns the feed ID.
func (f *Fetcher) ImportSubscription(title, url, category string) (int64, error) {
	feed := &models.Feed{
		Title:    title,
		URL:      url,
		Link:     "", // Link will be fetched later when feed is refreshed
		Category: category,
	}
	return f.db.AddFeed(feed)
}

// ParseFeed parses an RSS feed from a URL and returns the parsed feed
func (f *Fetcher) ParseFeed(ctx context.Context, url string) (*gofeed.Feed, error) {
	return f.fp.ParseURLWithContext(url, ctx)
}

// ParseFeedWithScript parses an RSS feed, using a custom script if specified.
// If scriptPath is non-empty, it executes the script to get the feed content.
// Otherwise, it fetches from the URL as normal.
// priority: true for high-priority requests (like article content fetching), false for normal requests (like feed refresh)
func (f *Fetcher) ParseFeedWithScript(ctx context.Context, url string, scriptPath string, priority bool) (*gofeed.Feed, error) {
	if priority {
		// High priority requests get dedicated processing without interference from low priority operations
		f.priorityMu.Lock()
		defer f.priorityMu.Unlock()

		return f.parseFeedWithScriptInternal(ctx, url, scriptPath, true)
	}

	// Normal priority requests
	return f.parseFeedWithScriptInternal(ctx, url, scriptPath, false)
}

// parseFeedWithScriptInternal does the actual parsing work
func (f *Fetcher) parseFeedWithScriptInternal(ctx context.Context, url string, scriptPath string, priority bool) (*gofeed.Feed, error) {
	if scriptPath != "" {
		// Execute the custom script to fetch feed
		if f.scriptExecutor == nil {
			return nil, &ScriptError{Message: "Script executor not initialized"}
		}

		// For high priority requests, use shorter timeout
		scriptCtx := ctx
		if priority {
			var cancel context.CancelFunc
			scriptCtx, cancel = context.WithTimeout(ctx, 15*time.Second) // Shorter timeout for content fetching
			defer cancel()
		}

		return f.scriptExecutor.ExecuteScript(scriptCtx, scriptPath)
	}

	// Use traditional URL-based fetching
	// For high priority requests, use shorter timeout
	fetchCtx := ctx
	if priority {
		var cancel context.CancelFunc
		fetchCtx, cancel = context.WithTimeout(ctx, 15*time.Second) // Shorter timeout for content fetching
		defer cancel()
	}

	return f.fp.ParseURLWithContext(url, fetchCtx)
}

// ScriptError represents an error related to script execution
type ScriptError struct {
	Message string
}

func (e *ScriptError) Error() string {
	return e.Message
}
