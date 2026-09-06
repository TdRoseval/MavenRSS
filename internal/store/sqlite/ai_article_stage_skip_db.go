package sqlite

import "database/sql"

// SetAIArticleStageSkip stores an article-stage skip marker so background AI
// recovery can stop retrying content that is known to be non-recoverable.
// The marker is only written by the background AI pipeline, so it runs at
// background write priority.
func (db *DB) SetAIArticleStageSkip(userID, articleID int64, stage, reason string) error {
	db.WaitForReady()
	_, err := db.execWithPriority(
		writePriorityBackground,
		`INSERT INTO ai_article_stage_skips (user_id, article_id, stage, reason, created_at, updated_at)
		 VALUES (?, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
		 ON CONFLICT(article_id, stage) DO UPDATE SET
			user_id = excluded.user_id,
			reason = excluded.reason,
			updated_at = CURRENT_TIMESTAMP`,
		userID, articleID, stage, reason,
	)
	return err
}

// DeleteAIArticleStageSkip removes a skip marker after successful processing.
// The marker is only written by the background AI pipeline, so it runs at
// background write priority.
func (db *DB) DeleteAIArticleStageSkip(articleID int64, stage string) error {
	db.WaitForReady()
	_, err := db.execWithPriority(
		writePriorityBackground,
		`DELETE FROM ai_article_stage_skips WHERE article_id = ? AND stage = ?`,
		articleID, stage,
	)
	return err
}

// GetAIArticleStageSkipReason retrieves the stored skip reason for an article stage.
func (db *DB) GetAIArticleStageSkipReason(articleID int64, stage string) (string, bool, error) {
	db.WaitForReady()

	var reason string
	err := db.QueryRow(
		`SELECT reason FROM ai_article_stage_skips WHERE article_id = ? AND stage = ?`,
		articleID, stage,
	).Scan(&reason)
	if err == sql.ErrNoRows {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return reason, true, nil
}
