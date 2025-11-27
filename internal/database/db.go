package database

import (
	"database/sql"
	"strings"
	"sync"
	"time"

	_ "modernc.org/sqlite"
)

// DB wraps sql.DB with initialization state tracking.
type DB struct {
	*sql.DB
	ready chan struct{}
	once  sync.Once
}

// NewDB creates a new database connection with optimized settings.
func NewDB(dataSourceName string) (*DB, error) {
	// Add busy_timeout to prevent "database is locked" errors
	// Also enable WAL mode for better concurrency
	// Add performance optimizations: increase cache size, set synchronous=NORMAL
	if !strings.Contains(dataSourceName, "?") {
		dataSourceName += "?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=cache_size(-32000)&_pragma=synchronous(NORMAL)"
	} else {
		dataSourceName += "&_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=cache_size(-32000)&_pragma=synchronous(NORMAL)"
	}

	db, err := sql.Open("sqlite", dataSourceName)
	if err != nil {
		return nil, err
	}

	// Set connection pool limits for better performance
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)

	return &DB{
		DB:    db,
		ready: make(chan struct{}),
	}, nil
}

// Init initializes the database schema and settings.
func (db *DB) Init() error {
	var err error
	db.once.Do(func() {
		defer close(db.ready)

		if err = db.Ping(); err != nil {
			return
		}

		if err = initSchema(db.DB); err != nil {
			return
		}

		// Create settings table if not exists
		_, _ = db.Exec(`CREATE TABLE IF NOT EXISTS settings (
			key TEXT PRIMARY KEY,
			value TEXT
		)`)

		// Insert default settings if they don't exist
		_, _ = db.Exec(`INSERT OR IGNORE INTO settings (key, value) VALUES ('update_interval', '10')`)
		_, _ = db.Exec(`INSERT OR IGNORE INTO settings (key, value) VALUES ('translation_enabled', 'false')`)
		_, _ = db.Exec(`INSERT OR IGNORE INTO settings (key, value) VALUES ('target_language', 'zh-CN')`)
		_, _ = db.Exec(`INSERT OR IGNORE INTO settings (key, value) VALUES ('translation_provider', 'google')`)
		_, _ = db.Exec(`INSERT OR IGNORE INTO settings (key, value) VALUES ('deepl_api_key', '')`)
		_, _ = db.Exec(`INSERT OR IGNORE INTO settings (key, value) VALUES ('auto_cleanup_enabled', 'false')`)
		_, _ = db.Exec(`INSERT OR IGNORE INTO settings (key, value) VALUES ('max_cache_size_mb', '20')`)
		_, _ = db.Exec(`INSERT OR IGNORE INTO settings (key, value) VALUES ('max_article_age_days', '30')`)
		_, _ = db.Exec(`INSERT OR IGNORE INTO settings (key, value) VALUES ('language', 'en-US')`)
		_, _ = db.Exec(`INSERT OR IGNORE INTO settings (key, value) VALUES ('theme', 'auto')`)
		_, _ = db.Exec(`INSERT OR IGNORE INTO settings (key, value) VALUES ('last_article_update', '')`)
		_, _ = db.Exec(`INSERT OR IGNORE INTO settings (key, value) VALUES ('show_hidden_articles', 'false')`)
		_, _ = db.Exec(`INSERT OR IGNORE INTO settings (key, value) VALUES ('default_view_mode', 'original')`)

		// Migration: Add link column to feeds table if it doesn't exist
		// Note: SQLite doesn't support IF NOT EXISTS for ALTER TABLE ADD COLUMN.
		// Error is ignored - if column exists, the operation fails harmlessly.
		_, _ = db.Exec(`ALTER TABLE feeds ADD COLUMN link TEXT DEFAULT ''`)

		// Migration: Add discovery_completed column to feeds table
		// Error is ignored - if column exists, the operation fails harmlessly.
		_, _ = db.Exec(`ALTER TABLE feeds ADD COLUMN discovery_completed BOOLEAN DEFAULT 0`)

		// Migration: Add script_path column to feeds table for custom script support
		// Error is ignored - if column exists, the operation fails harmlessly.
		_, _ = db.Exec(`ALTER TABLE feeds ADD COLUMN script_path TEXT DEFAULT ''`)
	})
	return err
}

// WaitForReady blocks until the database is initialized.
func (db *DB) WaitForReady() {
	<-db.ready
}

func initSchema(db *sql.DB) error {
	query := `
	CREATE TABLE IF NOT EXISTS feeds (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		title TEXT,
		url TEXT UNIQUE,
		link TEXT DEFAULT '',
		description TEXT,
		category TEXT DEFAULT '',
		image_url TEXT DEFAULT '',
		last_updated DATETIME,
		last_error TEXT DEFAULT ''
	);

	CREATE TABLE IF NOT EXISTS articles (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		feed_id INTEGER,
		title TEXT,
		url TEXT UNIQUE,
		image_url TEXT,
		translated_title TEXT,
		content TEXT DEFAULT '',
		published_at DATETIME,
		is_read BOOLEAN DEFAULT 0,
		is_favorite BOOLEAN DEFAULT 0,
		is_hidden BOOLEAN DEFAULT 0,
		FOREIGN KEY(feed_id) REFERENCES feeds(id)
	);

	-- Create indexes for better query performance
	CREATE INDEX IF NOT EXISTS idx_articles_feed_id ON articles(feed_id);
	CREATE INDEX IF NOT EXISTS idx_articles_published_at ON articles(published_at DESC);
	CREATE INDEX IF NOT EXISTS idx_articles_is_read ON articles(is_read);
	CREATE INDEX IF NOT EXISTS idx_articles_is_favorite ON articles(is_favorite);
	CREATE INDEX IF NOT EXISTS idx_articles_is_hidden ON articles(is_hidden);
	CREATE INDEX IF NOT EXISTS idx_feeds_category ON feeds(category);

	-- Composite indexes for common query patterns
	CREATE INDEX IF NOT EXISTS idx_articles_feed_published ON articles(feed_id, published_at DESC);
	CREATE INDEX IF NOT EXISTS idx_articles_read_published ON articles(is_read, published_at DESC);
	CREATE INDEX IF NOT EXISTS idx_articles_fav_published ON articles(is_favorite, published_at DESC);
	`
	_, err := db.Exec(query)
	if err != nil {
		return err
	}

	// Run migrations for existing databases
	return runMigrations(db)
}

// runMigrations applies database migrations for existing databases
func runMigrations(db *sql.DB) error {
	// Migration: Add content and is_hidden columns if they don't exist
	// SQLite doesn't support IF NOT EXISTS for ALTER TABLE, so we ignore errors if columns already exist
	_, _ = db.Exec(`ALTER TABLE articles ADD COLUMN content TEXT DEFAULT ''`)
	_, _ = db.Exec(`ALTER TABLE articles ADD COLUMN is_hidden BOOLEAN DEFAULT 0`)
	_, _ = db.Exec(`ALTER TABLE feeds ADD COLUMN last_error TEXT DEFAULT ''`)

	return nil
}
