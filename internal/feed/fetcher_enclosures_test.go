package feed

import (
	"MavenRSS/internal/store/sqlite"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

)

func serveTestFeedWithEnclosures(t *testing.T, feedXML string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/xml; charset=utf-8")
		_, _ = w.Write([]byte(feedXML))
	}))
}

func TestFetchFeedWithAudioEnclosure(t *testing.T) {
	db, err := sqlite.NewDB(":memory:")
	if err != nil {
		t.Fatalf("Failed to create db: %v", err)
	}
	if err := db.Init(); err != nil {
		t.Fatalf("Failed to init db: %v", err)
	}

	fetcher := NewFetcher(db)

	srv := serveTestFeedWithEnclosures(t, `<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0">
  <channel>
    <title>Test Podcast</title>
    <description>Test Podcast Description</description>
    <link>http://example.com/</link>
    <item>
      <title>Podcast Episode 1</title>
      <link>http://example.com/episode1</link>
      <description>Episode Description</description>
      <content:encoded xmlns:content="http://purl.org/rss/1.0/modules/content/">Episode Content</content:encoded>
      <enclosure url="https://test.com/audio/episode1.mp3" type="audio/mpeg" length="12345678" />
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
	if articles[0].Title != "Podcast Episode 1" {
		t.Errorf("Expected article title 'Podcast Episode 1', got '%s'", articles[0].Title)
	}
	expectedAudioURL := "https://test.com/audio/episode1.mp3"
	if articles[0].AudioURL != expectedAudioURL {
		t.Errorf("Expected audio URL '%s', got '%s'", expectedAudioURL, articles[0].AudioURL)
	}
}

func TestFetchFeedWithImageEnclosure(t *testing.T) {
	db, err := sqlite.NewDB(":memory:")
	if err != nil {
		t.Fatalf("Failed to create db: %v", err)
	}
	if err := db.Init(); err != nil {
		t.Fatalf("Failed to init db: %v", err)
	}

	fetcher := NewFetcher(db)

	srv := serveTestFeedWithEnclosures(t, `<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0">
  <channel>
    <title>Test Feed</title>
    <description>Test Description</description>
    <link>http://example.com/</link>
    <item>
      <title>Article with PNG</title>
      <link>http://example.com/article1</link>
      <description>Article Description</description>
      <content:encoded xmlns:content="http://purl.org/rss/1.0/modules/content/">Article Content</content:encoded>
      <pubDate>` + time.Now().Add(time.Hour).Format(time.RFC1123Z) + `</pubDate>
      <enclosure url="https://test.com/images/image1.png" type="image/png" length="12345" />
    </item>
    <item>
      <title>Article with JPEG</title>
      <link>http://example.com/article2</link>
      <description>Article Description</description>
      <content:encoded xmlns:content="http://purl.org/rss/1.0/modules/content/">Article Content</content:encoded>
      <pubDate>` + time.Now().Format(time.RFC1123Z) + `</pubDate>
      <enclosure url="https://test.com/images/image2.jpg" type="image/jpeg" length="23456" />
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

	if len(articles) != 2 {
		t.Errorf("Expected 2 articles, got %d", len(articles))
	}

	if articles[0].ImageURL != "https://test.com/images/image1.png" {
		t.Errorf("Expected PNG image URL, got '%s'", articles[0].ImageURL)
	}
	if articles[1].ImageURL != "https://test.com/images/image2.jpg" {
		t.Errorf("Expected JPEG image URL, got '%s'", articles[1].ImageURL)
	}
}

func TestFetchFeedWithMultipleEnclosures(t *testing.T) {
	db, err := sqlite.NewDB(":memory:")
	if err != nil {
		t.Fatalf("Failed to create db: %v", err)
	}
	if err := db.Init(); err != nil {
		t.Fatalf("Failed to init db: %v", err)
	}

	fetcher := NewFetcher(db)

	srv := serveTestFeedWithEnclosures(t, `<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0">
  <channel>
    <title>Test Podcast</title>
    <description>Test Podcast Description</description>
    <link>http://example.com/</link>
    <item>
      <title>Podcast Episode with Cover</title>
      <link>http://example.com/episode1</link>
      <description>Episode Description</description>
      <content:encoded xmlns:content="http://purl.org/rss/1.0/modules/content/">Episode Content</content:encoded>
      <enclosure url="https://test.com/images/cover.jpg" type="image/jpeg" length="12345" />
      <enclosure url="https://test.com/audio/episode1.mp3" type="audio/mpeg" length="98765432" />
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

	expectedImageURL := "https://test.com/images/cover.jpg"
	if articles[0].ImageURL != expectedImageURL {
		t.Errorf("Expected image URL '%s', got '%s'", expectedImageURL, articles[0].ImageURL)
	}

	expectedAudioURL := "https://test.com/audio/episode1.mp3"
	if articles[0].AudioURL != expectedAudioURL {
		t.Errorf("Expected audio URL '%s', got '%s'", expectedAudioURL, articles[0].AudioURL)
	}
}
