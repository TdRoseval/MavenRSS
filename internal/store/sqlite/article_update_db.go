package sqlite

import (
	"context"
	"database/sql"
)

// UpdateArticleContent updates the content field for an article in the articles table.
func (db *DB) UpdateArticleContent(id int64, content string) error {
	db.WaitForReady()
	_, err := db.Exec("UPDATE articles SET content = ? WHERE id = ?", content, id)
	return err
}

// UpdateArticleTranslation updates the translated_title field for an article.
func (db *DB) UpdateArticleTranslation(id int64, translatedTitle string) error {
	db.WaitForReady()
	_, err := db.Exec("UPDATE articles SET translated_title = ? WHERE id = ?", translatedTitle, id)
	return err
}

// UpdateArticleSummary updates the cached summary for an article.
func (db *DB) UpdateArticleSummary(id int64, summary string) error {
	db.WaitForReady()
	_, err := db.Exec("UPDATE articles SET summary = ? WHERE id = ?", summary, id)
	return err
}

// UpdateArticleEmbeddings upserts normalized title and summary embeddings into the vec0 virtual table.
func (db *DB) UpdateArticleEmbeddings(articleID int64, titleEmb, summaryEmb []byte) error {
	db.WaitForReady()
	titleEmb, summaryEmb = ensureVecColumnBlobs(titleEmb, summaryEmb)
	return db.WithWriteTx(context.Background(), func(tx *sql.Tx) error {
		if _, err := tx.Exec(`DELETE FROM article_embeddings WHERE article_id = ?`, articleID); err != nil {
			return err
		}
		_, err := tx.Exec(
			`INSERT INTO article_embeddings (article_id, title_embedding, summary_embedding) VALUES (?, ?, ?)`,
			articleID, titleEmb, summaryEmb,
		)
		return err
	})
}

func ensureVecColumnBlobs(primary, secondary []byte) ([]byte, []byte) {
	switch {
	case len(primary) == 0 && len(secondary) == 0:
		zero := make([]byte, 1024*4)
		return zero, zero
	case len(primary) == 0:
		return secondary, secondary
	case len(secondary) == 0:
		return primary, primary
	default:
		return primary, secondary
	}
}

// ClearAllTranslations clears all translated titles from articles.
func (db *DB) ClearAllTranslations() error {
	db.WaitForReady()
	_, err := db.Exec("UPDATE articles SET translated_title = ''")
	return err
}

// ClearArticleTranslation clears the translated title for a single article.
func (db *DB) ClearArticleTranslation(id int64) error {
	db.WaitForReady()
	_, err := db.Exec("UPDATE articles SET translated_title = '' WHERE id = ?", id)
	return err
}

// ClearAllSummaries clears all summaries from articles.
func (db *DB) ClearAllSummaries() error {
	db.WaitForReady()
	_, err := db.Exec("UPDATE articles SET summary = ''")
	return err
}

// ClearArticleSummary clears the summary for a single article.
func (db *DB) ClearArticleSummary(id int64) error {
	db.WaitForReady()
	_, err := db.Exec("UPDATE articles SET summary = '' WHERE id = ?", id)
	return err
}

// ClearArticleContent clears the content for a single article.
func (db *DB) ClearArticleContent(id int64) error {
	db.WaitForReady()
	_, err := db.Exec("UPDATE articles SET content = '' WHERE id = ?", id)
	return err
}
