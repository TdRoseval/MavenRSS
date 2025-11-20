package feed

import (
	"MrRSS/internal/database"
	"MrRSS/internal/models"
	"MrRSS/internal/translation"
	"log"
	"regexp"
	"sync"
	"time"

	"github.com/mmcdole/gofeed"
)

// FeedParser interface to allow mocking
type FeedParser interface {
	ParseURL(url string) (*gofeed.Feed, error)
}

type Fetcher struct {
	db         *database.DB
	fp         FeedParser
	translator translation.Translator
	progress   Progress
	mu         sync.Mutex
}

type Progress struct {
	Total     int  `json:"total"`
	Current   int  `json:"current"`
	IsRunning bool `json:"is_running"`
}

func NewFetcher(db *database.DB, translator translation.Translator) *Fetcher {
	return &Fetcher{
		db:         db,
		fp:         gofeed.NewParser(),
		translator: translator,
	}
}

func (f *Fetcher) GetProgress() Progress {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.progress
}

func (f *Fetcher) FetchAll() {
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
		wg.Add(1)
		sem <- struct{}{}
		go func(fd models.Feed) {
			defer wg.Done()
			defer func() { <-sem }()
			f.FetchFeed(fd)
			f.mu.Lock()
			f.progress.Current++
			f.mu.Unlock()
		}(feed)
	}
	wg.Wait()

	f.mu.Lock()
	f.progress.IsRunning = false
	f.mu.Unlock()
}

func (f *Fetcher) FetchFeed(feed models.Feed) {
	parsedFeed, err := f.fp.ParseURL(feed.URL)
	if err != nil {
		log.Printf("Error parsing feed %s: %v", feed.URL, err)
		return
	}

	// Update Feed Image if available and not set
	if feed.ImageURL == "" && parsedFeed.Image != nil {
		f.db.UpdateFeedImage(feed.ID, parsedFeed.Image.URL)
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

		content := item.Content
		if content == "" {
			content = item.Description
		}

		summary := item.Description
		if summary == "" && len(content) > 200 {
			summary = content[:200] + "..."
		}

		imageURL := ""
		if item.Image != nil {
			imageURL = item.Image.URL
		} else if len(item.Enclosures) > 0 && item.Enclosures[0].Type == "image/jpeg" { // Simple check
			imageURL = item.Enclosures[0].URL
		}

		// Fallback: Try to find image in content
		if imageURL == "" {
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

		article := &models.Article{
			FeedID:          feed.ID,
			Title:           item.Title,
			URL:             item.Link,
			Content:         content,
			Summary:         summary,
			ImageURL:        imageURL,
			PublishedAt:     published,
			TranslatedTitle: translatedTitle,
		}
		articlesToSave = append(articlesToSave, article)
	}

	if len(articlesToSave) > 0 {
		if err := f.db.SaveArticles(articlesToSave); err != nil {
			log.Printf("Error saving articles for feed %s: %v", feed.Title, err)
		}
	}
	log.Printf("Updated feed: %s", feed.Title)
}

func (f *Fetcher) AddSubscription(url string, category string) error {
	parsedFeed, err := f.fp.ParseURL(url)
	if err != nil {
		return err
	}

	feed := &models.Feed{
		Title:       parsedFeed.Title,
		URL:         url,
		Description: parsedFeed.Description,
		Category:    category,
	}

	if parsedFeed.Image != nil {
		feed.ImageURL = parsedFeed.Image.URL
	}

	return f.db.AddFeed(feed)
}

func (f *Fetcher) ImportSubscription(title, url, category string) error {
	feed := &models.Feed{
		Title:    title,
		URL:      url,
		Category: category,
	}
	return f.db.AddFeed(feed)
}
