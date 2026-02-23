package database_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	dbpkg "MavenRSS/internal/database"
	"MavenRSS/internal/models"
)

func setupDBWithFeed(t *testing.T) *dbpkg.DB {
	t.Helper()
	db := setupTestDB(t)

	// Insert a feed to satisfy foreign key joins
	res, err := db.Exec(`INSERT INTO feeds (title, url, category, is_image_mode, hide_from_timeline) VALUES (?, ?, ?, ?, ?)`, "Test Feed", "https://example.com/feed", "news", 0, 0)
	if err != nil {
		t.Fatalf("insert feed error: %v", err)
	}
	_, _ = res.LastInsertId()
	return db
}

func TestSaveAndGetArticle(t *testing.T) {
	db := setupDBWithFeed(t)

	// Get feed id
	var feedID int64
	row := db.QueryRow(`SELECT id FROM feeds WHERE url = ?`, "https://example.com/feed")
	if err := row.Scan(&feedID); err != nil {
		t.Fatalf("scan feed id: %v", err)
	}

	a := &models.Article{
		FeedID:      feedID,
		Title:       "Hello",
		URL:         "https://example.com/article/1",
		ImageURL:    "https://example.com/img.jpg",
		PublishedAt: time.Now(),
	}

	if err := db.SaveArticle(a); err != nil {
		t.Fatalf("SaveArticle error: %v", err)
	}

	// Retrieve by GetArticles
	list, err := db.GetArticles("all", 0, "", false, 10, 0)
	if err != nil {
		t.Fatalf("GetArticles error: %v", err)
	}
	if len(list) == 0 {
		t.Fatalf("expected at least one article, got 0")
	}

	// Get by ID
	got, err := db.GetArticleByID(list[0].ID)
	if err != nil {
		t.Fatalf("GetArticleByID error: %v", err)
	}
	if got.URL != a.URL || got.Title != a.Title {
		t.Fatalf("retrieved article mismatch: %+v vs %+v", got, a)
	}
}

func TestMarkReadAndReadLaterAndFavorites(t *testing.T) {
	db := setupDBWithFeed(t)

	// Get feed id
	var feedID int64
	_ = db.QueryRow(`SELECT id FROM feeds WHERE url = ?`, "https://example.com/feed").Scan(&feedID)

	// Insert article
	res, err := db.Exec(`INSERT INTO articles (feed_id, title, url, published_at, is_read, is_favorite, is_read_later) VALUES (?, ?, ?, ?, 0, 0, 0)`, feedID, "A", "u1", time.Now())
	if err != nil {
		t.Fatalf("insert article: %v", err)
	}
	id, _ := res.LastInsertId()

	// Mark read
	if err := db.MarkArticleRead(id, true); err != nil {
		t.Fatalf("MarkArticleRead error: %v", err)
	}

	// Should be marked read and not read later
	var isRead, isReadLater int
	_ = db.QueryRow("SELECT is_read, is_read_later FROM articles WHERE id = ?", id).Scan(&isRead, &isReadLater)
	if isRead != 1 || isReadLater != 0 {
		t.Fatalf("unexpected read/readlater state: %d/%d", isRead, isReadLater)
	}

	// Toggle favorite
	if err := db.ToggleFavorite(id); err != nil {
		t.Fatalf("ToggleFavorite error: %v", err)
	}
	var isFav int
	_ = db.QueryRow("SELECT is_favorite FROM articles WHERE id = ?", id).Scan(&isFav)
	if isFav != 1 {
		t.Fatalf("expected favorite set, got %d", isFav)
	}

	// Toggle read later (will unset since currently 0 -> toggled to 0? ensure it works)
	if err := db.ToggleReadLater(id); err != nil {
		t.Fatalf("ToggleReadLater error: %v", err)
	}
}

func TestUnreadCountsAndMarkAll(t *testing.T) {
	db := setupDBWithFeed(t)

	// Insert feed id
	var feedID int64
	_ = db.QueryRow(`SELECT id FROM feeds WHERE url = ?`, "https://example.com/feed").Scan(&feedID)

	// Insert multiple articles
	for i := 0; i < 5; i++ {
		_, err := db.Exec(`INSERT INTO articles (feed_id, title, url, published_at, is_read, is_hidden) VALUES (?, ?, ?, ?, 0, 0)`, feedID, fmt.Sprintf("t%d", i), fmt.Sprintf("u%d", i), time.Now())
		if err != nil {
			t.Fatalf("insert article: %v", err)
		}
	}

	total, err := db.GetTotalUnreadCount()
	if err != nil {
		t.Fatalf("GetTotalUnreadCount error: %v", err)
	}
	if total < 5 {
		t.Fatalf("expected at least 5 unread, got %d", total)
	}

	byFeed, err := db.GetUnreadCountByFeed(feedID)
	if err != nil {
		t.Fatalf("GetUnreadCountByFeed error: %v", err)
	}
	if byFeed < 1 {
		t.Fatalf("expected unread for feed, got %d", byFeed)
	}

	counts, err := db.GetUnreadCountsForAllFeeds()
	if err != nil {
		t.Fatalf("GetUnreadCountsForAllFeeds error: %v", err)
	}
	if counts[feedID] < 1 {
		t.Fatalf("expected counts map to include feed %d", feedID)
	}

	// Mark all as read for feed
	if err := db.MarkAllAsReadForFeed(feedID); err != nil {
		t.Fatalf("MarkAllAsReadForFeed error: %v", err)
	}
	totalAfter, _ := db.GetTotalUnreadCount()
	if totalAfter != 0 {
		t.Fatalf("expected 0 unread after marking all read, got %d", totalAfter)
	}
}

