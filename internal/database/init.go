package database

import (
	"fmt"

	"MrRSS/internal/config"

	_ "modernc.org/sqlite"
)

// Init initializes the database schema and settings.
// This method must be called before any database operations.
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

		// Initialize FreshRSS sync queue table
		if err = InitFreshRSSSyncTable(db.DB); err != nil {
			return
		}

		// Initialize statistics table
		if err = InitStatisticsTable(db.DB); err != nil {
			return
		}

		// Create settings table if not exists
		_, _ = db.Exec(`CREATE TABLE IF NOT EXISTS settings (
			key TEXT PRIMARY KEY,
			value TEXT
		)`)

		// Insert default settings if they don't exist (using centralized defaults from config)
		settingsKeys := config.SettingsKeys()
		for _, key := range settingsKeys {
			defaultVal := config.GetString(key)
			_, _ = db.Exec(fmt.Sprintf(`INSERT OR IGNORE INTO settings (key, value) VALUES ('%s', '%s')`, key, defaultVal))
		}

		// Apply additional migrations
		if err = applyAdditionalMigrations(db); err != nil {
			return
		}
	})
	return err
}

// applyAdditionalMigrations applies migrations that need to run after schema initialization
func applyAdditionalMigrations(db *DB) error {
	// Migration: Add link column to feeds table if it doesn't exist
	// Note: SQLite doesn't support IF NOT EXISTS for ALTER TABLE ADD COLUMN.
	// Error is ignored - if column exists, the operation fails harmlessly.
	_, _ = db.Exec(`ALTER TABLE feeds ADD COLUMN link TEXT DEFAULT ''`)

	// Migration: Add discovery_completed column to feeds table
	_, _ = db.Exec(`ALTER TABLE feeds ADD COLUMN discovery_completed BOOLEAN DEFAULT 0`)

	// Migration: Add script_path column to feeds table for custom script support
	_, _ = db.Exec(`ALTER TABLE feeds ADD COLUMN script_path TEXT DEFAULT ''`)

	// Migration: Add hide_from_timeline column to feeds table
	_, _ = db.Exec(`ALTER TABLE feeds ADD COLUMN hide_from_timeline BOOLEAN DEFAULT 0`)

	// Migration: Add proxy and refresh interval columns to feeds table
	_, _ = db.Exec(`ALTER TABLE feeds ADD COLUMN proxy_url TEXT DEFAULT ''`)
	_, _ = db.Exec(`ALTER TABLE feeds ADD COLUMN proxy_enabled BOOLEAN DEFAULT 0`)
	_, _ = db.Exec(`ALTER TABLE feeds ADD COLUMN refresh_interval INTEGER DEFAULT 0`)

	// Migration: Add is_image_mode column to feeds table for image gallery feature
	_, _ = db.Exec(`ALTER TABLE feeds ADD COLUMN is_image_mode BOOLEAN DEFAULT 0`)

	// Migration: Add position column to feeds table for custom ordering
	_, _ = db.Exec(`ALTER TABLE feeds ADD COLUMN position INTEGER DEFAULT 0`)

	// Migration: Add article_view_mode column to feeds table for per-feed view mode override
	_, _ = db.Exec(`ALTER TABLE feeds ADD COLUMN article_view_mode TEXT DEFAULT 'global'`)

	// Migration: Add auto_expand_content column to feeds table for per-feed content expansion override
	_, _ = db.Exec(`ALTER TABLE feeds ADD COLUMN auto_expand_content TEXT DEFAULT 'global'`)

	// Migration: Add is_freshrss_source column to feeds table to mark feeds from FreshRSS
	_, _ = db.Exec(`ALTER TABLE feeds ADD COLUMN is_freshrss_source BOOLEAN DEFAULT 0`)

	// Migration: Add freshrss_stream_id column to feeds table to store FreshRSS stream ID
	_, _ = db.Exec(`ALTER TABLE feeds ADD COLUMN freshrss_stream_id TEXT DEFAULT ''`)

	// Migration: Add summary column to articles table for AI-generated summaries
	_, _ = db.Exec(`ALTER TABLE articles ADD COLUMN summary TEXT DEFAULT ''`)

	// Run complex table migrations
	if err := migrateUniqueIDOnArticles(db.DB); err != nil {
		return err
	}

	if err := migrateDropUniqueConstraintOnArticles(db.DB); err != nil {
		return err
	}

	if err := migrateDropUniqueConstraintOnFeeds(db.DB); err != nil {
		return err
	}

	return nil
}
