package database

import (
	"database/sql"
	"log"
	"time"

	"MrRSS/internal/models"

	_ "modernc.org/sqlite"
)

type DB struct {
	*sql.DB
}

func NewDB(dataSourceName string) (*DB, error) {
	db, err := sql.Open("sqlite", dataSourceName)
	if err != nil {
		return nil, err
	}

	if err := db.Ping(); err != nil {
		return nil, err
	}

	if err := initSchema(db); err != nil {
		return nil, err
	}

	// Migration: Add category column if not exists
	_, _ = db.Exec("ALTER TABLE feeds ADD COLUMN category TEXT DEFAULT ''")
	// Migration: Add image_url to feeds
	_, _ = db.Exec("ALTER TABLE feeds ADD COLUMN image_url TEXT DEFAULT ''")
	// Migration: Add summary and image_url to articles
	_, _ = db.Exec("ALTER TABLE articles ADD COLUMN summary TEXT DEFAULT ''")
	_, _ = db.Exec("ALTER TABLE articles ADD COLUMN image_url TEXT DEFAULT ''")
	// Migration: Add translated_title to articles
	_, _ = db.Exec("ALTER TABLE articles ADD COLUMN translated_title TEXT DEFAULT ''")

	// Migration: Create settings table
	_, _ = db.Exec(`CREATE TABLE IF NOT EXISTS settings (
		key TEXT PRIMARY KEY,
		value TEXT
	)`)
	// Default settings
	_, _ = db.Exec(`INSERT OR IGNORE INTO settings (key, value) VALUES ('update_interval', '10')`)
	// Default settings for translation
	_, _ = db.Exec(`INSERT OR IGNORE INTO settings (key, value) VALUES ('translation_enabled', 'false')`)
	_, _ = db.Exec(`INSERT OR IGNORE INTO settings (key, value) VALUES ('target_language', 'es')`)
	_, _ = db.Exec(`INSERT OR IGNORE INTO settings (key, value) VALUES ('translation_provider', 'google')`)
	_, _ = db.Exec(`INSERT OR IGNORE INTO settings (key, value) VALUES ('deepl_api_key', '')`)

	return &DB{db}, nil
}

func initSchema(db *sql.DB) error {
	query := `
	CREATE TABLE IF NOT EXISTS feeds (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		title TEXT,
		url TEXT UNIQUE,
		description TEXT,
		category TEXT DEFAULT '',
		image_url TEXT DEFAULT '',
		last_updated DATETIME
	);

	CREATE TABLE IF NOT EXISTS articles (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		feed_id INTEGER,
		title TEXT,
		url TEXT UNIQUE,
		content TEXT,
		summary TEXT,
		image_url TEXT,
		translated_title TEXT,
		published_at DATETIME,
		is_read BOOLEAN DEFAULT 0,
		is_favorite BOOLEAN DEFAULT 0,
		FOREIGN KEY(feed_id) REFERENCES feeds(id)
	);
	`
	_, err := db.Exec(query)
	return err
}

func (db *DB) AddFeed(feed *models.Feed) error {
	query := `INSERT OR IGNORE INTO feeds (title, url, description, category, image_url, last_updated) VALUES (?, ?, ?, ?, ?, ?)`
	_, err := db.Exec(query, feed.Title, feed.URL, feed.Description, feed.Category, feed.ImageURL, time.Now())
	return err
}

func (db *DB) DeleteFeed(id int64) error {
	// First delete associated articles
	_, err := db.Exec("DELETE FROM articles WHERE feed_id = ?", id)
	if err != nil {
		return err
	}
	_, err = db.Exec("DELETE FROM feeds WHERE id = ?", id)
	return err
}

func (db *DB) GetFeeds() ([]models.Feed, error) {
	rows, err := db.Query("SELECT id, title, url, description, category, image_url, last_updated FROM feeds")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var feeds []models.Feed
	for rows.Next() {
		var f models.Feed
		var category, imageURL sql.NullString
		if err := rows.Scan(&f.ID, &f.Title, &f.URL, &f.Description, &category, &imageURL, &f.LastUpdated); err != nil {
			return nil, err
		}
		f.Category = category.String
		f.ImageURL = imageURL.String
		feeds = append(feeds, f)
	}
	return feeds, nil
}

