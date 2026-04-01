package sqlite

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"MavenRSS/internal/models"
)

func TestStartupCheck(t *testing.T) {
	dbFile := filepath.Join(t.TempDir(), "startup.db")

	db, err := NewDB(dbFile)
	if err != nil {
		t.Fatalf("NewDB() error = %v", err)
	}
	defer db.Close()

	result, err := db.StartupCheck()
	if err != nil {
		t.Fatalf("StartupCheck() error = %v", err)
	}

	if result.DriverName != fileDriverName {
		t.Fatalf("StartupCheck().DriverName = %q, want %q", result.DriverName, fileDriverName)
	}
	if result.SQLiteVersion == "" {
		t.Fatal("StartupCheck().SQLiteVersion is empty")
	}
	if result.VecVersion == "" {
		t.Fatal("StartupCheck().VecVersion is empty")
	}
	if !strings.EqualFold(result.JournalMode, "wal") {
		t.Fatalf("StartupCheck().JournalMode = %q, want wal", result.JournalMode)
	}
	if !result.ForeignKeysEnabled {
		t.Fatal("StartupCheck().ForeignKeysEnabled = false, want true")
	}
	if result.BusyTimeout != defaultBusyTimeout {
		t.Fatalf("StartupCheck().BusyTimeout = %d, want %d", result.BusyTimeout, defaultBusyTimeout)
	}
}

func TestMemoryDatabaseCompatibility(t *testing.T) {
	db, err := NewDB(":memory:")
	if err != nil {
		t.Fatalf("NewDB() error = %v", err)
	}
	defer db.Close()

	result, err := db.StartupCheck()
	if err != nil {
		t.Fatalf("StartupCheck() error = %v", err)
	}
	if !result.InMemory {
		t.Fatal("StartupCheck().InMemory = false, want true")
	}

	ctx := context.Background()
	conn1, err := db.Conn(ctx)
	if err != nil {
		t.Fatalf("Conn() error = %v", err)
	}
	defer conn1.Close()

	conn2, err := db.Conn(ctx)
	if err != nil {
		t.Fatalf("Conn() error = %v", err)
	}
	defer conn2.Close()

	if _, err := conn1.ExecContext(ctx, `CREATE TABLE memory_compatibility_test (value TEXT)`); err != nil {
		t.Fatalf("ExecContext(create) error = %v", err)
	}
	if _, err := conn1.ExecContext(ctx, `INSERT INTO memory_compatibility_test (value) VALUES (?)`, "shared"); err != nil {
		t.Fatalf("ExecContext(insert) error = %v", err)
	}

	var value string
	if err := conn2.QueryRowContext(ctx, `SELECT value FROM memory_compatibility_test LIMIT 1`).Scan(&value); err != nil {
		t.Fatalf("QueryRowContext() error = %v", err)
	}
	if value != "shared" {
		t.Fatalf("QueryRowContext() value = %q, want %q", value, "shared")
	}
}

func TestLegacyPragmaCompatibility(t *testing.T) {
	dbFile := filepath.Join(t.TempDir(), "legacy.db")

	db, err := NewDB(dbFile + "?_pragma=busy_timeout(10000)&_pragma=journal_mode(WAL)")
	if err != nil {
		t.Fatalf("NewDB() error = %v", err)
	}
	defer db.Close()

	result, err := db.StartupCheck()
	if err != nil {
		t.Fatalf("StartupCheck() error = %v", err)
	}

	if strings.Contains(result.DataSourceName, "_pragma=") {
		t.Fatalf("StartupCheck().DataSourceName = %q, want legacy pragmas removed", result.DataSourceName)
	}
}

func TestReopenExistingDatabaseCompatibility(t *testing.T) {
	dbFile := filepath.Join(t.TempDir(), "reopen.db")
	feedURL := "https://example.com/reopen"
	articleURL := "https://example.com/reopen/article"
	settingKey := "reopen_test_key"
	settingValue := "reopen_test_value"

	db, err := NewDB(dbFile)
	if err != nil {
		t.Fatalf("NewDB() error = %v", err)
	}

	if err := db.Init(); err != nil {
		db.Close()
		t.Fatalf("Init() error = %v", err)
	}

	if err := db.SetSetting(settingKey, settingValue); err != nil {
		db.Close()
		t.Fatalf("SetSetting() error = %v", err)
	}

	feed := &models.Feed{
		Title:    "Reopen Feed",
		URL:      feedURL,
		Category: "tests",
	}
	if _, err := db.AddFeed(feed); err != nil {
		db.Close()
		t.Fatalf("AddFeed() error = %v", err)
	}

	feeds, err := db.GetFeeds()
	if err != nil {
		db.Close()
		t.Fatalf("GetFeeds() error = %v", err)
	}
	if len(feeds) != 1 {
		db.Close()
		t.Fatalf("GetFeeds() returned %d feeds, want 1", len(feeds))
	}

	article := &models.Article{
		FeedID:      feeds[0].ID,
		Title:       "Reopen Article",
		URL:         articleURL,
		PublishedAt: time.Now().UTC().Truncate(time.Second),
	}
	if err := db.SaveArticle(article); err != nil {
		db.Close()
		t.Fatalf("SaveArticle() error = %v", err)
	}

	if err := db.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	reopened, err := NewDB(dbFile)
	if err != nil {
		t.Fatalf("NewDB(reopen) error = %v", err)
	}
	defer reopened.Close()

	startupResult, err := reopened.StartupCheck()
	if err != nil {
		t.Fatalf("StartupCheck() error after reopen = %v", err)
	}
	if startupResult.InMemory {
		t.Fatal("StartupCheck().InMemory = true after reopen, want false")
	}

	if err := reopened.Init(); err != nil {
		t.Fatalf("Init() error after reopen = %v", err)
	}

	gotSetting, err := reopened.GetSetting(settingKey)
	if err != nil {
		t.Fatalf("GetSetting() error after reopen = %v", err)
	}
	if gotSetting != settingValue {
		t.Fatalf("GetSetting() = %q, want %q", gotSetting, settingValue)
	}

	reopenedFeeds, err := reopened.GetFeeds()
	if err != nil {
		t.Fatalf("GetFeeds() error after reopen = %v", err)
	}
	if len(reopenedFeeds) != 1 {
		t.Fatalf("GetFeeds() after reopen returned %d feeds, want 1", len(reopenedFeeds))
	}
	if reopenedFeeds[0].URL != feedURL {
		t.Fatalf("GetFeeds()[0].URL = %q, want %q", reopenedFeeds[0].URL, feedURL)
	}

	reopenedArticles, err := reopened.GetArticles("all", reopenedFeeds[0].ID, "", false, 10, 0)
	if err != nil {
		t.Fatalf("GetArticles() error after reopen = %v", err)
	}
	if len(reopenedArticles) != 1 {
		t.Fatalf("GetArticles() after reopen returned %d articles, want 1", len(reopenedArticles))
	}
	if reopenedArticles[0].URL != articleURL {
		t.Fatalf("GetArticles()[0].URL = %q, want %q", reopenedArticles[0].URL, articleURL)
	}
}
