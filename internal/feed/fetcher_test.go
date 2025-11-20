package feed

import (
	"MrRSS/internal/database"
	"MrRSS/internal/translation"
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

func TestAddSubscription(t *testing.T) {
	db, err := database.NewDB(":memory:")
	if err != nil {
		t.Fatalf("Failed to create db: %v", err)
	}

	mockFeed := &gofeed.Feed{
		Title:       "Test Feed",
		Description: "Test Description",
		Items:       []*gofeed.Item{},
	}

	fetcher := NewFetcher(db, translation.NewMockTranslator())
	fetcher.fp = &MockParser{Feed: mockFeed}

	err = fetcher.AddSubscription("http://test.com/rss", "Test Category")
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

	err = fetcher.AddSubscription("http://test.com/rss", "Test Category")
	if err != nil {
		t.Fatalf("AddSubscription failed: %v", err)
	}

	feeds, _ := db.GetFeeds()

	// Fetch the feed
	fetcher.FetchFeed(feeds[0])

	articles, err := db.GetArticles("", 0, "", 10, 0)
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
