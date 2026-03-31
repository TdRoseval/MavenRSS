package sqlite

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"MavenRSS/internal/models"
)

func newMigrationTestDB(t *testing.T, dbFile string) *DB {
	t.Helper()

	db, err := NewDB(dbFile)
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	return db
}

func TestDatabaseInitialization(t *testing.T) {
	dbFile := "test_init.db"
	defer os.Remove(dbFile)

	db, err := NewDB(dbFile)
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer db.Close()

	err = db.Init()
	if err != nil {
		t.Fatalf("Failed to initialize database: %v", err)
	}

	var tableCount int
	err = db.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name IN ('feeds', 'articles', 'settings')").Scan(&tableCount)
	if err != nil {
		t.Fatalf("Failed to query tables: %v", err)
	}
	if tableCount != 3 {
		t.Errorf("Expected 3 tables, got %d", tableCount)
	}

	var indexCount int
	err = db.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='index' AND name LIKE 'idx_%'").Scan(&indexCount)
	if err != nil {
		t.Fatalf("Failed to query indexes: %v", err)
	}
	if indexCount < 8 {
		t.Errorf("Expected at least 8 indexes, got %d", indexCount)
	}
}

func TestDatabasePerformanceWithIndexes(t *testing.T) {
	dbFile := "test_perf.db"
	defer os.Remove(dbFile)

	db, err := NewDB(dbFile)
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer db.Close()

	err = db.Init()
	if err != nil {
		t.Fatalf("Failed to initialize database: %v", err)
	}

	feed := &models.Feed{
		Title:       "Test Feed",
		URL:         "https://example.com/feed",
		Description: "Test Description",
		Category:    "test",
	}
	_, err = db.AddFeed(feed)
	if err != nil {
		t.Fatalf("Failed to add feed: %v", err)
	}

	feeds, err := db.GetFeeds()
	if err != nil || len(feeds) == 0 {
		t.Fatalf("Failed to get feeds: %v", err)
	}
	feedID := feeds[0].ID

	ctx := context.Background()
	numArticles := 1000
	articles := make([]*models.Article, numArticles)
	for i := 0; i < numArticles; i++ {
		articles[i] = &models.Article{
			FeedID:      feedID,
			Title:       fmt.Sprintf("Article %d", i),
			URL:         fmt.Sprintf("https://example.com/article-%d", i),
			PublishedAt: time.Now().Add(-time.Duration(i) * time.Minute),
			IsRead:      i%2 == 0,
			IsFavorite:  i%10 == 0,
		}
	}

	startInsert := time.Now()
	err = db.SaveArticles(ctx, articles)
	if err != nil {
		t.Fatalf("Failed to save articles: %v", err)
	}
	insertDuration := time.Since(startInsert)
	t.Logf("Inserted %d articles in %v", numArticles, insertDuration)

	startQuery := time.Now()
	results, err := db.GetArticles("unread", feedID, "", false, 50, 0)
	if err != nil {
		t.Fatalf("Failed to get articles: %v", err)
	}
	queryDuration := time.Since(startQuery)
	t.Logf("Queried articles in %v, got %d results", queryDuration, len(results))

	if queryDuration > 50*time.Millisecond {
		t.Logf("Warning: Query took longer than expected: %v", queryDuration)
	}

	startCategoryQuery := time.Now()
	results, err = db.GetArticles("", 0, "test", false, 50, 0)
	if err != nil {
		t.Fatalf("Failed to get articles by category: %v", err)
	}
	categoryQueryDuration := time.Since(startCategoryQuery)
	t.Logf("Queried articles by category in %v, got %d results", categoryQueryDuration, len(results))

	startFavQuery := time.Now()
	results, err = db.GetArticles("favorites", 0, "", false, 50, 0)
	if err != nil {
		t.Fatalf("Failed to get favorite articles: %v", err)
	}
	favQueryDuration := time.Since(startFavQuery)
	t.Logf("Queried favorite articles in %v, got %d results", favQueryDuration, len(results))
}

func TestMigrationIdempotency(t *testing.T) {
	dbFile := "test_migration.db"
	defer os.Remove(dbFile)

	db, err := NewDB(dbFile)
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer db.Close()

	for i := 0; i < 3; i++ {
		err = db.Init()
		if err != nil {
			t.Fatalf("Failed to initialize database on iteration %d: %v", i, err)
		}
	}

	var tableCount int
	err = db.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name IN ('feeds', 'articles', 'settings')").Scan(&tableCount)
	if err != nil {
		t.Fatalf("Failed to query tables: %v", err)
	}
	if tableCount != 3 {
		t.Errorf("Expected 3 tables after multiple inits, got %d", tableCount)
	}
}

