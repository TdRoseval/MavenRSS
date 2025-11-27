package feed

import (
	"MrRSS/internal/database"
	"MrRSS/internal/models"
	"MrRSS/internal/rules"
	"MrRSS/internal/translation"
	"MrRSS/internal/utils"
	"context"
	"log"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/mmcdole/gofeed"
)

// FeedParser interface to allow mocking
type FeedParser interface {
	ParseURL(url string) (*gofeed.Feed, error)
	ParseURLWithContext(url string, ctx context.Context) (*gofeed.Feed, error)
}

type Fetcher struct {
	db             *database.DB
	fp             FeedParser
	translator     translation.Translator
	scriptExecutor *ScriptExecutor
	progress       Progress
	mu             sync.Mutex
}

type Progress struct {
	Total     int  `json:"total"`
	Current   int  `json:"current"`
	IsRunning bool `json:"is_running"`
}

func NewFetcher(db *database.DB, translator translation.Translator) *Fetcher {
	// Initialize script executor with scripts directory
	scriptsDir, err := utils.GetScriptsDir()
	var executor *ScriptExecutor
	if err == nil {
		executor = NewScriptExecutor(scriptsDir)
	}

	return &Fetcher{
		db:             db,
		fp:             gofeed.NewParser(),
		translator:     translator,
		scriptExecutor: executor,
	}
}

func (f *Fetcher) GetProgress() Progress {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.progress
}

func (f *Fetcher) FetchAll(ctx context.Context) {
	f.mu.Lock()
	if f.progress.IsRunning {
		f.mu.Unlock()
		return
	}
	f.progress.IsRunning = true
	f.progress.Current = 0
	f.mu.Unlock()

	// Setup translator based on settings
	provider, _ := f.db.GetSetting("translation_provider")
	apiKey, _ := f.db.GetSetting("deepl_api_key")

	var t translation.Translator
	if provider == "deepl" && apiKey != "" {
		t = translation.NewDeepLTranslator(apiKey)
	} else {
		t = translation.NewGoogleFreeTranslator()
	}
	f.translator = t

	feeds, err := f.db.GetFeeds()
	if err != nil {
		log.Println("Error getting feeds:", err)
		f.mu.Lock()
		f.progress.IsRunning = false
		f.mu.Unlock()
		return
	}

	f.mu.Lock()
	f.progress.Total = len(feeds)
	f.mu.Unlock()

	var wg sync.WaitGroup
	sem := make(chan struct{}, 5) // Limit concurrency

	for _, feed := range feeds {
		// Check for cancellation
		select {
		case <-ctx.Done():
			log.Println("FetchAll cancelled (loop)")
			goto Finish
		default:
		}

		wg.Add(1)
		sem <- struct{}{}
		go func(fd models.Feed) {
			defer wg.Done()
			defer func() { <-sem }()

			// Check for cancellation inside goroutine before starting
			select {
			case <-ctx.Done():
				return
			default:
			}

			f.FetchFeed(ctx, fd)
			f.mu.Lock()
			f.progress.Current++
			f.mu.Unlock()
		}(feed)
	}

Finish:
	wg.Wait()

	f.mu.Lock()
	f.progress.IsRunning = false
	f.mu.Unlock()

	// Update last article update time
	f.db.SetSetting("last_article_update", time.Now().Format(time.RFC3339))
}