func TestCleanupOldAndUnimportantAndDBSize(t *testing.T) {
	db := setupDBWithFeed(t)

	// Insert old article (older than default 30 days)
	oldTime := time.Now().AddDate(0, 0, -100)
	var feedID int64
	_ = db.QueryRow(`SELECT id FROM feeds WHERE url = ?`, "https://example.com/feed").Scan(&feedID)

	_, err := db.Exec(`INSERT INTO articles (feed_id, title, url, published_at, is_favorite, is_read_later) VALUES (?, ?, ?, ?, 0, 0)`, feedID, "old", "oldurl", oldTime)
	if err != nil {
		t.Fatalf("insert old article: %v", err)
	}

	// Insert unimportant article (unread, not favorite/readlater)
	_, err = db.Exec(`INSERT INTO articles (feed_id, title, url, published_at, is_read, is_favorite, is_read_later) VALUES (?, ?, ?, ?, 0, 0, 0)`, feedID, "tmp", "u2", time.Now())
	if err != nil {
		t.Fatalf("insert tmp article: %v", err)
	}

	// Cleanup old articles
	deleted, err := db.CleanupOldArticles()
	if err != nil {
		t.Fatalf("CleanupOldArticles error: %v", err)
	}
	if deleted < 1 {
		t.Fatalf("expected at least 1 deleted old article, got %d", deleted)
	}

	// Cleanup unimportant
	del2, err := db.CleanupUnimportantArticles()
	if err != nil {
		t.Fatalf("CleanupUnimportantArticles error: %v", err)
	}
	if del2 < 0 {
		t.Fatalf("unexpected deleted count: %d", del2)
	}

	// DB size
	sz, err := db.GetDatabaseSizeMB()
	if err != nil {
		t.Fatalf("GetDatabaseSizeMB error: %v", err)
	}
	if sz < 0 {
		t.Fatalf("unexpected db size: %f", sz)
	}
}

func TestSaveArticlesBatchContextCancel(t *testing.T) {
	db := setupDBWithFeed(t)

	// Prepare articles
	// determine feed id
	var feedID2 int64
	_ = db.QueryRow(`SELECT id FROM feeds WHERE url = ?`, "https://example.com/feed").Scan(&feedID2)

	articles := []*models.Article{}
	for i := 0; i < 10; i++ {
		articles = append(articles, &models.Article{FeedID: feedID2, Title: "b", URL: "u" + string(rune(i))})
	}

	// Cancel context immediately
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if err := db.SaveArticles(ctx, articles); err == nil {
		t.Fatalf("expected error due to canceled context")
	}
}

func TestArticleDeduplicationByUniqueID(t *testing.T) {
	db := setupDBWithFeed(t)

	// Get feed id
	var feedID int64
	row := db.QueryRow(`SELECT id FROM feeds WHERE url = ?`, "https://example.com/feed")
	if err := row.Scan(&feedID); err != nil {
		t.Fatalf("scan feed id: %v", err)
	}

	publishedAt := time.Now()

	// Save same article multiple times with different URLs (should be deduplicated by unique_id)
	article1 := &models.Article{
		FeedID:      feedID,
		Title:       "Test Article",
		URL:         "https://example.com/article/1",
		PublishedAt: publishedAt,
	}

	article2 := &models.Article{
		FeedID:      feedID,
		Title:       "Test Article",                                  // Same title
		URL:         "https://example.com/article/1?utm_source=test", // Different URL
		PublishedAt: publishedAt,                                     // Same time
	}

	// Save first article
	if err := db.SaveArticle(article1); err != nil {
		t.Fatalf("SaveArticle error: %v", err)
	}

	// Try to save the same article again (should be ignored due to unique_id)
	if err := db.SaveArticle(article2); err != nil {
		t.Fatalf("SaveArticle error: %v", err)
	}

	// Verify only one article exists
	articles, err := db.GetArticles("all", feedID, "", false, 10, 0)
	if err != nil {
		t.Fatalf("GetArticles error: %v", err)
	}

	if len(articles) != 1 {
		t.Fatalf("expected 1 article after deduplication, got %d", len(articles))
	}

	// Verify the article has the correct unique_id
	if articles[0].Title != "Test Article" {
		t.Fatalf("expected title 'Test Article', got '%s'", articles[0].Title)
	}
}

func TestArticleDifferentTitlesNotDeduplicated(t *testing.T) {
	db := setupDBWithFeed(t)

	// Get feed id
	var feedID int64
	row := db.QueryRow(`SELECT id FROM feeds WHERE url = ?`, "https://example.com/feed")
	if err := row.Scan(&feedID); err != nil {
		t.Fatalf("scan feed id: %v", err)
	}

	publishedAt := time.Now()

	// Save different articles with same feed and time (should NOT be deduplicated)
	article1 := &models.Article{
		FeedID:      feedID,
		Title:       "Article One",
		URL:         "https://example.com/article/1",
		PublishedAt: publishedAt,
	}

	article2 := &models.Article{
		FeedID:      feedID,
		Title:       "Article Two", // Different title
		URL:         "https://example.com/article/2",
		PublishedAt: publishedAt, // Same time
	}

	// Save both articles
	if err := db.SaveArticle(article1); err != nil {
		t.Fatalf("SaveArticle error: %v", err)
	}

	if err := db.SaveArticle(article2); err != nil {
		t.Fatalf("SaveArticle error: %v", err)
	}

	// Verify both articles exist
	articles, err := db.GetArticles("all", feedID, "", false, 10, 0)
	if err != nil {
		t.Fatalf("GetArticles error: %v", err)
	}

	if len(articles) != 2 {
		t.Fatalf("expected 2 articles with different titles, got %d", len(articles))
	}
}