func TestRunMigrationsCreatesMissingChatTables(t *testing.T) {
	dbFile := "test_chat_schema_migration.db"
	defer os.Remove(dbFile)

	db := newMigrationTestDB(t, dbFile)
	defer db.Close()

	statements := []string{
		`CREATE TABLE users (id INTEGER PRIMARY KEY AUTOINCREMENT, username TEXT)`,
		`CREATE TABLE feeds (id INTEGER PRIMARY KEY AUTOINCREMENT, user_id INTEGER NOT NULL DEFAULT 0, title TEXT, url TEXT)`,
		`CREATE TABLE articles (id INTEGER PRIMARY KEY AUTOINCREMENT, user_id INTEGER NOT NULL DEFAULT 0, feed_id INTEGER, title TEXT, url TEXT, published_at DATETIME)`,
	}
	for _, stmt := range statements {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatalf("Failed to prepare schema with %q: %v", stmt, err)
		}
	}

	if err := runMigrations(db.DB); err != nil {
		t.Fatalf("runMigrations failed: %v", err)
	}

	for _, tableName := range []string{"chat_sessions", "chat_messages"} {
		var count int
		if err := db.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name=?", tableName).Scan(&count); err != nil {
			t.Fatalf("Failed to query table %s: %v", tableName, err)
		}
		if count != 1 {
			t.Fatalf("Expected table %s to exist", tableName)
		}
	}

	for _, indexName := range []string{"idx_chat_sessions_user_id", "idx_chat_sessions_article_id", "idx_chat_sessions_updated_at", "idx_chat_messages_session_id"} {
		var count int
		if err := db.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='index' AND name=?", indexName).Scan(&count); err != nil {
			t.Fatalf("Failed to query index %s: %v", indexName, err)
		}
		if count != 1 {
			t.Fatalf("Expected index %s to exist", indexName)
		}
	}
}

func TestRunMigrationsBackfillsChatSessionUserID(t *testing.T) {
	dbFile := "test_chat_user_id_backfill.db"
	defer os.Remove(dbFile)

	db := newMigrationTestDB(t, dbFile)
	defer db.Close()

	statements := []string{
		`CREATE TABLE users (id INTEGER PRIMARY KEY AUTOINCREMENT, username TEXT)`,
		`CREATE TABLE articles (id INTEGER PRIMARY KEY AUTOINCREMENT, user_id INTEGER NOT NULL DEFAULT 0, feed_id INTEGER, title TEXT, url TEXT, published_at DATETIME)`,
		`CREATE TABLE chat_sessions (id INTEGER PRIMARY KEY AUTOINCREMENT, article_id INTEGER NOT NULL, title TEXT NOT NULL, created_at DATETIME DEFAULT CURRENT_TIMESTAMP, updated_at DATETIME DEFAULT CURRENT_TIMESTAMP)`,
	}
	for _, stmt := range statements {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatalf("Failed to prepare schema with %q: %v", stmt, err)
		}
	}

	if _, err := db.Exec(`INSERT INTO users (id, username) VALUES (1, 'tester')`); err != nil {
		t.Fatalf("Failed to insert user: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO articles (id, user_id, title, url) VALUES (100, 1, 'article', 'https://example.com/article')`); err != nil {
		t.Fatalf("Failed to insert article: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO chat_sessions (id, article_id, title) VALUES (10, 100, 'session')`); err != nil {
		t.Fatalf("Failed to insert chat session: %v", err)
	}

	if err := runMigrations(db.DB); err != nil {
		t.Fatalf("runMigrations failed: %v", err)
	}

	var userID int
	if err := db.QueryRow(`SELECT user_id FROM chat_sessions WHERE id = 10`).Scan(&userID); err != nil {
		t.Fatalf("Failed to query migrated chat session: %v", err)
	}
	if userID != 1 {
		t.Fatalf("Expected chat_sessions.user_id to be backfilled to 1, got %d", userID)
	}
}

func TestRunMigrationsCreatesDailyRecommendationsOnlyOnce(t *testing.T) {
	dbFile := "test_daily_recommendations_migration.db"
	defer os.Remove(dbFile)

	db := newMigrationTestDB(t, dbFile)
	defer db.Close()

	statements := []string{
		`CREATE TABLE users (id INTEGER PRIMARY KEY AUTOINCREMENT, username TEXT)`,
		`CREATE TABLE clusters (id INTEGER PRIMARY KEY AUTOINCREMENT, user_id INTEGER NOT NULL, status TEXT NOT NULL DEFAULT 'pending_merge')`,
	}
	for _, stmt := range statements {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatalf("Failed to prepare schema with %q: %v", stmt, err)
		}
	}

	for i := 0; i < 2; i++ {
		if err := runMigrations(db.DB); err != nil {
			t.Fatalf("runMigrations failed on iteration %d: %v", i, err)
		}
	}

	var count int
	if err := db.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='daily_recommendations'").Scan(&count); err != nil {
		t.Fatalf("Failed to query daily_recommendations table: %v", err)
	}
	if count != 1 {
		t.Fatalf("Expected one daily_recommendations table, got %d", count)
	}

	for _, columnName := range []string{"recommendation_archive_date", "recommendation_score", "is_ai_recommended", "recommendation_profile_id"} {
		if !columnExists(db.DB, "clusters", columnName) {
			t.Fatalf("Expected clusters.%s to exist after migration", columnName)
		}
	}

	for _, indexName := range []string{"idx_clusters_user_ai_recommended", "idx_clusters_archive_date", "idx_daily_recommendations_user_date", "idx_daily_recommendations_cluster"} {
		var indexCount int
		if err := db.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='index' AND name=?", indexName).Scan(&indexCount); err != nil {
			t.Fatalf("Failed to query index %s: %v", indexName, err)
		}
		if indexCount != 1 {
			t.Fatalf("Expected index %s to exist once, got %d", indexName, indexCount)
		}
	}
}

