package sqlite

import (
	"os"
	"testing"
)

func TestLegacyArticlesWithoutClusterIDCanInit(t *testing.T) {
	dbFile := "test_legacy_cluster_id.db"
	defer func() { _ = os.Remove(dbFile) }()

	db := newMigrationTestDB(t, dbFile)
	defer db.Close()

	statements := []string{
		`CREATE TABLE users (id INTEGER PRIMARY KEY AUTOINCREMENT, username TEXT, email TEXT, password_hash TEXT, role TEXT DEFAULT 'admin', status TEXT DEFAULT 'active')`,
		`CREATE TABLE feeds (id INTEGER PRIMARY KEY AUTOINCREMENT, user_id INTEGER NOT NULL DEFAULT 1, title TEXT, url TEXT, category TEXT DEFAULT '', last_error TEXT DEFAULT '')`,
		`CREATE TABLE articles (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id INTEGER NOT NULL DEFAULT 1,
			feed_id INTEGER,
			title TEXT,
			url TEXT,
			image_url TEXT,
			audio_url TEXT DEFAULT '',
			video_url TEXT DEFAULT '',
			translated_title TEXT,
			published_at DATETIME,
			is_read BOOLEAN DEFAULT 0,
			is_favorite BOOLEAN DEFAULT 0,
			is_hidden BOOLEAN DEFAULT 0,
			is_read_later BOOLEAN DEFAULT 0,
			summary TEXT DEFAULT '',
			unique_id TEXT
		)`,
	}
	for _, stmt := range statements {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatalf("Failed to prepare legacy schema with %q: %v", stmt, err)
		}
	}

	if err := initSchema(db.DB); err != nil {
		t.Fatalf("initSchema failed for legacy database: %v", err)
	}

	for _, columnName := range []string{"cluster_id", "simhash_64", "simhash_b1", "simhash_b2", "simhash_b3", "simhash_b4"} {
		if !columnExists(db.DB, "articles", columnName) {
			t.Fatalf("Expected articles.%s to exist after initSchema", columnName)
		}
	}
}