func (db *DB) SaveArticle(article *models.Article) error {
	query := `INSERT OR IGNORE INTO articles (feed_id, title, url, content, summary, image_url, published_at, translated_title) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`
	_, err := db.Exec(query, article.FeedID, article.Title, article.URL, article.Content, article.Summary, article.ImageURL, article.PublishedAt, article.TranslatedTitle)
	return err
}

func (db *DB) SaveArticles(articles []*models.Article) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(`INSERT OR IGNORE INTO articles (feed_id, title, url, content, summary, image_url, published_at, translated_title) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, article := range articles {
		_, err := stmt.Exec(article.FeedID, article.Title, article.URL, article.Content, article.Summary, article.ImageURL, article.PublishedAt, article.TranslatedTitle)
		if err != nil {
			log.Println("Error saving article in batch:", err)
			// Continue even if one fails
		}
	}

	return tx.Commit()
}

func (db *DB) GetArticles(filter string, feedID int64, category string, limit, offset int) ([]models.Article, error) {
	baseQuery := `
		SELECT a.id, a.feed_id, a.title, a.url, a.content, a.summary, a.image_url, a.published_at, a.is_read, a.is_favorite, a.translated_title, f.title 
		FROM articles a 
		JOIN feeds f ON a.feed_id = f.id 
	`
	var args []interface{}
	whereClauses := []string{}

	if filter == "unread" {
		whereClauses = append(whereClauses, "a.is_read = 0")
	} else if filter == "favorites" {
		whereClauses = append(whereClauses, "a.is_favorite = 1")
	}

	if feedID > 0 {
		whereClauses = append(whereClauses, "a.feed_id = ?")
		args = append(args, feedID)
	}

	if category != "" {
		// Simple prefix match for category hierarchy
		whereClauses = append(whereClauses, "(f.category = ? OR f.category LIKE ?)")
		args = append(args, category, category+"/%")
	}

	query := baseQuery
	if len(whereClauses) > 0 {
		query += " WHERE " + whereClauses[0]
		for i := 1; i < len(whereClauses); i++ {
			query += " AND " + whereClauses[i]
		}
	}
	query += " ORDER BY a.published_at DESC LIMIT ? OFFSET ?"
	args = append(args, limit, offset)

	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var articles []models.Article
	for rows.Next() {
		var a models.Article
		var summary, imageURL, translatedTitle sql.NullString
		if err := rows.Scan(&a.ID, &a.FeedID, &a.Title, &a.URL, &a.Content, &summary, &imageURL, &a.PublishedAt, &a.IsRead, &a.IsFavorite, &translatedTitle, &a.FeedTitle); err != nil {
			log.Println("Error scanning article:", err)
			continue
		}
		a.Summary = summary.String
		a.ImageURL = imageURL.String
		a.TranslatedTitle = translatedTitle.String
		articles = append(articles, a)
	}
	return articles, nil
}

func (db *DB) UpdateFeed(id int64, title, url, category string) error {
	_, err := db.Exec("UPDATE feeds SET title = ?, url = ?, category = ? WHERE id = ?", title, url, category, id)
	return err
}

func (db *DB) UpdateFeedCategory(id int64, category string) error {
	_, err := db.Exec("UPDATE feeds SET category = ? WHERE id = ?", category, id)
	return err
}

func (db *DB) UpdateFeedImage(id int64, imageURL string) error {
	_, err := db.Exec("UPDATE feeds SET image_url = ? WHERE id = ?", imageURL, id)
	return err
}

func (db *DB) MarkArticleRead(id int64, read bool) error {
	isRead := 0
	if read {
		isRead = 1
	}
	_, err := db.Exec("UPDATE articles SET is_read = ? WHERE id = ?", isRead, id)
	return err
}

func (db *DB) ToggleFavorite(id int64) error {
	// First get current state
	var isFav bool
	err := db.QueryRow("SELECT is_favorite FROM articles WHERE id = ?", id).Scan(&isFav)
	if err != nil {
		return err
	}
	_, err = db.Exec("UPDATE articles SET is_favorite = ? WHERE id = ?", !isFav, id)
	return err
}

func (db *DB) GetSetting(key string) (string, error) {
	var value string
	err := db.QueryRow("SELECT value FROM settings WHERE key = ?", key).Scan(&value)
	if err != nil {
		return "", err
	}
	return value, nil
}

func (db *DB) SetSetting(key, value string) error {
	_, err := db.Exec("INSERT OR REPLACE INTO settings (key, value) VALUES (?, ?)", key, value)
	return err
}