func BenchmarkGetArticles(b *testing.B) {
	dbFile := "bench.db"
	defer os.Remove(dbFile)

	db, err := NewDB(dbFile)
	if err != nil {
		b.Fatalf("Failed to create database: %v", err)
	}
	defer db.Close()

	err = db.Init()
	if err != nil {
		b.Fatalf("Failed to initialize database: %v", err)
	}

	feed := &models.Feed{
		Title:       "Bench Feed",
		URL:         "https://example.com/bench",
		Description: "Bench Description",
	}
	_, err = db.AddFeed(feed)
	if err != nil {
		b.Fatalf("Failed to add feed: %v", err)
	}

	feeds, _ := db.GetFeeds()
	feedID := feeds[0].ID

	ctx := context.Background()
	articles := make([]*models.Article, 500)
	for i := 0; i < 500; i++ {
		articles[i] = &models.Article{
			FeedID:      feedID,
			Title:       fmt.Sprintf("Bench Article %d", i),
			URL:         fmt.Sprintf("https://example.com/bench-%d", i),
			PublishedAt: time.Now().Add(-time.Duration(i) * time.Minute),
			IsRead:      i%3 == 0,
		}
	}
	db.SaveArticles(ctx, articles)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := db.GetArticles("", feedID, "", false, 50, 0)
		if err != nil {
			b.Fatalf("Failed to get articles: %v", err)
		}
	}
}

