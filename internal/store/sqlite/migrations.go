package sqlite

import (
	"database/sql"
	"log"
	"strings"
)

func tableExists(db *sql.DB, tableName string) bool {
	var name string
	if err := db.QueryRow(`SELECT name FROM sqlite_master WHERE type = 'table' AND name = ?`, tableName).Scan(&name); err != nil {
		return false
	}
	return name == tableName
}

func columnExists(db *sql.DB, tableName, columnName string) bool {
	rows, err := db.Query(`PRAGMA table_info(` + tableName + `)`)
	if err != nil {
		return false
	}
	defer rows.Close()

	for rows.Next() {
		var cid int
		var name string
		var dataType string
		var notNull int
		var defaultValue sql.NullString
		var pk int
		if err := rows.Scan(&cid, &name, &dataType, &notNull, &defaultValue, &pk); err != nil {
			return false
		}
		if name == columnName {
			return true
		}
	}

	return false
}

func ensureChatSchema(db *sql.DB) {
	_, _ = db.Exec(`CREATE TABLE IF NOT EXISTS chat_sessions (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		user_id INTEGER NOT NULL DEFAULT 0,
		article_id INTEGER NOT NULL,
		title TEXT NOT NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY(user_id) REFERENCES users(id) ON DELETE CASCADE,
		FOREIGN KEY(article_id) REFERENCES articles(id) ON DELETE CASCADE
	)`)
	if columnExists(db, "chat_sessions", "user_id") {
		_, _ = db.Exec(`UPDATE chat_sessions SET user_id = (
			SELECT articles.user_id FROM articles WHERE articles.id = chat_sessions.article_id
		) WHERE user_id = 0`)
	} else {
		_, _ = db.Exec(`ALTER TABLE chat_sessions ADD COLUMN user_id INTEGER NOT NULL DEFAULT 0`)
		_, _ = db.Exec(`UPDATE chat_sessions SET user_id = (
			SELECT articles.user_id FROM articles WHERE articles.id = chat_sessions.article_id
		) WHERE user_id = 0`)
	}
	_, _ = db.Exec(`CREATE TABLE IF NOT EXISTS chat_messages (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		session_id INTEGER NOT NULL,
		role TEXT NOT NULL,
		content TEXT NOT NULL,
		thinking TEXT,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY(session_id) REFERENCES chat_sessions(id) ON DELETE CASCADE
	)`)
	_, _ = db.Exec(`CREATE INDEX IF NOT EXISTS idx_chat_sessions_user_id ON chat_sessions(user_id)`)
	_, _ = db.Exec(`CREATE INDEX IF NOT EXISTS idx_chat_sessions_article_id ON chat_sessions(article_id)`)
	_, _ = db.Exec(`CREATE INDEX IF NOT EXISTS idx_chat_sessions_updated_at ON chat_sessions(updated_at DESC)`)
	_, _ = db.Exec(`CREATE INDEX IF NOT EXISTS idx_chat_messages_session_id ON chat_messages(session_id)`)
}

func ensureDailyRecommendationSchema(db *sql.DB) {
	if !tableExists(db, "clusters") {
		return
	}

	if !columnExists(db, "clusters", "recommendation_archive_date") {
		_, _ = db.Exec(`ALTER TABLE clusters ADD COLUMN recommendation_archive_date TEXT DEFAULT ''`)
	}
	if !columnExists(db, "clusters", "recommendation_score") {
		_, _ = db.Exec(`ALTER TABLE clusters ADD COLUMN recommendation_score REAL DEFAULT 0`)
	}
	if !columnExists(db, "clusters", "is_ai_recommended") {
		_, _ = db.Exec(`ALTER TABLE clusters ADD COLUMN is_ai_recommended BOOLEAN DEFAULT 0`)
	}
	if !columnExists(db, "clusters", "recommendation_profile_id") {
		_, _ = db.Exec(`ALTER TABLE clusters ADD COLUMN recommendation_profile_id INTEGER DEFAULT 0`)
	}

	_, _ = db.Exec(`CREATE TABLE IF NOT EXISTS daily_recommendations (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		user_id INTEGER NOT NULL,
		cluster_id INTEGER NOT NULL,
		recommendation_date TEXT NOT NULL,
		recommendation_score REAL DEFAULT 0,
		recommendation_rank INTEGER DEFAULT 0,
		recommendation_profile_id INTEGER DEFAULT 0,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY(user_id) REFERENCES users(id) ON DELETE CASCADE,
		FOREIGN KEY(cluster_id) REFERENCES clusters(id) ON DELETE CASCADE,
		UNIQUE(user_id, recommendation_date, cluster_id)
	)`)
	_, _ = db.Exec(`CREATE INDEX IF NOT EXISTS idx_clusters_user_ai_recommended ON clusters(user_id, is_ai_recommended)`)
	_, _ = db.Exec(`CREATE INDEX IF NOT EXISTS idx_clusters_archive_date ON clusters(user_id, recommendation_archive_date DESC)`)
	_, _ = db.Exec(`CREATE INDEX IF NOT EXISTS idx_daily_recommendations_user_date ON daily_recommendations(user_id, recommendation_date DESC)`)
	_, _ = db.Exec(`CREATE INDEX IF NOT EXISTS idx_daily_recommendations_cluster ON daily_recommendations(cluster_id)`)
}

