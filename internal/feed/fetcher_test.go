package feed

import (
	"MrRSS/internal/database"
	"MrRSS/internal/translation"
	"context"
	"testing"

	"github.com/mmcdole/gofeed"
)

type MockParser struct {
	Feed *gofeed.Feed
	Err  error
}

func (m *MockParser) ParseURL(url string) (*gofeed.Feed, error) {
	return m.Feed, m.Err
}

func (m *MockParser) ParseURLWithContext(url string, ctx context.Context) (*gofeed.Feed, error) {
	return m.Feed, m.Err
}

func TestAddSubscription(t *testing.T) {
	db, err := database.NewDB(":memory:")
	if err != nil {
		t.Fatalf("Failed to create db: %v", err)
	}
	if err := db.Init(); err != nil {
		t.Fatalf("Failed to init db: %v", err)
	}

	mockFeed := &gofeed.Feed{
		Title:       "Test Feed",
		Description: "Test Description",
		Items:       []*gofeed.Item{},
	}

	fetcher := NewFetcher(db, translation.NewMockTranslator())
	fetcher.fp = &MockParser{Feed: mockFeed}

	err = fetcher.AddSubscription("http://test.com/rss", "Test Category", "")
	if err != nil {
		t.Fatalf("AddSubscription failed: %v", err)
	}

	feeds, err := db.GetFeeds()
	if err != nil {
		t.Fatalf("GetFeeds failed: %v", err)
	}

	if len(feeds) != 1 {
		t.Errorf("Expected 1 feed, got %d", len(feeds))
	}
	if feeds[0].Title != "Test Feed" {
		t.Errorf("Expected title 'Test Feed', got '%s'", feeds[0].Title)
	}
}

func TestFetchFeed(t *testing.T) {
	db, err := database.NewDB(":memory:")
	if err != nil {
		t.Fatalf("Failed to create db: %v", err)
	}
	if err := db.Init(); err != nil {
		t.Fatalf("Failed to init db: %v", err)
	}

	// Add a feed first
	fetcher := NewFetcher(db, translation.NewMockTranslator())

	// Mock the parser for AddSubscription
	mockFeed := &gofeed.Feed{
		Title:       "Test Feed",
		Description: "Test Description",
		Items: []*gofeed.Item{
			{
				Title:       "Test Article",
				Link:        "http://test.com/article",
				Description: "Article Description",
				Content:     "Article Content",
			},
		},
	}
	fetcher.fp = &MockParser{Feed: mockFeed}

	err = fetcher.AddSubscription("http://test.com/rss", "Test Category", "")
	if err != nil {
		t.Fatalf("AddSubscription failed: %v", err)
	}

	feeds, _ := db.GetFeeds()

	// Fetch the feed
	fetcher.FetchFeed(context.Background(), feeds[0])

	articles, err := db.GetArticles("", 0, "", false, 10, 0)
	if err != nil {
		t.Fatalf("GetArticles failed: %v", err)
	}

	if len(articles) != 1 {
		t.Errorf("Expected 1 article, got %d", len(articles))
	}
	if articles[0].Title != "Test Article" {
		t.Errorf("Expected article title 'Test Article', got '%s'", articles[0].Title)
	}
}

func TestFetchFeedWithMissingTitle(t *testing.T) {
	db, err := database.NewDB(":memory:")
	if err != nil {
		t.Fatalf("Failed to create db: %v", err)
	}
	if err := db.Init(); err != nil {
		t.Fatalf("Failed to init db: %v", err)
	}

	// Add a feed first
	fetcher := NewFetcher(db, translation.NewMockTranslator())

	// Mock the parser with an item that has no title but has content
	mockFeed := &gofeed.Feed{
		Title:       "Test Feed",
		Description: "Test Description",
		Items: []*gofeed.Item{
			{
				Title:       "", // Missing title
				Link:        "http://test.com/article",
				Description: "",
				Content:     "This is a short content.",
			},
		},
	}
	fetcher.fp = &MockParser{Feed: mockFeed}

	err = fetcher.AddSubscription("http://test.com/rss", "Test Category", "")
	if err != nil {
		t.Fatalf("AddSubscription failed: %v", err)
	}

	feeds, _ := db.GetFeeds()

	// Fetch the feed
	fetcher.FetchFeed(context.Background(), feeds[0])

	articles, err := db.GetArticles("", 0, "", false, 10, 0)
	if err != nil {
		t.Fatalf("GetArticles failed: %v", err)
	}

	if len(articles) != 1 {
		t.Errorf("Expected 1 article, got %d", len(articles))
	}
	expectedTitle := "This is a short content."
	if articles[0].Title != expectedTitle {
		t.Errorf("Expected article title '%s', got '%s'", expectedTitle, articles[0].Title)
	}
}

func TestFetchFeedWithMissingTitleLongContent(t *testing.T) {
	db, err := database.NewDB(":memory:")
	if err != nil {
		t.Fatalf("Failed to create db: %v", err)
	}
	if err := db.Init(); err != nil {
		t.Fatalf("Failed to init db: %v", err)
	}

	// Add a feed first
	fetcher := NewFetcher(db, translation.NewMockTranslator())

	// Mock the parser with an item that has no title but has long content
	longContent := "This is a very long article content that should be truncated to generate a title from the beginning of the content when the title is missing from the RSS feed item."
	mockFeed := &gofeed.Feed{
		Title:       "Test Feed",
		Description: "Test Description",
		Items: []*gofeed.Item{
			{
				Title:       "", // Missing title
				Link:        "http://test.com/article",
				Description: "",
				Content:     longContent,
			},
		},
	}
	fetcher.fp = &MockParser{Feed: mockFeed}

	err = fetcher.AddSubscription("http://test.com/rss", "Test Category", "")
	if err != nil {
		t.Fatalf("AddSubscription failed: %v", err)
	}

	feeds, _ := db.GetFeeds()

	// Fetch the feed
	fetcher.FetchFeed(context.Background(), feeds[0])

	articles, err := db.GetArticles("", 0, "", false, 10, 0)
	if err != nil {
		t.Fatalf("GetArticles failed: %v", err)
	}

	if len(articles) != 1 {
		t.Errorf("Expected 1 article, got %d", len(articles))
	}
	// Should be truncated to 100 chars + "..."
	expectedTitle := longContent[:100] + "..."
	if articles[0].Title != expectedTitle {
		t.Errorf("Expected article title '%s', got '%s'", expectedTitle, articles[0].Title)
	}
}
