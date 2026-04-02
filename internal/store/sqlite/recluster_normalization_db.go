package sqlite

import (
	"database/sql"
	"fmt"

	"MavenRSS/internal/interest"
)

func (db *DB) ResetAIClustersForRenormalization(userID int64) error {
	db.WaitForReady()

	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("begin reset renormalization transaction: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.Exec(`DELETE FROM daily_recommendations WHERE user_id = ?`, userID); err != nil {
		return fmt.Errorf("delete daily recommendations: %w", err)
	}
	if _, err := tx.Exec(
		`DELETE FROM cluster_embeddings
		 WHERE cluster_id IN (SELECT id FROM clusters WHERE user_id = ?)`,
		userID,
	); err != nil {
		return fmt.Errorf("delete cluster embeddings: %w", err)
	}
	if _, err := tx.Exec(
		`UPDATE articles
		 SET cluster_id = NULL,
		     simhash_64 = 0,
		     simhash_b1 = 0,
		     simhash_b2 = 0,
		     simhash_b3 = 0,
		     simhash_b4 = 0
		 WHERE user_id = ?`,
		userID,
	); err != nil {
		return fmt.Errorf("reset article clustering fields: %w", err)
	}
	if _, err := tx.Exec(`DELETE FROM clusters WHERE user_id = ?`, userID); err != nil {
		return fmt.Errorf("delete clusters: %w", err)
	}
	if _, err := tx.Exec(`UPDATE users SET interest_vector = NULL, updated_at = CURRENT_TIMESTAMP WHERE id = ?`, userID); err != nil {
		return fmt.Errorf("clear user interest vector: %w", err)
	}
	if _, err := tx.Exec(`DELETE FROM user_interest_embeddings WHERE user_id = ?`, userID); err != nil {
		return fmt.Errorf("delete user interest embedding: %w", err)
	}
	if _, err := tx.Exec(`DELETE FROM ai_article_stage_skips WHERE user_id = ?`, userID); err != nil {
		return fmt.Errorf("delete article stage skips: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit reset renormalization transaction: %w", err)
	}
	return nil
}

func (db *DB) NormalizeArticleEmbeddingsForUser(userID int64) (int, int, error) {
	db.WaitForReady()

	rows, err := db.Query(
		`SELECT ae.article_id, ae.title_embedding, ae.summary_embedding
		   FROM article_embeddings ae
		   JOIN articles a ON a.id = ae.article_id
		  WHERE a.user_id = ?
		  ORDER BY ae.article_id ASC`,
		userID,
	)
	if err != nil {
		return 0, 0, fmt.Errorf("query article embeddings for renormalization: %w", err)
	}
	defer rows.Close()

	type rowData struct {
		articleID int64
		titleBlob []byte
		sumBlob   []byte
	}
	allRows := make([]rowData, 0)
	for rows.Next() {
		var row rowData
		if err := rows.Scan(&row.articleID, &row.titleBlob, &row.sumBlob); err != nil {
			return 0, 0, fmt.Errorf("scan article embedding row: %w", err)
		}
		allRows = append(allRows, row)
	}
	if err := rows.Err(); err != nil {
		return 0, 0, fmt.Errorf("iterate article embedding rows: %w", err)
	}

	tx, err := db.Begin()
	if err != nil {
		return 0, 0, fmt.Errorf("begin normalize article embeddings transaction: %w", err)
	}
	defer tx.Rollback()

	normalizedCount := 0
	clearedCount := 0
	for _, row := range allRows {
		titleVec, titleErr := interest.DeserializeVector(row.titleBlob)
		sumVec, sumErr := interest.DeserializeVector(row.sumBlob)
		if titleErr != nil || sumErr != nil || len(titleVec) == 0 || len(sumVec) == 0 {
			if _, err := tx.Exec(`DELETE FROM article_embeddings WHERE article_id = ?`, row.articleID); err != nil {
				return 0, 0, fmt.Errorf("delete invalid article embedding %d: %w", row.articleID, err)
			}
			clearedCount++
			continue
		}

		titleBlob, err := interest.NormalizeAndSerialize(titleVec)
		if err != nil {
			if _, execErr := tx.Exec(`DELETE FROM article_embeddings WHERE article_id = ?`, row.articleID); execErr != nil {
				return 0, 0, fmt.Errorf("delete article embedding after title renormalization failure %d: %w", row.articleID, execErr)
			}
			clearedCount++
			continue
		}
		sumBlob, err := interest.NormalizeAndSerialize(sumVec)
		if err != nil {
			if _, execErr := tx.Exec(`DELETE FROM article_embeddings WHERE article_id = ?`, row.articleID); execErr != nil {
				return 0, 0, fmt.Errorf("delete article embedding after summary renormalization failure %d: %w", row.articleID, execErr)
			}
			clearedCount++
			continue
		}

		if _, err := tx.Exec(`DELETE FROM article_embeddings WHERE article_id = ?`, row.articleID); err != nil {
			return 0, 0, fmt.Errorf("delete existing article embedding %d before reinserting normalized data: %w", row.articleID, err)
		}
		if _, err := tx.Exec(
			`INSERT INTO article_embeddings (article_id, title_embedding, summary_embedding)
			 VALUES (?, ?, ?)`,
			row.articleID, titleBlob, sumBlob,
		); err != nil {
			return 0, 0, fmt.Errorf("update normalized article embedding %d: %w", row.articleID, err)
		}
		normalizedCount++
	}

	if err := tx.Commit(); err != nil {
		return 0, 0, fmt.Errorf("commit normalize article embeddings transaction: %w", err)
	}

	return normalizedCount, clearedCount, nil
}

