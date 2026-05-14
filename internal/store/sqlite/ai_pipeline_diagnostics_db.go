package sqlite

type AIStageCount struct {
	Stage string `json:"stage"`
	Count int    `json:"count"`
}

type AIStageTimeoutFailureSummary struct {
	Stage           string `json:"stage"`
	Count           int    `json:"count"`
	MaxTimeoutCount int    `json:"max_timeout_count"`
	FirstFailedAt   string `json:"first_failed_at,omitempty"`
	LastFailedAt    string `json:"last_failed_at,omitempty"`
}

type ClusterStatusCount struct {
	Status string `json:"status"`
	Count  int    `json:"count"`
}

type AIPipelineBlockingArticleSample struct {
	ArticleID     int64  `json:"article_id"`
	FeedID        int64  `json:"feed_id"`
	Title         string `json:"title"`
	BlockingStage string `json:"blocking_stage"`
}

func (db *DB) GetAIArticleStageSkipCounts(userID int64) ([]AIStageCount, error) {
	db.WaitForReady()

	rows, err := db.Query(
		`SELECT stage, COUNT(*)
		   FROM ai_article_stage_skips
		  WHERE user_id = ?
		  GROUP BY stage
		  ORDER BY stage`,
		userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	counts := make([]AIStageCount, 0)
	for rows.Next() {
		var count AIStageCount
		if err := rows.Scan(&count.Stage, &count.Count); err != nil {
			return nil, err
		}
		counts = append(counts, count)
	}
	return counts, rows.Err()
}

func (db *DB) GetAIArticleStageTimeoutFailureSummaries(userID int64) ([]AIStageTimeoutFailureSummary, error) {
	db.WaitForReady()

	rows, err := db.Query(
		`SELECT stage,
		        COUNT(*),
		        COALESCE(MAX(timeout_count), 0),
		        COALESCE(MIN(first_failed_at), ''),
		        COALESCE(MAX(last_failed_at), '')
		   FROM ai_article_stage_timeout_failures
		  WHERE user_id = ?
		  GROUP BY stage
		  ORDER BY stage`,
		userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	summaries := make([]AIStageTimeoutFailureSummary, 0)
	for rows.Next() {
		var summary AIStageTimeoutFailureSummary
		if err := rows.Scan(
			&summary.Stage,
			&summary.Count,
			&summary.MaxTimeoutCount,
			&summary.FirstFailedAt,
			&summary.LastFailedAt,
		); err != nil {
			return nil, err
		}
		summaries = append(summaries, summary)
	}
	return summaries, rows.Err()
}

func (db *DB) GetClusterStatusCounts(userID int64) ([]ClusterStatusCount, error) {
	db.WaitForReady()

	rows, err := db.Query(
		`SELECT status, COUNT(*)
		   FROM clusters
		  WHERE user_id = ?
		  GROUP BY status
		  ORDER BY status`,
		userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	counts := make([]ClusterStatusCount, 0)
	for rows.Next() {
		var count ClusterStatusCount
		if err := rows.Scan(&count.Status, &count.Count); err != nil {
			return nil, err
		}
		counts = append(counts, count)
	}
	return counts, rows.Err()
}

func (db *DB) GetAIPipelineBlockingArticleSamples(
	userID int64,
	targetLang string,
	limit int,
) ([]AIPipelineBlockingArticleSample, error) {
	db.WaitForReady()

	if targetLang == "" {
		targetLang = "zh"
	}
	if limit <= 0 {
		limit = 5
	}

	rows, err := db.Query(
		`
		WITH eligible_articles AS (
			SELECT
				a.id,
				a.feed_id,
				COALESCE(a.title, '') AS title,
				((TRIM(COALESCE(a.summary, '')) <> '' AND COALESCE(a.summary, '') <> '<no content>') OR skip_summary.article_id IS NOT NULL) AS has_summary,
				(COALESCE(f.translate_articles, 0) = 1) AS translate_articles,
				CASE
					WHEN COALESCE(f.translate_articles, 0) = 0 THEN 1
					WHEN skip_translation.article_id IS NOT NULL THEN 1
					WHEN atc.article_id IS NOT NULL AND TRIM(COALESCE(a.translated_title, '')) <> '' THEN 1
					ELSE 0
				END AS has_translation,
				(ae.article_id IS NOT NULL OR skip_embedding.article_id IS NOT NULL) AS has_embedding,
				(c.id IS NOT NULL OR skip_clustering.article_id IS NOT NULL) AS has_cluster
			FROM articles a
			LEFT JOIN feeds f ON a.feed_id = f.id
			LEFT JOIN article_translated_contents atc ON atc.article_id = a.id AND atc.target_lang = ?
			LEFT JOIN ai_article_stage_skips skip_summary ON skip_summary.article_id = a.id AND skip_summary.stage = 'summary'
			LEFT JOIN ai_article_stage_skips skip_translation ON skip_translation.article_id = a.id AND skip_translation.stage = 'translation'
			LEFT JOIN ai_article_stage_skips skip_embedding ON skip_embedding.article_id = a.id AND skip_embedding.stage = 'embedding'
			LEFT JOIN ai_article_stage_skips skip_clustering ON skip_clustering.article_id = a.id AND skip_clustering.stage = 'clustering'
			LEFT JOIN article_embeddings ae ON ae.article_id = a.id
			LEFT JOIN clusters c ON a.cluster_id = c.id
			WHERE a.user_id = ?
			  AND (
				a.is_favorite = 1
				OR (a.is_favorite = 0 AND a.published_at >= datetime('now', '-2 days'))
			  )
		),
		blocking_articles AS (
			SELECT
				id,
				feed_id,
				title,
				CASE
					WHEN NOT has_summary THEN 'summary'
					WHEN translate_articles AND NOT has_translation THEN 'translation'
					WHEN NOT has_embedding THEN 'embedding'
					WHEN NOT has_cluster THEN 'clustering'
					ELSE 'complete'
				END AS blocking_stage
			FROM eligible_articles
		)
		SELECT id, feed_id, title, blocking_stage
		  FROM blocking_articles
		 WHERE blocking_stage <> 'complete'
		 ORDER BY id
		 LIMIT ?
		`,
		targetLang,
		userID,
		limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	samples := make([]AIPipelineBlockingArticleSample, 0, limit)
	for rows.Next() {
		var sample AIPipelineBlockingArticleSample
		if err := rows.Scan(&sample.ArticleID, &sample.FeedID, &sample.Title, &sample.BlockingStage); err != nil {
			return nil, err
		}
		samples = append(samples, sample)
	}
	return samples, rows.Err()
}
