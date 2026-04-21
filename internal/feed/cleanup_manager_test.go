package feed

import (
	"os"
	"testing"
	"time"

	"MavenRSS/internal/store/sqlite"
)

func setupCleanupManagerTestDB(t *testing.T) *sqlite.DB {
	t.Helper()

	dbFile := "test_cleanup_manager.db"
	_ = os.Remove(dbFile)
	t.Cleanup(func() { _ = os.Remove(dbFile) })

	db, err := sqlite.NewDB(dbFile)
	if err != nil {
		t.Fatalf("create db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	if err := db.Init(); err != nil {
		t.Fatalf("init db: %v", err)
	}

	return db
}

func TestCleanupManagerLayeredCleanupIsScopedToUser(t *testing.T) {
	db := setupCleanupManagerTestDB(t)
	const userOneID int64 = 11
	const userTwoID int64 = 12

	if _, err := db.Exec(`INSERT INTO users (id, username, email, password_hash, role, status) VALUES (?, 'user1', 'user1@example.com', 'hash', 'user', 'active')`, userOneID); err != nil {
		t.Fatalf("insert user1: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO users (id, username, email, password_hash, role, status) VALUES (?, 'user2', 'user2@example.com', 'hash', 'user', 'active')`, userTwoID); err != nil {
		t.Fatalf("insert user2: %v", err)
	}

	if _, err := db.Exec(`INSERT INTO user_quota (user_id, max_feeds, max_articles, max_ai_tokens, max_ai_concurrency, max_feed_fetch_concurrency, max_db_query_concurrency, max_media_cache_concurrency, max_rss_discovery_concurrency, max_rss_path_check_concurrency, max_translation_concurrency, max_storage_mb) VALUES (?, 100, 100000, 100000, 5, 5, 5, 5, 5, 5, 5, 500)`, userOneID); err != nil {
		t.Fatalf("insert quota1: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO user_quota (user_id, max_feeds, max_articles, max_ai_tokens, max_ai_concurrency, max_feed_fetch_concurrency, max_db_query_concurrency, max_media_cache_concurrency, max_rss_discovery_concurrency, max_rss_path_check_concurrency, max_translation_concurrency, max_storage_mb) VALUES (?, 100, 100000, 100000, 5, 5, 5, 5, 5, 5, 5, 500)`, userTwoID); err != nil {
		t.Fatalf("insert quota2: %v", err)
	}

	result, err := db.Exec(`INSERT INTO feeds (user_id, title, url, category, is_image_mode, hide_from_timeline) VALUES (?, 'Feed 1', 'https://example.com/feed1', 'news', 0, 0)`, userOneID)
	if err != nil {
		t.Fatalf("insert feed1: %v", err)
	}
	feedOneID, _ := result.LastInsertId()

	result, err = db.Exec(`INSERT INTO feeds (user_id, title, url, category, is_image_mode, hide_from_timeline) VALUES (?, 'Feed 2', 'https://example.com/feed2', 'news', 0, 0)`, userTwoID)
	if err != nil {
		t.Fatalf("insert feed2: %v", err)
	}
	feedTwoID, _ := result.LastInsertId()

	oldTime := time.Now().AddDate(0, 0, -45)
	if _, err := db.Exec(`INSERT INTO articles (user_id, feed_id, title, url, published_at, is_read, is_favorite, is_read_later, unique_id) VALUES (?, ?, 'User1 Old', 'https://example.com/u1-old', ?, 1, 0, 0, 'cleanup-manager-u1')`, userOneID, feedOneID, oldTime); err != nil {
		t.Fatalf("insert user1 article: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO articles (user_id, feed_id, title, url, published_at, is_read, is_favorite, is_read_later, unique_id) VALUES (?, ?, 'User2 Old', 'https://example.com/u2-old', ?, 1, 0, 0, 'cleanup-manager-u2')`, userTwoID, feedTwoID, oldTime); err != nil {
		t.Fatalf("insert user2 article: %v", err)
	}

	cm := NewCleanupManager(&Fetcher{db: db})

	removed := cm.layeredCleanup(userOneID, -1)
	if removed == 0 {
		t.Fatalf("expected scoped cleanup to remove user1 data")
	}

	var userOneCount int
	if err := db.QueryRow(`SELECT COUNT(*) FROM articles WHERE user_id = ?`, userOneID).Scan(&userOneCount); err != nil {
		t.Fatalf("count user1 articles: %v", err)
	}
	if userOneCount != 0 {
		t.Fatalf("expected user1 articles to be cleaned, got %d", userOneCount)
	}

	var userTwoCount int
	if err := db.QueryRow(`SELECT COUNT(*) FROM articles WHERE user_id = ?`, userTwoID).Scan(&userTwoCount); err != nil {
		t.Fatalf("count user2 articles: %v", err)
	}
	if userTwoCount != 1 {
		t.Fatalf("expected user2 articles to remain untouched, got %d", userTwoCount)
	}

	cm.Stop()
}