func (f *Fetcher) FetchFeed(ctx context.Context, feed models.Feed) {
	var parsedFeed *gofeed.Feed
	var err error

	// Check if this feed uses a custom script
	if feed.ScriptPath != "" {
		// Execute the custom script to fetch feed
		if f.scriptExecutor == nil {
			log.Printf("Script executor not initialized for feed %s", feed.Title)
			f.db.UpdateFeedError(feed.ID, "Script executor not initialized")
			return
		}
		parsedFeed, err = f.scriptExecutor.ExecuteScript(ctx, feed.ScriptPath)
		if err != nil {
			log.Printf("Error executing script for feed %s: %v", feed.Title, err)
			f.db.UpdateFeedError(feed.ID, err.Error())
			return
		}
	} else {
		// Use traditional URL-based fetching
		parsedFeed, err = f.fp.ParseURLWithContext(feed.URL, ctx)
		if err != nil {
			log.Printf("Error parsing feed %s: %v", feed.URL, err)
			f.db.UpdateFeedError(feed.ID, err.Error())
			return
		}
	}

	// Clear any previous error on successful fetch
	f.db.UpdateFeedError(feed.ID, "")

	// Update Feed Image if available and not set
	if feed.ImageURL == "" && parsedFeed.Image != nil {
		f.db.UpdateFeedImage(feed.ID, parsedFeed.Image.URL)
	}

	// Update Feed Link if available and not set
	if feed.Link == "" && parsedFeed.Link != "" {
		f.db.UpdateFeedLink(feed.ID, parsedFeed.Link)
	}

	// Check translation settings
	translationEnabledStr, _ := f.db.GetSetting("translation_enabled")
	targetLang, _ := f.db.GetSetting("target_language")
	translationEnabled := translationEnabledStr == "true"

	var articlesToSave []*models.Article

	for _, item := range parsedFeed.Items {
		published := time.Now()
		if item.PublishedParsed != nil {
			published = *item.PublishedParsed
		}

		imageURL := ""
		if item.Image != nil {
			imageURL = item.Image.URL
		} else if len(item.Enclosures) > 0 && item.Enclosures[0].Type == "image/jpeg" { // Simple check
			imageURL = item.Enclosures[0].URL
		}

		// Fallback: Try to find image in description/content
		if imageURL == "" {
			content := item.Content
			if content == "" {
				content = item.Description
			}
			re := regexp.MustCompile(`<img[^>]+src="([^">]+)"`)
			matches := re.FindStringSubmatch(content)
			if len(matches) > 1 {
				imageURL = matches[1]
			}
		}

		translatedTitle := ""
		if translationEnabled && f.translator != nil {
			t, err := f.translator.Translate(item.Title, targetLang)
			if err == nil {
				translatedTitle = t
			}
		}

		// Extract content from RSS item (prefer Content over Description)
		content := item.Content
		if content == "" {
			content = item.Description
		}

		// Generate title if missing
		title := item.Title
		if title == "" {
			title = generateTitleFromContent(content)
		}

		article := &models.Article{
			FeedID:          feed.ID,
			Title:           title,
			URL:             item.Link,
			ImageURL:        imageURL,
			Content:         content,
			PublishedAt:     published,
			TranslatedTitle: translatedTitle,
		}
		articlesToSave = append(articlesToSave, article)
	}

	// Check context before heavy DB operation
	select {
	case <-ctx.Done():
		return
	default:
	}

	if len(articlesToSave) > 0 {
		if err := f.db.SaveArticles(ctx, articlesToSave); err != nil {
			log.Printf("Error saving articles for feed %s: %v", feed.Title, err)
		} else {
			// Apply rules to newly saved articles
			// We fetch the recent articles for this feed since SaveArticles doesn't return IDs
			// This is limited to the number of articles we just saved
			savedArticles, err := f.db.GetArticles("", feed.ID, "", false, len(articlesToSave), 0)
			if err == nil && len(savedArticles) > 0 {
				engine := rules.NewEngine(f.db)
				affected, err := engine.ApplyRulesToArticles(savedArticles)
				if err != nil {
					log.Printf("Error applying rules for feed %s: %v", feed.Title, err)
				} else if affected > 0 {
					log.Printf("Applied rules to %d articles in feed %s", affected, feed.Title)
				}
			}
		}
	}
	log.Printf("Updated feed: %s", feed.Title)
}

func (f *Fetcher) AddSubscription(url string, category string, customTitle string) error {
	parsedFeed, err := f.fp.ParseURL(url)
	if err != nil {
		return err
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
func (f *Fetcher) AddScriptSubscription(scriptPath string, category string, customTitle string) error {
	// Validate script path
	if f.scriptExecutor == nil {
		return &ScriptError{Message: "script executor not initialized"}
	}

	// Execute script to get initial feed info
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	parsedFeed, err := f.scriptExecutor.ExecuteScript(ctx, scriptPath)
	if err != nil {
		return err
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

// ScriptError represents an error related to script execution
type ScriptError struct {
	Message string
}

func (e *ScriptError) Error() string {
	return e.Message
}

func (f *Fetcher) ImportSubscription(title, url, category string) error {
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

// generateTitleFromContent generates a title from content when title is missing
func generateTitleFromContent(content string) string {
	if content == "" {
		return "Untitled Article"
	}

	// Remove HTML tags
	htmlTagRegex := regexp.MustCompile(`<[^>]+>`)
	plainText := htmlTagRegex.ReplaceAllString(content, "")

	// Trim whitespace
	plainText = strings.TrimSpace(plainText)

	// Limit to 100 characters
	if len(plainText) > 100 {
		plainText = plainText[:100] + "..."
	}

	// If still empty after cleaning, use default
	if plainText == "" {
		return "Untitled Article"
	}

	return plainText
}