func runMigrations(db *sql.DB) error {
	_, _ = db.Exec(`ALTER TABLE articles ADD COLUMN content TEXT DEFAULT ''`)
	_, _ = db.Exec(`ALTER TABLE articles ADD COLUMN is_hidden BOOLEAN DEFAULT 0`)
	_, _ = db.Exec(`ALTER TABLE feeds ADD COLUMN last_error TEXT DEFAULT ''`)

	_, _ = db.Exec(`ALTER TABLE articles ADD COLUMN is_read_later BOOLEAN DEFAULT 0`)

	_, _ = db.Exec(`ALTER TABLE articles ADD COLUMN audio_url TEXT DEFAULT ''`)

	_, _ = db.Exec(`ALTER TABLE articles ADD COLUMN video_url TEXT DEFAULT ''`)

	_, _ = db.Exec(`UPDATE articles SET user_id = (SELECT feeds.user_id FROM feeds WHERE feeds.id = articles.feed_id) WHERE articles.user_id = 0`)

	_, _ = db.Exec(`ALTER TABLE feeds ADD COLUMN type TEXT DEFAULT ''`)
	_, _ = db.Exec(`ALTER TABLE feeds ADD COLUMN xpath_item TEXT DEFAULT ''`)
	_, _ = db.Exec(`ALTER TABLE feeds ADD COLUMN xpath_item_title TEXT DEFAULT ''`)
	_, _ = db.Exec(`ALTER TABLE feeds ADD COLUMN xpath_item_content TEXT DEFAULT ''`)
	_, _ = db.Exec(`ALTER TABLE feeds ADD COLUMN xpath_item_uri TEXT DEFAULT ''`)
	_, _ = db.Exec(`ALTER TABLE feeds ADD COLUMN xpath_item_author TEXT DEFAULT ''`)
	_, _ = db.Exec(`ALTER TABLE feeds ADD COLUMN xpath_item_timestamp TEXT DEFAULT ''`)
	_, _ = db.Exec(`ALTER TABLE feeds ADD COLUMN xpath_item_time_format TEXT DEFAULT ''`)
	_, _ = db.Exec(`ALTER TABLE feeds ADD COLUMN xpath_item_thumbnail TEXT DEFAULT ''`)
	_, _ = db.Exec(`ALTER TABLE feeds ADD COLUMN xpath_item_categories TEXT DEFAULT ''`)
	_, _ = db.Exec(`ALTER TABLE feeds ADD COLUMN xpath_item_uid TEXT DEFAULT ''`)

	_, _ = db.Exec(`ALTER TABLE articles ADD COLUMN summary TEXT DEFAULT ''`)

	_, _ = db.Exec(`CREATE TABLE IF NOT EXISTS article_contents (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		article_id INTEGER NOT NULL UNIQUE,
		content TEXT NOT NULL,
		fetched_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY(article_id) REFERENCES articles(id) ON DELETE CASCADE
	)`)
	_, _ = db.Exec(`CREATE INDEX IF NOT EXISTS idx_article_contents_article_id ON article_contents(article_id)`)

	_, _ = db.Exec(`CREATE TABLE IF NOT EXISTS translation_cache (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		source_text_hash TEXT NOT NULL,
		source_text TEXT NOT NULL,
		target_lang TEXT NOT NULL,
		translated_text TEXT NOT NULL,
		provider TEXT NOT NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		UNIQUE(source_text_hash, target_lang, provider)
	)`)
	_, _ = db.Exec(`CREATE INDEX IF NOT EXISTS idx_translation_cache_lookup ON translation_cache(source_text_hash, target_lang, provider)`)

	ensureChatSchema(db)

	_, _ = db.Exec(`ALTER TABLE feeds ADD COLUMN email_address TEXT DEFAULT ''`)
	_, _ = db.Exec(`ALTER TABLE feeds ADD COLUMN email_imap_server TEXT DEFAULT ''`)
	_, _ = db.Exec(`ALTER TABLE feeds ADD COLUMN email_imap_port INTEGER DEFAULT 993`)
	_, _ = db.Exec(`ALTER TABLE feeds ADD COLUMN email_username TEXT DEFAULT ''`)
	_, _ = db.Exec(`ALTER TABLE feeds ADD COLUMN email_password TEXT DEFAULT ''`)
	_, _ = db.Exec(`ALTER TABLE feeds ADD COLUMN email_folder TEXT DEFAULT 'INBOX'`)
	_, _ = db.Exec(`ALTER TABLE feeds ADD COLUMN email_last_uid INTEGER DEFAULT 0`)

	_, _ = db.Exec(`ALTER TABLE feeds ADD COLUMN is_freshrss_source BOOLEAN DEFAULT 0`)
	_, _ = db.Exec(`ALTER TABLE feeds ADD COLUMN freshrss_stream_id TEXT DEFAULT ''`)
	_, _ = db.Exec(`ALTER TABLE articles ADD COLUMN freshrss_item_id TEXT DEFAULT ''`)

	_, _ = db.Exec(`ALTER TABLE articles ADD COLUMN author TEXT DEFAULT ''`)

	_, _ = db.Exec(`CREATE TABLE IF NOT EXISTS saved_filters (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT NOT NULL UNIQUE,
		conditions TEXT NOT NULL,
		position INTEGER DEFAULT 0,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	)`)
	_, _ = db.Exec(`CREATE INDEX IF NOT EXISTS idx_saved_filters_position ON saved_filters(position)`)

	_, _ = db.Exec(`CREATE TABLE IF NOT EXISTS tags (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT NOT NULL UNIQUE,
		color TEXT NOT NULL DEFAULT '#3B82F6',
		position INTEGER DEFAULT 0
	)`)
	_, _ = db.Exec(`CREATE INDEX IF NOT EXISTS idx_tags_position ON tags(position)`)

	_, _ = db.Exec(`CREATE TABLE IF NOT EXISTS feed_tags (
		feed_id INTEGER NOT NULL,
		tag_id INTEGER NOT NULL,
		PRIMARY KEY (feed_id, tag_id),
		FOREIGN KEY (feed_id) REFERENCES feeds(id) ON DELETE CASCADE,
		FOREIGN KEY (tag_id) REFERENCES tags(id) ON DELETE CASCADE
	)`)
	_, _ = db.Exec(`CREATE INDEX IF NOT EXISTS idx_feed_tags_feed_id ON feed_tags(feed_id)`)
	_, _ = db.Exec(`CREATE INDEX IF NOT EXISTS idx_feed_tags_tag_id ON feed_tags(tag_id)`)

	_, _ = db.Exec(`CREATE TABLE IF NOT EXISTS ai_profiles (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT NOT NULL,
		api_key TEXT DEFAULT '',
		endpoint TEXT NOT NULL,
		model TEXT NOT NULL,
		custom_headers TEXT DEFAULT '',
		timeout_seconds INTEGER DEFAULT 0,
		is_default BOOLEAN DEFAULT 0,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	)`)
	_, _ = db.Exec(`CREATE INDEX IF NOT EXISTS idx_ai_profiles_is_default ON ai_profiles(is_default)`)

	_, _ = db.Exec(`ALTER TABLE ai_profiles ADD COLUMN use_global_proxy BOOLEAN DEFAULT 1`)
	_, _ = db.Exec(`ALTER TABLE ai_profiles ADD COLUMN timeout_seconds INTEGER DEFAULT 0`)

	_, _ = db.Exec(`ALTER TABLE user_quota ADD COLUMN max_ai_tokens INTEGER DEFAULT 1000000`)
	_, _ = db.Exec(`ALTER TABLE user_quota ADD COLUMN max_ai_concurrency INTEGER DEFAULT 5`)
	_, _ = db.Exec(`ALTER TABLE user_quota ADD COLUMN used_ai_tokens INTEGER DEFAULT 0`)
	_, _ = db.Exec(`ALTER TABLE user_quota ADD COLUMN max_feed_fetch_concurrency INTEGER DEFAULT 3`)
	_, _ = db.Exec(`ALTER TABLE user_quota ADD COLUMN max_db_query_concurrency INTEGER DEFAULT 5`)

	_, _ = db.Exec(`ALTER TABLE user_quota ADD COLUMN max_media_cache_concurrency INTEGER DEFAULT 5`)
	_, _ = db.Exec(`ALTER TABLE user_quota ADD COLUMN max_rss_discovery_concurrency INTEGER DEFAULT 8`)
	_, _ = db.Exec(`ALTER TABLE user_quota ADD COLUMN max_rss_path_check_concurrency INTEGER DEFAULT 5`)
	_, _ = db.Exec(`ALTER TABLE user_quota ADD COLUMN max_translation_concurrency INTEGER DEFAULT 3`)

	_, _ = db.Exec(`ALTER TABLE feeds ADD COLUMN translate_articles BOOLEAN DEFAULT 0`)

	_, _ = db.Exec(`ALTER TABLE feeds ADD COLUMN etag TEXT DEFAULT ''`)
	_, _ = db.Exec(`ALTER TABLE feeds ADD COLUMN last_modified TEXT DEFAULT ''`)

	_, _ = db.Exec(`CREATE TABLE IF NOT EXISTS article_translated_contents (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		article_id INTEGER NOT NULL UNIQUE,
		content TEXT NOT NULL,
		target_lang TEXT NOT NULL,
		provider TEXT NOT NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY(article_id) REFERENCES articles(id) ON DELETE CASCADE
	)`)
	_, _ = db.Exec(`CREATE INDEX IF NOT EXISTS idx_article_translated_contents_article_id ON article_translated_contents(article_id)`)

	_, _ = db.Exec(`CREATE TABLE IF NOT EXISTS ai_article_stage_skips (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		user_id INTEGER NOT NULL,
		article_id INTEGER NOT NULL,
		stage TEXT NOT NULL,
		reason TEXT DEFAULT '',
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY(user_id) REFERENCES users(id) ON DELETE CASCADE,
		FOREIGN KEY(article_id) REFERENCES articles(id) ON DELETE CASCADE,
		UNIQUE(article_id, stage)
	)`)
	_, _ = db.Exec(`CREATE INDEX IF NOT EXISTS idx_ai_article_stage_skips_user_stage ON ai_article_stage_skips(user_id, stage)`)
	_, _ = db.Exec(`CREATE INDEX IF NOT EXISTS idx_ai_article_stage_skips_article_stage ON ai_article_stage_skips(article_id, stage)`)
	_, _ = db.Exec(`CREATE TABLE IF NOT EXISTS ai_article_stage_timeout_failures (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		user_id INTEGER NOT NULL,
		article_id INTEGER NOT NULL,
		stage TEXT NOT NULL,
		timeout_count INTEGER NOT NULL DEFAULT 0,
		last_reason TEXT DEFAULT '',
		first_failed_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		last_failed_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY(user_id) REFERENCES users(id) ON DELETE CASCADE,
		FOREIGN KEY(article_id) REFERENCES articles(id) ON DELETE CASCADE,
		UNIQUE(article_id, stage)
	)`)
	_, _ = db.Exec(`CREATE INDEX IF NOT EXISTS idx_ai_article_stage_timeout_failures_user_stage ON ai_article_stage_timeout_failures(user_id, stage)`)
	_, _ = db.Exec(`CREATE INDEX IF NOT EXISTS idx_ai_article_stage_timeout_failures_article_stage ON ai_article_stage_timeout_failures(article_id, stage)`)

	_, _ = db.Exec(`CREATE TABLE IF NOT EXISTS system_messages (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		user_id INTEGER NOT NULL,
		kind TEXT NOT NULL,
		title TEXT NOT NULL,
		body TEXT NOT NULL,
		metadata_json TEXT DEFAULT '',
		is_read BOOLEAN DEFAULT 0,
		read_at DATETIME DEFAULT NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY(user_id) REFERENCES users(id) ON DELETE CASCADE
	)`)
	_, _ = db.Exec(`CREATE INDEX IF NOT EXISTS idx_system_messages_user_updated ON system_messages(user_id, updated_at DESC)`)
	_, _ = db.Exec(`CREATE INDEX IF NOT EXISTS idx_system_messages_user_unread ON system_messages(user_id, is_read, updated_at DESC)`)
	_, _ = db.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS idx_system_messages_user_kind ON system_messages(user_id, kind)`)

	_, _ = db.Exec(`CREATE VIRTUAL TABLE IF NOT EXISTS article_embeddings USING vec0(
		article_id INTEGER PRIMARY KEY,
		user_id INTEGER partition key,
		title_embedding float[1024],
		summary_embedding float[1024],
		summary_embedding_bin bit[1024]
	)`)

	_, _ = db.Exec(`CREATE TABLE IF NOT EXISTS clusters (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		user_id INTEGER NOT NULL,
		status TEXT NOT NULL DEFAULT 'pending_merge',
		merged_title TEXT DEFAULT '',
		merged_summary TEXT DEFAULT '',
		merged_content TEXT DEFAULT '',
		article_count INTEGER DEFAULT 1,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		is_read BOOLEAN DEFAULT 0,
		is_favorite BOOLEAN DEFAULT 0,
		is_read_later BOOLEAN DEFAULT 0,
		is_hidden BOOLEAN DEFAULT 0,
		FOREIGN KEY(user_id) REFERENCES users(id) ON DELETE CASCADE
	)`)

	_, _ = db.Exec(`ALTER TABLE articles ADD COLUMN cluster_id INTEGER DEFAULT NULL`)
	_, _ = db.Exec(`ALTER TABLE articles ADD COLUMN simhash_64 INTEGER DEFAULT 0`)
	_, _ = db.Exec(`ALTER TABLE articles ADD COLUMN simhash_b1 INTEGER DEFAULT 0`)
	_, _ = db.Exec(`ALTER TABLE articles ADD COLUMN simhash_b2 INTEGER DEFAULT 0`)
	_, _ = db.Exec(`ALTER TABLE articles ADD COLUMN simhash_b3 INTEGER DEFAULT 0`)
	_, _ = db.Exec(`ALTER TABLE articles ADD COLUMN simhash_b4 INTEGER DEFAULT 0`)

	_, _ = db.Exec(`CREATE INDEX IF NOT EXISTS idx_articles_cluster_id ON articles(cluster_id)`)
	_, _ = db.Exec(`CREATE INDEX IF NOT EXISTS idx_articles_simhash_b1 ON articles(user_id, simhash_b1)`)
	_, _ = db.Exec(`CREATE INDEX IF NOT EXISTS idx_articles_simhash_b2 ON articles(user_id, simhash_b2)`)
	_, _ = db.Exec(`CREATE INDEX IF NOT EXISTS idx_articles_simhash_b3 ON articles(user_id, simhash_b3)`)
	_, _ = db.Exec(`CREATE INDEX IF NOT EXISTS idx_articles_simhash_b4 ON articles(user_id, simhash_b4)`)
	_, _ = db.Exec(`CREATE INDEX IF NOT EXISTS idx_clusters_user_id ON clusters(user_id)`)
	_, _ = db.Exec(`CREATE INDEX IF NOT EXISTS idx_clusters_status ON clusters(status)`)
	_, _ = db.Exec(`CREATE INDEX IF NOT EXISTS idx_clusters_updated_at ON clusters(updated_at DESC)`)
	_, _ = db.Exec(`CREATE INDEX IF NOT EXISTS idx_clusters_user_status ON clusters(user_id, status)`)
	_, _ = db.Exec(`CREATE INDEX IF NOT EXISTS idx_clusters_user_favorite ON clusters(user_id, is_favorite)`)
	_, _ = db.Exec(`CREATE INDEX IF NOT EXISTS idx_clusters_user_read ON clusters(user_id, is_read)`)

	_, _ = db.Exec(`CREATE VIRTUAL TABLE IF NOT EXISTS cluster_embeddings USING vec0(
		cluster_id INTEGER PRIMARY KEY,
		user_id INTEGER partition key,
		title_embedding float[1024],
		summary_embedding float[1024],
		summary_embedding_bin bit[1024]
	)`)

	ensureDailyRecommendationSchema(db)

	_, _ = db.Exec(`CREATE TABLE IF NOT EXISTS cluster_feed_first_page_cache (
		user_id INTEGER NOT NULL,
		filter TEXT NOT NULL,
		vector_hash TEXT NOT NULL,
		payload_json TEXT NOT NULL,
		generated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		PRIMARY KEY(user_id, filter),
		FOREIGN KEY(user_id) REFERENCES users(id) ON DELETE CASCADE
	)`)
	_, _ = db.Exec(`CREATE INDEX IF NOT EXISTS idx_cluster_feed_first_page_cache_user ON cluster_feed_first_page_cache(user_id)`)

	_, _ = db.Exec(`ALTER TABLE users ADD COLUMN interest_vector BLOB DEFAULT NULL`)
	_, _ = db.Exec(`ALTER TABLE users ADD COLUMN ai_read_count INTEGER DEFAULT 0`)
	_, _ = db.Exec(`ALTER TABLE users ADD COLUMN ai_total_read_time INTEGER DEFAULT 0`)

	_, _ = db.Exec(`CREATE VIRTUAL TABLE IF NOT EXISTS user_interest_embeddings USING vec0(
		user_id INTEGER PRIMARY KEY,
		interest_embedding float[1024]
	)`)

	// Rebuild embedding vec0 tables to the target schema (partition key +
	// binary-quantized recall column). Idempotent; skips up-to-date tables.
	if err := migrateVecTablesToLatest(db); err != nil {
		return err
	}

	return nil
}

func migrateUniqueIDOnArticles(db *sql.DB) error {
	_, _ = db.Exec(`ALTER TABLE articles ADD COLUMN unique_id TEXT UNIQUE`)

	_, err := db.Exec(`
		UPDATE articles
		SET unique_id = LOWER(HEX(MD5(title || '|' || feed_id || '|' || COALESCE(strftime('%Y-%m-%d', published_at), ''))))
		WHERE unique_id IS NULL
	`)
	if err != nil {
		log.Printf("Warning: Failed to migrate unique_id: %v", err)
	}

	result, err := db.Exec(`
		UPDATE articles
		SET published_at = datetime('now')
		WHERE published_at IS NULL
	`)
	if err != nil {
		log.Printf("Warning: Failed to backfill published_at: %v", err)
	} else {
		rowsAffected, _ := result.RowsAffected()
		if rowsAffected > 0 {
			log.Printf("Backfilled published_at for %d articles", rowsAffected)
		}
	}

	return nil
}

func migrateDropUniqueConstraintOnArticles(db *sql.DB) error {
	var tableInfo string
	_ = db.QueryRow("SELECT sql FROM sqlite_master WHERE type='table' AND name='articles'").Scan(&tableInfo)
	if strings.Contains(tableInfo, "url TEXT UNIQUE") {
		_, err := db.Exec(`
			CREATE TABLE articles_new (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
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
				unique_id TEXT UNIQUE,
				FOREIGN KEY(feed_id) REFERENCES feeds(id)
			)
		`)
		if err == nil {
			_, _ = db.Exec(`
				INSERT INTO articles_new (id, feed_id, title, url, image_url, audio_url, video_url, translated_title, published_at, is_read, is_favorite, is_hidden, is_read_later, summary, unique_id)
				SELECT id, feed_id, title, url, image_url, audio_url, video_url, translated_title, published_at, is_read, is_favorite, is_hidden, is_read_later,
					COALESCE(summary, '') as summary,
					LOWER(HEX(MD5(title || '|' || feed_id || '|' || COALESCE(strftime('%Y-%m-%d', published_at), '')))) as unique_id
				FROM articles
			`)
			_, _ = db.Exec(`DROP TABLE articles`)
			_, _ = db.Exec(`ALTER TABLE articles_new RENAME TO articles`)
			_, _ = db.Exec(`ALTER TABLE articles ADD COLUMN cluster_id INTEGER DEFAULT NULL`)
			_, _ = db.Exec(`ALTER TABLE articles ADD COLUMN simhash_64 INTEGER DEFAULT 0`)
			_, _ = db.Exec(`ALTER TABLE articles ADD COLUMN simhash_b1 INTEGER DEFAULT 0`)
			_, _ = db.Exec(`ALTER TABLE articles ADD COLUMN simhash_b2 INTEGER DEFAULT 0`)
			_, _ = db.Exec(`ALTER TABLE articles ADD COLUMN simhash_b3 INTEGER DEFAULT 0`)
			_, _ = db.Exec(`ALTER TABLE articles ADD COLUMN simhash_b4 INTEGER DEFAULT 0`)
			_, _ = db.Exec(`CREATE INDEX IF NOT EXISTS idx_articles_feed_id ON articles(feed_id)`)
			_, _ = db.Exec(`CREATE INDEX IF NOT EXISTS idx_articles_unique_id ON articles(unique_id)`)
			_, _ = db.Exec(`CREATE INDEX IF NOT EXISTS idx_articles_cluster_id ON articles(cluster_id)`)
			_, _ = db.Exec(`CREATE INDEX IF NOT EXISTS idx_articles_published_at ON articles(published_at DESC)`)
			_, _ = db.Exec(`CREATE INDEX IF NOT EXISTS idx_articles_is_read ON articles(is_read)`)
			_, _ = db.Exec(`CREATE INDEX IF NOT EXISTS idx_articles_is_favorite ON articles(is_favorite)`)
			_, _ = db.Exec(`CREATE INDEX IF NOT EXISTS idx_articles_is_hidden ON articles(is_hidden)`)
			_, _ = db.Exec(`CREATE INDEX IF NOT EXISTS idx_articles_is_read_later ON articles(is_read_later)`)
			_, _ = db.Exec(`CREATE INDEX IF NOT EXISTS idx_articles_feed_published ON articles(feed_id, published_at DESC)`)
			_, _ = db.Exec(`CREATE INDEX IF NOT EXISTS idx_articles_read_published ON articles(is_read, published_at DESC)`)
			_, _ = db.Exec(`CREATE INDEX IF NOT EXISTS idx_articles_fav_published ON articles(is_favorite, published_at DESC)`)
			_, _ = db.Exec(`CREATE INDEX IF NOT EXISTS idx_articles_readlater_published ON articles(is_read_later, published_at DESC)`)
			_, _ = db.Exec(`CREATE INDEX IF NOT EXISTS idx_articles_hidden_published ON articles(is_hidden, published_at DESC)`)
		}
	}
	return nil
}

func migrateDropUniqueConstraintOnFeeds(db *sql.DB) error {
	var feedsTableInfo string
	_ = db.QueryRow("SELECT sql FROM sqlite_master WHERE type='table' AND name='feeds'").Scan(&feedsTableInfo)
	if strings.Contains(feedsTableInfo, "url TEXT UNIQUE") {
		log.Printf("Migration: Dropping UNIQUE constraint on feeds.url to allow FreshRSS and local feeds to coexist")
		_, err := db.Exec(`
			CREATE TABLE feeds_new (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				title TEXT,
				url TEXT,
				link TEXT DEFAULT '',
				description TEXT,
				category TEXT DEFAULT '',
				image_url TEXT DEFAULT '',
				position INTEGER DEFAULT 0,
				last_updated DATETIME,
				last_error TEXT DEFAULT '',
				discovery_completed BOOLEAN DEFAULT 0,
				script_path TEXT DEFAULT '',
				hide_from_timeline BOOLEAN DEFAULT 0,
				proxy_url TEXT DEFAULT '',
				proxy_enabled BOOLEAN DEFAULT 0,
				refresh_interval INTEGER DEFAULT 0,
				is_image_mode BOOLEAN DEFAULT 0,
				type TEXT DEFAULT '',
				xpath_item TEXT DEFAULT '',
				xpath_item_title TEXT DEFAULT '',
				xpath_item_content TEXT DEFAULT '',
				xpath_item_uri TEXT DEFAULT '',
				xpath_item_author TEXT DEFAULT '',
				xpath_item_timestamp TEXT DEFAULT '',
				xpath_item_time_format TEXT DEFAULT '',
				xpath_item_thumbnail TEXT DEFAULT '',
				xpath_item_categories TEXT DEFAULT '',
				xpath_item_uid TEXT DEFAULT '',
				article_view_mode TEXT DEFAULT '',
				auto_expand_content TEXT DEFAULT '',
				email_address TEXT DEFAULT '',
				email_imap_server TEXT DEFAULT '',
				email_imap_port INTEGER DEFAULT 993,
				email_username TEXT DEFAULT '',
				email_password TEXT DEFAULT '',
				email_folder TEXT DEFAULT 'INBOX',
				email_last_uid INTEGER DEFAULT 0,
				is_freshrss_source BOOLEAN DEFAULT 0,
				freshrss_stream_id TEXT DEFAULT '',
				translate_articles BOOLEAN DEFAULT 0
			)
		`)
		if err == nil {
			_, err = db.Exec(`
				INSERT INTO feeds_new (
					id, title, url, link, description, category, image_url, position, last_updated, last_error,
					discovery_completed, script_path, hide_from_timeline, proxy_url, proxy_enabled, refresh_interval,
					is_image_mode, type, xpath_item, xpath_item_title, xpath_item_content, xpath_item_uri,
					xpath_item_author, xpath_item_timestamp, xpath_item_time_format, xpath_item_thumbnail,
					xpath_item_categories, xpath_item_uid, article_view_mode, auto_expand_content,
					email_address, email_imap_server, email_imap_port, email_username, email_password,
					email_folder, email_last_uid, is_freshrss_source, freshrss_stream_id, translate_articles
				)
				SELECT
					id, title, url, link, description, category, image_url,
					COALESCE(position, 0) as position,
					last_updated, COALESCE(last_error, '') as last_error,
					COALESCE(discovery_completed, 0) as discovery_completed,
					COALESCE(script_path, '') as script_path,
					COALESCE(hide_from_timeline, 0) as hide_from_timeline,
					COALESCE(proxy_url, '') as proxy_url,
					COALESCE(proxy_enabled, 0) as proxy_enabled,
					COALESCE(refresh_interval, 0) as refresh_interval,
					COALESCE(is_image_mode, 0) as is_image_mode,
					COALESCE(type, '') as type,
					COALESCE(xpath_item, '') as xpath_item,
					COALESCE(xpath_item_title, '') as xpath_item_title,
					COALESCE(xpath_item_content, '') as xpath_item_content,
					COALESCE(xpath_item_uri, '') as xpath_item_uri,
					COALESCE(xpath_item_author, '') as xpath_item_author,
					COALESCE(xpath_item_timestamp, '') as xpath_item_timestamp,
					COALESCE(xpath_item_time_format, '') as xpath_item_time_format,
					COALESCE(xpath_item_thumbnail, '') as xpath_item_thumbnail,
					COALESCE(xpath_item_categories, '') as xpath_item_categories,
					COALESCE(xpath_item_uid, '') as xpath_item_uid,
					COALESCE(article_view_mode, '') as article_view_mode,
					COALESCE(auto_expand_content, '') as auto_expand_content,
					COALESCE(email_address, '') as email_address,
					COALESCE(email_imap_server, '') as email_imap_server,
					COALESCE(email_imap_port, 993) as email_imap_port,
					COALESCE(email_username, '') as email_username,
					COALESCE(email_password, '') as email_password,
					COALESCE(email_folder, 'INBOX') as email_folder,
					COALESCE(email_last_uid, 0) as email_last_uid,
					COALESCE(is_freshrss_source, 0) as is_freshrss_source,
					COALESCE(freshrss_stream_id, '') as freshrss_stream_id,
					0 as translate_articles
				FROM feeds
			`)
			if err != nil {
				log.Printf("Error copying feeds data: %v", err)
			}
			_, _ = db.Exec(`DROP TABLE feeds`)
			_, _ = db.Exec(`ALTER TABLE feeds_new RENAME TO feeds`)
			_, _ = db.Exec(`CREATE INDEX IF NOT EXISTS idx_feeds_category ON feeds(category)`)
			log.Printf("Migration completed: UNIQUE constraint dropped from feeds.url")
		} else {
			log.Printf("Error creating feeds_new table: %v", err)
		}
	} else {
		_, _ = db.Exec(`ALTER TABLE feeds ADD COLUMN translate_articles BOOLEAN DEFAULT 0`)
		log.Printf("Migration: Added translate_articles column to feeds table")
	}
	return nil
}
