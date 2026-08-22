package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
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

// BatchUpdateArticleTranslation updates translated_title for multiple articles
// in a single write transaction using a CASE expression. This replaces the
// previous N+1 pattern in resolveCachedSingleClusterTitles where each cached
// translation hit triggered a separate UPDATE statement.
//
// items maps article ID to its translated title. Empty maps are no-ops.
func (db *DB) BatchUpdateArticleTranslation(items map[int64]string) error {
	db.WaitForReady()
	if len(items) == 0 {
		return nil
	}

	ids := make([]int64, 0, len(items))
	for id := range items {
		ids = append(ids, id)
	}

	placeholders := make([]string, len(ids))
	caseParts := make([]string, 0, len(ids))
	args := make([]interface{}, 0, len(ids)*2)

	for i, id := range ids {
		placeholders[i] = "?"
		caseParts = append(caseParts, "WHEN id = ? THEN ?")
		args = append(args, id, items[id])
	}

	// UPDATE articles SET translated_title = CASE WHEN id = ? THEN ? ... END WHERE id IN (?, ?, ...)
	query := fmt.Sprintf(
		"UPDATE articles SET translated_title = CASE %s END WHERE id IN (%s)",
		strings.Join(caseParts, " "),
		strings.Join(placeholders, ","),
	)
	for _, id := range ids {
		args = append(args, id)
	}

	_, err := db.Exec(query, args...)
	return err
}

// UpdateArticleSummary updates the cached summary for an article.
func (db *DB) UpdateArticleSummary(id int64, summary string) error {
	db.WaitForReady()
	_, err := db.Exec("UPDATE articles SET summary = ? WHERE id = ?", summary, id)
	return err
}

// UpdateArticleEmbeddings upserts normalized title and summary embeddings into the vec0 virtual table.
// The user_id partition key is resolved from the articles table so KNN scans
// only touch the owning user's partition.
func (db *DB) UpdateArticleEmbeddings(articleID int64, titleEmb, summaryEmb []byte) error {
	db.WaitForReady()
	// vec0 rejects NULL/zero-length vectors. When both embeddings are missing,
	// keep no row at all — it would otherwise contribute an 8KB zero vector to
	// every KNN scan and pollute "has embedding" checks.
	if len(titleEmb) == 0 && len(summaryEmb) == 0 {
		_, err := db.Exec(`DELETE FROM article_embeddings WHERE article_id = ?`, articleID)
		return err
	}
	titleEmb, summaryEmb = ensureVecColumnBlobs(titleEmb, summaryEmb)
	return db.WithWriteTx(context.Background(), func(tx *sql.Tx) error {
		if _, err := tx.Exec(`DELETE FROM article_embeddings WHERE article_id = ?`, articleID); err != nil {
			return err
		}
		_, err := tx.Exec(
			`INSERT INTO article_embeddings (article_id, user_id, title_embedding, summary_embedding, summary_embedding_bin)
			 VALUES (?, (SELECT user_id FROM articles WHERE id = ?), ?, ?, vec_quantize_binary(?))`,
			articleID, articleID, titleEmb, summaryEmb, summaryEmb,
		)
		return err
	})
}

func ensureVecColumnBlobs(primary, secondary []byte) ([]byte, []byte) {
	// vec0 rejects NULL vectors, so a missing column reuses the other column's
	// blob (both callers already handle the "both missing" case by not writing).
	switch {
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
