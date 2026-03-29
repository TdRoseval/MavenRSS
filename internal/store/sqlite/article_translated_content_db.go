package sqlite

import "database/sql"

// ArticleTranslatedContent represents a cached translated article content entry
type ArticleTranslatedContent struct {
	ID         int64
	ArticleID  int64
	Content    string
	TargetLang string
	Provider   string
	CreatedAt  string
}

// GetArticleTranslatedContent retrieves cached translated content for an article
func (db *DB) GetArticleTranslatedContent(articleID int64, targetLang string) (string, string, bool, error) {
	db.WaitForReady()
	var content, provider string
	err := db.QueryRow(
		`SELECT content, provider FROM article_translated_contents 
		 WHERE article_id = ? AND target_lang = ?`,
		articleID, targetLang,
	).Scan(&content, &provider)

	if err == sql.ErrNoRows {
		return "", "", false, nil
	}
	if err != nil {
		return "", "", false, err
	}
	return content, provider, true, nil
}

// SetArticleTranslatedContent stores or updates translated content for an article
func (db *DB) SetArticleTranslatedContent(articleID int64, content, targetLang, provider string) error {
	db.WaitForReady()
	_, err := db.Exec(
		`INSERT OR REPLACE INTO article_translated_contents 
			(article_id, content, target_lang, provider, created_at)
		 VALUES (?, ?, ?, ?, CURRENT_TIMESTAMP)`,
		articleID, content, targetLang, provider,
	)
	return err
}

// DeleteArticleTranslatedContent removes cached translated content for an article
func (db *DB) DeleteArticleTranslatedContent(articleID int64) error {
	db.WaitForReady()
	_, err := db.Exec(
		`DELETE FROM article_translated_contents WHERE article_id = ?`,
		articleID,
	)
	return err
}

// CleanupOldArticleTranslatedContents removes old translated content cache entries
func (db *DB) CleanupOldArticleTranslatedContents(maxAgeDays int, userID int64) (int64, error) {
	db.WaitForReady()
	var result sql.Result
	var err error
	if userID > 0 {
		result, err = db.Exec(`
			DELETE FROM article_translated_contents 
			WHERE created_at < datetime('now', '-' || ? || ' days')
			AND article_id IN (SELECT id FROM articles WHERE user_id = ?)
		`, maxAgeDays, userID)
	} else {
		result, err = db.Exec(
			`DELETE FROM article_translated_contents WHERE created_at < datetime('now', '-' || ? || ' days')`,
			maxAgeDays,
		)
	}
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

// GetArticleTranslatedContentCount returns the total number of cached translated content entries
func (db *DB) GetArticleTranslatedContentCount(userID int64) (int64, error) {
	db.WaitForReady()
	var count int64
	err := db.QueryRow(`
		SELECT COUNT(*) FROM article_translated_contents 
		WHERE article_id IN (SELECT id FROM articles WHERE user_id = ?)
	`, userID).Scan(&count)
	if err != nil {
		return 0, err
	}
	return count, nil
}

// ClearAllArticleTranslatedContents clears all translated article contents
func (db *DB) ClearAllArticleTranslatedContents() error {
	db.WaitForReady()
	_, err := db.Exec(`DELETE FROM article_translated_contents`)
	return err
}