func (db *DB) GetArticlesForAIReclusterNormalization(userID int64, targetLang string) ([]AIBatchProcessingArticle, error) {
	db.WaitForReady()
	if targetLang == "" {
		targetLang = "zh"
	}

	rows, err := db.Query(
		`SELECT a.id, a.feed_id, COALESCE(a.title, ''), COALESCE(a.url, ''), COALESCE(a.image_url, ''), COALESCE(a.audio_url, ''), COALESCE(a.video_url, ''),
		        a.published_at, a.is_read, a.is_favorite, a.is_hidden, a.is_read_later,
		        COALESCE(a.translated_title, ''), COALESCE(a.summary, ''), COALESCE(a.freshrss_item_id, ''), COALESCE(f.title, ''), COALESCE(a.author, ''),
		        a.cluster_id,
		        COALESCE(f.translate_articles, 0),
		        ac.article_id IS NOT NULL,
		        (TRIM(COALESCE(a.summary, '')) <> '' AND COALESCE(a.summary, '') <> '<no content>'),
		        atc.article_id IS NOT NULL,
		        ae.article_id IS NOT NULL
		   FROM articles a
		   LEFT JOIN feeds f ON a.feed_id = f.id
		   LEFT JOIN article_contents ac ON ac.article_id = a.id
		   LEFT JOIN article_translated_contents atc ON atc.article_id = a.id AND atc.target_lang = ?
		   LEFT JOIN article_embeddings ae ON ae.article_id = a.id
		  WHERE a.user_id = ?
		  ORDER BY a.published_at ASC, a.id ASC`,
		targetLang,
		userID,
	)
	if err != nil {
		return nil, fmt.Errorf("query articles for recluster normalization: %w", err)
	}
	defer rows.Close()

	articles := make([]AIBatchProcessingArticle, 0)
	for rows.Next() {
		var article AIBatchProcessingArticle
		var clusterID sql.NullInt64
		if err := rows.Scan(
			&article.Article.ID, &article.Article.FeedID, &article.Article.Title, &article.Article.URL, &article.Article.ImageURL, &article.Article.AudioURL, &article.Article.VideoURL,
			&article.Article.PublishedAt, &article.Article.IsRead, &article.Article.IsFavorite, &article.Article.IsHidden, &article.Article.IsReadLater,
			&article.Article.TranslatedTitle, &article.Article.Summary, &article.Article.FreshRSSItemID, &article.Article.FeedTitle, &article.Article.Author,
			&clusterID,
			&article.TranslateArticles,
			&article.HasContent,
			&article.HasSummary,
			&article.HasTranslation,
			&article.HasArticleEmbedding,
		); err != nil {
			return nil, fmt.Errorf("scan article for recluster normalization: %w", err)
		}
		if clusterID.Valid {
			article.Article.ClusterID = clusterID.Int64
		}
		article.HasCluster = false
		article.ClusterNeedsPostProcess = false
		article.ClusterNeedsEmbeddingRepair = false
		articles = append(articles, article)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate articles for recluster normalization: %w", err)
	}
	return articles, nil
}