func TestCleanupOldArticles(t *testing.T) {
	dbFile := "test_cleanup.db"
	defer os.Remove(dbFile)

	db, err := NewDB(dbFile)
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer db.Close()

	err = db.Init()
	if err != nil {
		t.Fatalf("Failed to initialize database: %v", err)
	}

	feed := &models.Feed{
		Title:       "Test Feed",
		URL:         "https://example.com/test",
		Description: "Test Description",
	}
	_, err = db.AddFeed(feed)
	if err != nil {
		t.Fatalf("Failed to add feed: %v", err)
	}

	feeds, _ := db.GetFeeds()
	feedID := feeds[0].ID

	now := time.Now()
	articles := []*models.Article{
		{FeedID: feedID, Title: "Old 1", URL: "https://example.com/old1", PublishedAt: now.AddDate(0, -2, 0), IsRead: false, IsFavorite: false},
		{FeedID: feedID, Title: "Old 2", URL: "https://example.com/old2", PublishedAt: now.AddDate(0, -2, 0), IsRead: true, IsFavorite: false},
		{FeedID: feedID, Title: "Old Fav", URL: "https://example.com/oldfav", PublishedAt: now.AddDate(0, -2, 0), IsRead: false, IsFavorite: true},
		{FeedID: feedID, Title: "Week Old Unread", URL: "https://example.com/weekold", PublishedAt: now.AddDate(0, 0, -8), IsRead: false, IsFavorite: false},
		{FeedID: feedID, Title: "Week Old Read", URL: "https://example.com/weekoldread", PublishedAt: now.AddDate(0, 0, -8), IsRead: true, IsFavorite: false},
		{FeedID: feedID, Title: "Recent", URL: "https://example.com/recent", PublishedAt: now.AddDate(0, 0, -1), IsRead: false, IsFavorite: false},
	}

	for _, article := range articles {
		err = db.SaveArticle(article)
		if err != nil {
			t.Fatalf("Failed to save article: %v", err)
		}
	}

	allArticles, _ := db.GetArticles("", feedID, "", false, 100, 0)
	if len(allArticles) != 6 {
		t.Errorf("Expected 6 articles initially, got %d", len(allArticles))
	}
	for _, a := range allArticles {
		t.Logf("Before cleanup: %s (read: %v, fav: %v, published: %v)", a.Title, a.IsRead, a.IsFavorite, a.PublishedAt)
	}

	count, err := db.CleanupOldArticles(0)
	if err != nil {
		t.Fatalf("Failed to cleanup articles: %v", err)
	}

	t.Logf("Cleaned up %d articles", count)

	remainingArticles, _ := db.GetArticles("", feedID, "", false, 100, 0)
	t.Logf("Remaining articles: %d", len(remainingArticles))

	if len(remainingArticles) != 4 {
		t.Errorf("Expected 4 articles after cleanup, got %d", len(remainingArticles))
		for _, a := range remainingArticles {
			t.Logf("  - %s (read: %v, fav: %v, published: %v)", a.Title, a.IsRead, a.IsFavorite, a.PublishedAt)
		}
	}

	titles := make(map[string]bool)
	for _, a := range remainingArticles {
		titles[a.Title] = true
	}

	expectedTitles := []string{"Old Fav", "Week Old Unread", "Week Old Read", "Recent"}
	for _, expected := range expectedTitles {
		if !titles[expected] {
			t.Errorf("Expected article '%s' to remain after cleanup", expected)
		}
	}
}

func TestCleanupUnimportantArticles(t *testing.T) {
	dbFile := "test_cleanup_unimportant.db"
	defer os.Remove(dbFile)

	db, err := NewDB(dbFile)
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer db.Close()

	err = db.Init()
	if err != nil {
		t.Fatalf("Failed to initialize database: %v", err)
	}

	feed := &models.Feed{
		Title:       "Test Feed",
		URL:         "https://example.com/test",
		Description: "Test Description",
	}
	_, err = db.AddFeed(feed)
	if err != nil {
		t.Fatalf("Failed to add feed: %v", err)
	}

	feeds, _ := db.GetFeeds()
	feedID := feeds[0].ID

	articles := []*models.Article{
		{FeedID: feedID, Title: "Unread Unfav", URL: "https://example.com/1", PublishedAt: time.Now(), IsRead: false, IsFavorite: false},
		{FeedID: feedID, Title: "Read Unfav", URL: "https://example.com/2", PublishedAt: time.Now(), IsRead: true, IsFavorite: false},
		{FeedID: feedID, Title: "Unread Fav", URL: "https://example.com/3", PublishedAt: time.Now(), IsRead: false, IsFavorite: true},
		{FeedID: feedID, Title: "Read Fav", URL: "https://example.com/4", PublishedAt: time.Now(), IsRead: true, IsFavorite: true},
	}

	for _, article := range articles {
		err = db.SaveArticle(article)
		if err != nil {
			t.Fatalf("Failed to save article: %v", err)
		}
	}

	count, err := db.CleanupUnimportantArticles(0)
	if err != nil {
		t.Fatalf("Failed to cleanup articles: %v", err)
	}

	if count != 1 {
		t.Errorf("Expected to delete 1 article, deleted %d", count)
	}

	remainingArticles, _ := db.GetArticles("", feedID, "", false, 100, 0)
	if len(remainingArticles) != 3 {
		t.Errorf("Expected 3 articles after cleanup, got %d", len(remainingArticles))
	}

	titles := make(map[string]bool)
	for _, a := range remainingArticles {
		titles[a.Title] = true
	}

	expectedTitles := []string{"Read Unfav", "Unread Fav", "Read Fav"}
	for _, expected := range expectedTitles {
		if !titles[expected] {
			t.Errorf("Expected article '%s' to remain after cleanup", expected)
		}
	}
}
