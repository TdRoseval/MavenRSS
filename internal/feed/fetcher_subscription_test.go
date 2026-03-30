package feed

import (
	"MavenRSS/internal/store/sqlite"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

)

func serveTestFeed(t *testing.T, feedXML string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/xml; charset=utf-8")
		_, _ = w.Write([]byte(feedXML))
	}))
}

func TestAddSubscription(t *testing.T) {
	db, err := sqlite.NewDB(":memory:")
	if err != nil {
		t.Fatalf("Failed to create db: %v", err)
	}
	if err := db.Init(); err != nil {
		t.Fatalf("Failed to init db: %v", err)
	}

	fetcher := NewFetcher(db)

	srv := serveTestFeed(t, `<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0">
  <channel>
    <title>Test Feed</title>
    <description>Test Description</description>
    <link>http://example.com/</link>
  </channel>
</rss>`)
	defer srv.Close()

	_, err = fetcher.AddSubscription(srv.URL, "Test Category", "")
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
	db, err := sqlite.NewDB(":memory:")
	if err != nil {
		t.Fatalf("Failed to create db: %v", err)
	}
	if err := db.Init(); err != nil {
		t.Fatalf("Failed to init db: %v", err)
	}

	fetcher := NewFetcher(db)

	srv := serveTestFeed(t, `<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0">
  <channel>
    <title>Test Feed</title>
    <description>Test Description</description>
    <link>http://example.com/</link>
    <item>
      <title>Test Article</title>
      <link>http://example.com/article</link>
      <description>Article Description</description>
      <content:encoded xmlns:content="http://purl.org/rss/1.0/modules/content/">Article Content</content:encoded>
    </item>
  </channel>
</rss>`)
	defer srv.Close()

	_, err = fetcher.AddSubscription(srv.URL, "Test Category", "")
	if err != nil {
		t.Fatalf("AddSubscription failed: %v", err)
	}

	feeds, _ := db.GetFeeds()

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
	db, err := sqlite.NewDB(":memory:")
	if err != nil {
		t.Fatalf("Failed to create db: %v", err)
	}
	if err := db.Init(); err != nil {
		t.Fatalf("Failed to init db: %v", err)
	}

	fetcher := NewFetcher(db)

	srv := serveTestFeed(t, `<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0">
  <channel>
    <title>Test Feed</title>
    <description>Test Description</description>
    <link>http://example.com/</link>
    <item>
      <title></title>
      <link>http://example.com/article</link>
      <description></description>
      <content:encoded xmlns:content="http://purl.org/rss/1.0/modules/content/">This is a short content.</content:encoded>
    </item>
  </channel>
</rss>`)
	defer srv.Close()

	_, err = fetcher.AddSubscription(srv.URL, "Test Category", "")
	if err != nil {
		t.Fatalf("AddSubscription failed: %v", err)
	}

	feeds, _ := db.GetFeeds()

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
	db, err := sqlite.NewDB(":memory:")
	if err != nil {
		t.Fatalf("Failed to create db: %v", err)
	}
	if err := db.Init(); err != nil {
		t.Fatalf("Failed to init db: %v", err)
	}

	fetcher := NewFetcher(db)

	longContent := "This is a very long article content that should be truncated to generate a title from the beginning of the content when the title is missing from the RSS feed item."
	srv := serveTestFeed(t, `<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0">
  <channel>
    <title>Test Feed</title>
    <description>Test Description</description>
    <link>http://example.com/</link>
    <item>
      <title></title>
      <link>http://example.com/article</link>
      <description></description>
      <content:encoded xmlns:content="http://purl.org/rss/1.0/modules/content/">` + longContent + `</content:encoded>
    </item>
  </channel>
</rss>`)
	defer srv.Close()

	_, err = fetcher.AddSubscription(srv.URL, "Test Category", "")
	if err != nil {
		t.Fatalf("AddSubscription failed: %v", err)
	}

	feeds, _ := db.GetFeeds()

	fetcher.FetchFeed(context.Background(), feeds[0])

	articles, err := db.GetArticles("", 0, "", false, 10, 0)
	if err != nil {
		t.Fatalf("GetArticles failed: %v", err)
	}

	if len(articles) != 1 {
		t.Errorf("Expected 1 article, got %d", len(articles))
	}
	expectedTitle := longContent[:100] + "..."
	if articles[0].Title != expectedTitle {
		t.Errorf("Expected article title '%s', got '%s'", expectedTitle, articles[0].Title)
	}
}
