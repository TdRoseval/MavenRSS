package database

import (
	"database/sql"
)

// initSchema initializes the database schema by creating all tables and indexes.
// This is extracted from db.go for better code organization.
func initSchema(db *sql.DB) error {
	// First create tables
	query := `
	-- User table
	CREATE TABLE IF NOT EXISTS users (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		username TEXT UNIQUE NOT NULL,
		email TEXT UNIQUE NOT NULL,
		password_hash TEXT NOT NULL,
		role TEXT NOT NULL DEFAULT 'user',
		status TEXT NOT NULL DEFAULT 'pending',
		inherited_from INTEGER,
		has_inherited BOOLEAN DEFAULT 0,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY(inherited_from) REFERENCES users(id)
	);

	-- User quota table
	CREATE TABLE IF NOT EXISTS user_quota (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		user_id INTEGER NOT NULL UNIQUE,
		max_feeds INTEGER DEFAULT 100,
		max_articles INTEGER DEFAULT 100000,
		max_ai_tokens INTEGER DEFAULT 1000000,
		max_ai_concurrency INTEGER DEFAULT 5,
		max_feed_fetch_concurrency INTEGER DEFAULT 3,
		max_db_query_concurrency INTEGER DEFAULT 5,
		max_storage_mb INTEGER DEFAULT 500,
		used_feeds INTEGER DEFAULT 0,
		used_articles INTEGER DEFAULT 0,
		used_ai_tokens INTEGER DEFAULT 0,
		used_storage_mb INTEGER DEFAULT 0,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY(user_id) REFERENCES users(id) ON DELETE CASCADE
	);

	-- User sessions table
	CREATE TABLE IF NOT EXISTS user_sessions (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		user_id INTEGER NOT NULL,
		refresh_token TEXT UNIQUE NOT NULL,
		user_agent TEXT,
		ip_address TEXT,
		expires_at DATETIME NOT NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY(user_id) REFERENCES users(id) ON DELETE CASCADE
	);

	-- Pending registrations table
	CREATE TABLE IF NOT EXISTS pending_registrations (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		username TEXT UNIQUE NOT NULL,
		email TEXT UNIQUE NOT NULL,
		password_hash TEXT NOT NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS feeds (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		user_id INTEGER NOT NULL,
		title TEXT,
		url TEXT,
		link TEXT DEFAULT '',
		description TEXT,
		category TEXT DEFAULT '',
		image_url TEXT DEFAULT '',
		last_updated DATETIME,
		last_error TEXT DEFAULT '',
		UNIQUE(user_id, url),
		FOREIGN KEY(user_id) REFERENCES users(id) ON DELETE CASCADE
	);

	CREATE TABLE IF NOT EXISTS articles (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		user_id INTEGER NOT NULL,
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
		unique_id TEXT,
		UNIQUE(user_id, unique_id),
		FOREIGN KEY(feed_id) REFERENCES feeds(id),
		FOREIGN KEY(user_id) REFERENCES users(id) ON DELETE CASCADE
	);

	-- Translation cache table to avoid redundant API calls
	CREATE TABLE IF NOT EXISTS translation_cache (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		source_text_hash TEXT NOT NULL,
		source_text TEXT NOT NULL,
		target_lang TEXT NOT NULL,
		translated_text TEXT NOT NULL,
		provider TEXT NOT NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		UNIQUE(source_text_hash, target_lang, provider)
	);

	-- Article content cache table to store full article content
	CREATE TABLE IF NOT EXISTS article_contents (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		article_id INTEGER NOT NULL UNIQUE,
		content TEXT NOT NULL,
		fetched_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY(article_id) REFERENCES articles(id) ON DELETE CASCADE
	);

	-- Chat sessions table to store AI chat conversations per article
	CREATE TABLE IF NOT EXISTS chat_sessions (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		user_id INTEGER NOT NULL,
		article_id INTEGER NOT NULL,
		title TEXT NOT NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY(article_id) REFERENCES articles(id) ON DELETE CASCADE,
		FOREIGN KEY(user_id) REFERENCES users(id) ON DELETE CASCADE
	);

	-- Chat messages table to store individual messages in chat sessions
	CREATE TABLE IF NOT EXISTS chat_messages (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		session_id INTEGER NOT NULL,
		role TEXT NOT NULL,
		content TEXT NOT NULL,
		thinking TEXT,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY(session_id) REFERENCES chat_sessions(id) ON DELETE CASCADE
	);

	-- Saved filters table
	CREATE TABLE IF NOT EXISTS saved_filters (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		user_id INTEGER NOT NULL,
		name TEXT NOT NULL,
		conditions TEXT NOT NULL,
		position INTEGER DEFAULT 0,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY(user_id) REFERENCES users(id) ON DELETE CASCADE
	);

	-- Tags table
	CREATE TABLE IF NOT EXISTS tags (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		user_id INTEGER NOT NULL,
		name TEXT NOT NULL,
		color TEXT,
		position INTEGER DEFAULT 0,
		UNIQUE(user_id, name),
		FOREIGN KEY(user_id) REFERENCES users(id) ON DELETE CASCADE
	);

	-- AI profiles table
	CREATE TABLE IF NOT EXISTS ai_profiles (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		user_id INTEGER NOT NULL,
		name TEXT NOT NULL,
		api_key TEXT,
		endpoint TEXT,
		model TEXT,
		custom_headers TEXT,
		is_default BOOLEAN DEFAULT 0,
		use_global_proxy BOOLEAN DEFAULT 0,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY(user_id) REFERENCES users(id) ON DELETE CASCADE
	);

	-- Global settings table
	CREATE TABLE IF NOT EXISTS settings (
		key TEXT PRIMARY KEY,
		value TEXT
	);

	-- User-specific settings table
	CREATE TABLE IF NOT EXISTS user_settings (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		user_id INTEGER NOT NULL,
		key TEXT NOT NULL,
		value TEXT NOT NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY(user_id) REFERENCES users(id) ON DELETE CASCADE,
		UNIQUE(user_id, key)
	);

	-- Create indexes for better query performance
	CREATE INDEX IF NOT EXISTS idx_users_username ON users(username);
	CREATE INDEX IF NOT EXISTS idx_users_email ON users(email);
	CREATE INDEX IF NOT EXISTS idx_users_role ON users(role);
	CREATE INDEX IF NOT EXISTS idx_users_status ON users(status);
	CREATE INDEX IF NOT EXISTS idx_articles_feed_id ON articles(feed_id);
	CREATE INDEX IF NOT EXISTS idx_articles_user_id ON articles(user_id);
	CREATE INDEX IF NOT EXISTS idx_articles_published_at ON articles(published_at DESC);
	CREATE INDEX IF NOT EXISTS idx_articles_is_read ON articles(is_read);
	CREATE INDEX IF NOT EXISTS idx_articles_is_favorite ON articles(is_favorite);
	CREATE INDEX IF NOT EXISTS idx_articles_is_hidden ON articles(is_hidden);
	CREATE INDEX IF NOT EXISTS idx_articles_is_read_later ON articles(is_read_later);
	CREATE INDEX IF NOT EXISTS idx_feeds_user_id ON feeds(user_id);
	CREATE INDEX IF NOT EXISTS idx_feeds_category ON feeds(category);
	CREATE INDEX IF NOT EXISTS idx_user_sessions_user_id ON user_sessions(user_id);
	CREATE INDEX IF NOT EXISTS idx_user_sessions_token ON user_sessions(refresh_token);
	CREATE INDEX IF NOT EXISTS idx_saved_filters_user_id ON saved_filters(user_id);
	CREATE INDEX IF NOT EXISTS idx_tags_user_id ON tags(user_id);
	CREATE INDEX IF NOT EXISTS idx_ai_profiles_user_id ON ai_profiles(user_id);
	CREATE INDEX IF NOT EXISTS idx_chat_sessions_user_id ON chat_sessions(user_id);

	-- Composite indexes for common query patterns
	CREATE INDEX IF NOT EXISTS idx_articles_feed_published ON articles(feed_id, published_at DESC);
	CREATE INDEX IF NOT EXISTS idx_articles_user_published ON articles(user_id, published_at DESC);
	CREATE INDEX IF NOT EXISTS idx_articles_read_published ON articles(is_read, published_at DESC);
	CREATE INDEX IF NOT EXISTS idx_articles_fav_published ON articles(is_favorite, published_at DESC);
	CREATE INDEX IF NOT EXISTS idx_articles_readlater_published ON articles(is_read_later, published_at DESC);
	CREATE INDEX IF NOT EXISTS idx_articles_user_read_published ON articles(user_id, is_read, published_at DESC);

	-- Covering index for category queries (hidden + published_at)
	-- Optimizes queries with: WHERE is_hidden = 0 ORDER BY published_at DESC
	CREATE INDEX IF NOT EXISTS idx_articles_hidden_published ON articles(is_hidden, published_at DESC);

	-- Composite index for common unread articles with hide_from_timeline filter
	CREATE INDEX IF NOT EXISTS idx_articles_unread_hidden_published ON articles(is_read, is_hidden, published_at DESC);

	-- Composite index for feed + hidden + published (for per-feed queries)
	CREATE INDEX IF NOT EXISTS idx_articles_feed_hidden_published ON articles(feed_id, is_hidden, published_at DESC);

	-- Composite index for unread per feed
	CREATE INDEX IF NOT EXISTS idx_articles_feed_read_published ON articles(feed_id, is_read, published_at DESC);

	-- Unique ID index for deduplication (critical for import performance)
	CREATE INDEX IF NOT EXISTS idx_articles_unique_id ON articles(unique_id);

	-- Translation cache index
	CREATE INDEX IF NOT EXISTS idx_translation_cache_lookup ON translation_cache(source_text_hash, target_lang, provider);

	-- Article content cache index
	CREATE INDEX IF NOT EXISTS idx_article_contents_article_id ON article_contents(article_id);

	-- Chat sessions and messages indexes
	CREATE INDEX IF NOT EXISTS idx_chat_sessions_article_id ON chat_sessions(article_id);
	CREATE INDEX IF NOT EXISTS idx_chat_sessions_updated_at ON chat_sessions(updated_at DESC);
	CREATE INDEX IF NOT EXISTS idx_chat_messages_session_id ON chat_messages(session_id);
	`
	_, err := db.Exec(query)
	if err != nil {
		return err
	}

	// Then run migrations to ensure all columns exist
	// This must happen AFTER creating tables
	if err := runMigrations(db); err != nil {
		return err
	}

	return nil
}
