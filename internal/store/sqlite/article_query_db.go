package sqlite

import (
	"database/sql"
	"log"

	"MavenRSS/internal/models"
)

type AIBatchProcessingArticle struct {
	Article             models.Article
	TranslateArticles   bool
	HasContent          bool
	HasSummary          bool
	HasTranslation      bool
	HasArticleEmbedding bool
	HasCluster          bool
	ClusterComplete     bool
}

type AIProcessingProgress struct {
	EligibleArticles           int `json:"eligible_articles"`
	PendingArticles            int `json:"pending_articles"`
	CompletedArticles          int `json:"completed_articles"`
	PendingSummaryArticles     int `json:"pending_summary_articles"`
	PendingTranslationArticles int `json:"pending_translation_articles"`
	PendingEmbeddingArticles   int `json:"pending_embedding_articles"`
	PendingClusteringArticles  int `json:"pending_clustering_articles"`
}

func (db *DB) GetArticlesForAIBatchProcessing(userID int64, targetLang string) ([]AIBatchProcessingArticle, error) {
	db.WaitForReady()
	if targetLang == "" {
		targetLang = "zh"
	}

	query := `
		SELECT a.id, a.feed_id, COALESCE(a.title, ''), COALESCE(a.url, ''), COALESCE(a.image_url, ''), COALESCE(a.audio_url, ''), COALESCE(a.video_url, ''),
			a.published_at, a.is_read, a.is_favorite, a.is_hidden, a.is_read_later,
			COALESCE(a.translated_title, ''), COALESCE(a.summary, ''), COALESCE(a.freshrss_item_id, ''), COALESCE(f.title, ''), COALESCE(a.author, ''),
			a.cluster_id,
			COALESCE(f.translate_articles, 0),
			ac.article_id IS NOT NULL,
			((TRIM(COALESCE(a.summary, '')) <> '' AND COALESCE(a.summary, '') <> '<no content>') OR skip_summary.article_id IS NOT NULL),
			(atc.article_id IS NOT NULL OR skip_translation.article_id IS NOT NULL),
			ae.article_id IS NOT NULL,
			c.id IS NOT NULL,
			(
				c.id IS NOT NULL
				AND c.status = 'complete'
				AND (
					ce.cluster_id IS NOT NULL
					OR (
						TRIM(COALESCE(c.merged_title, '')) = ''
						AND TRIM(COALESCE(c.merged_summary, '')) = ''
					)
				)
			)
		FROM articles a
		LEFT JOIN feeds f ON a.feed_id = f.id
		LEFT JOIN article_contents ac ON ac.article_id = a.id
		LEFT JOIN article_translated_contents atc ON atc.article_id = a.id AND atc.target_lang = ?
		LEFT JOIN ai_article_stage_skips skip_summary ON skip_summary.article_id = a.id AND skip_summary.stage = 'summary'
		LEFT JOIN ai_article_stage_skips skip_translation ON skip_translation.article_id = a.id AND skip_translation.stage = 'translation'
		LEFT JOIN article_embeddings ae ON ae.article_id = a.id
		LEFT JOIN clusters c ON a.cluster_id = c.id
		LEFT JOIN cluster_embeddings ce ON ce.cluster_id = c.id
		WHERE a.user_id = ?
		AND (
			a.is_favorite = 1
			OR (a.is_favorite = 0 AND a.published_at >= datetime('now', '-2 days'))
		)
		AND (
			a.is_favorite = 1
			OR (
				((TRIM(COALESCE(a.summary, '')) = '' OR COALESCE(a.summary, '') = '<no content>') AND skip_summary.article_id IS NULL)
				OR (COALESCE(f.translate_articles, 0) = 1 AND atc.article_id IS NULL AND skip_translation.article_id IS NULL)
				OR ae.article_id IS NULL
				OR c.id IS NULL
				OR NOT (
					c.status = 'complete'
					AND (
						ce.cluster_id IS NOT NULL
						OR (
							TRIM(COALESCE(c.merged_title, '')) = ''
							AND TRIM(COALESCE(c.merged_summary, '')) = ''
						)
					)
				)
			)
		)
		ORDER BY a.published_at DESC
	`

	rows, err := db.Query(query, targetLang, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var articles []AIBatchProcessingArticle
	for rows.Next() {
		var article AIBatchProcessingArticle
		var clusterID sql.NullInt64
		err := rows.Scan(
			&article.Article.ID, &article.Article.FeedID, &article.Article.Title, &article.Article.URL, &article.Article.ImageURL, &article.Article.AudioURL, &article.Article.VideoURL,
			&article.Article.PublishedAt, &article.Article.IsRead, &article.Article.IsFavorite, &article.Article.IsHidden, &article.Article.IsReadLater,
			&article.Article.TranslatedTitle, &article.Article.Summary, &article.Article.FreshRSSItemID, &article.Article.FeedTitle, &article.Article.Author,
			&clusterID,
			&article.TranslateArticles,
			&article.HasContent,
			&article.HasSummary,
			&article.HasTranslation,
			&article.HasArticleEmbedding,
			&article.HasCluster,
			&article.ClusterComplete,
		)
		if err != nil {
			log.Printf("Error scanning article for AI batch: %v", err)
			continue
		}
		if clusterID.Valid {
			article.Article.ClusterID = clusterID.Int64
		}
		articles = append(articles, article)
	}

	return articles, rows.Err()
}

func (db *DB) GetAIProcessingProgress(userID int64, targetLang string) (AIProcessingProgress, error) {
	db.WaitForReady()

	progress := AIProcessingProgress{}
	if targetLang == "" {
		targetLang = "zh"
	}

	const articleProgressQuery = `
		WITH eligible_articles AS (
			SELECT
				((TRIM(COALESCE(a.summary, '')) <> '' AND COALESCE(a.summary, '') <> '<no content>') OR skip_summary.article_id IS NOT NULL) AS has_summary,
				(COALESCE(f.translate_articles, 0) = 1) AS translate_articles,
				(atc.article_id IS NOT NULL OR skip_translation.article_id IS NOT NULL) AS has_translation,
				(ae.article_id IS NOT NULL) AS has_embedding,
				(c.id IS NOT NULL) AS has_cluster,
				(
					c.id IS NOT NULL
					AND c.status = 'complete'
					AND (
						ce.cluster_id IS NOT NULL
						OR (
							TRIM(COALESCE(c.merged_title, '')) = ''
							AND TRIM(COALESCE(c.merged_summary, '')) = ''
						)
					)
				) AS cluster_complete
			FROM articles a
			LEFT JOIN feeds f ON a.feed_id = f.id
			LEFT JOIN article_contents ac ON ac.article_id = a.id
			LEFT JOIN article_translated_contents atc ON atc.article_id = a.id AND atc.target_lang = ?
			LEFT JOIN ai_article_stage_skips skip_summary ON skip_summary.article_id = a.id AND skip_summary.stage = 'summary'
			LEFT JOIN ai_article_stage_skips skip_translation ON skip_translation.article_id = a.id AND skip_translation.stage = 'translation'
			LEFT JOIN article_embeddings ae ON ae.article_id = a.id
			LEFT JOIN clusters c ON a.cluster_id = c.id
			LEFT JOIN cluster_embeddings ce ON ce.cluster_id = c.id
			WHERE a.user_id = ?
			AND (
				a.is_favorite = 1
				OR (a.is_favorite = 0 AND a.published_at >= datetime('now', '-2 days'))
			)
		),
		stage_counts AS (
			SELECT
				CASE
					WHEN NOT has_summary THEN 'summary'
					WHEN translate_articles AND NOT has_translation THEN 'translation'
					WHEN NOT has_embedding THEN 'embedding'
					WHEN NOT has_cluster OR NOT cluster_complete THEN 'clustering'
					ELSE 'complete'
				END AS blocking_stage
			FROM eligible_articles
		)
		SELECT
			COUNT(*),
			COALESCE(SUM(CASE WHEN blocking_stage <> 'complete' THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN blocking_stage = 'summary' THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN blocking_stage = 'translation' THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN blocking_stage = 'embedding' THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN blocking_stage = 'clustering' THEN 1 ELSE 0 END), 0)
		FROM stage_counts
	`

	if err := db.QueryRow(articleProgressQuery, targetLang, userID).Scan(
		&progress.EligibleArticles,
		&progress.PendingArticles,
		&progress.PendingSummaryArticles,
		&progress.PendingTranslationArticles,
		&progress.PendingEmbeddingArticles,
		&progress.PendingClusteringArticles,
	); err != nil {
		return progress, err
	}

	progress.CompletedArticles = progress.EligibleArticles - progress.PendingArticles
	if progress.CompletedArticles < 0 {
		progress.CompletedArticles = 0
	}

	return progress, nil
}
