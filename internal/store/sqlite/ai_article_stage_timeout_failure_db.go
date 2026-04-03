package sqlite

import (
	"database/sql"
	"fmt"
	"time"
)

const sqliteTimestampLayout = "2006-01-02 15:04:05"

type AIArticleStageTimeoutFailure struct {
	ArticleID     int64
	Stage         string
	TimeoutCount  int
	LastReason    string
	FirstFailedAt time.Time
	LastFailedAt  time.Time
}

func (db *DB) RecordAIArticleStageTimeoutFailure(
	userID, articleID int64,
	stage, reason string,
) (AIArticleStageTimeoutFailure, error) {
	db.WaitForReady()

	if _, err := db.Exec(
		`INSERT INTO ai_article_stage_timeout_failures (
			user_id, article_id, stage, timeout_count, last_reason, first_failed_at, last_failed_at
		)
		 VALUES (?, ?, ?, 1, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
		 ON CONFLICT(article_id, stage) DO UPDATE SET
			user_id = excluded.user_id,
			timeout_count = ai_article_stage_timeout_failures.timeout_count + 1,
			last_reason = excluded.last_reason,
			last_failed_at = CURRENT_TIMESTAMP`,
		userID, articleID, stage, reason,
	); err != nil {
		return AIArticleStageTimeoutFailure{}, err
	}

	state, found, err := db.GetAIArticleStageTimeoutFailure(articleID, stage)
	if err != nil {
		return AIArticleStageTimeoutFailure{}, err
	}
	if !found {
		return AIArticleStageTimeoutFailure{}, fmt.Errorf("timeout failure state missing after upsert")
	}
	return state, nil
}

func (db *DB) DeleteAIArticleStageTimeoutFailure(articleID int64, stage string) error {
	db.WaitForReady()
	_, err := db.Exec(
		`DELETE FROM ai_article_stage_timeout_failures WHERE article_id = ? AND stage = ?`,
		articleID, stage,
	)
	return err
}

func (db *DB) GetAIArticleStageTimeoutFailure(
	articleID int64,
	stage string,
) (AIArticleStageTimeoutFailure, bool, error) {
	db.WaitForReady()

	var state AIArticleStageTimeoutFailure
	var firstFailedAt string
	var lastFailedAt string
	err := db.QueryRow(
		`SELECT article_id, stage, timeout_count, last_reason, first_failed_at, last_failed_at
		   FROM ai_article_stage_timeout_failures
		  WHERE article_id = ? AND stage = ?`,
		articleID, stage,
	).Scan(
		&state.ArticleID,
		&state.Stage,
		&state.TimeoutCount,
		&state.LastReason,
		&firstFailedAt,
		&lastFailedAt,
	)
	if err == sql.ErrNoRows {
		return AIArticleStageTimeoutFailure{}, false, nil
	}
	if err != nil {
		return AIArticleStageTimeoutFailure{}, false, err
	}

	state.FirstFailedAt = parseSQLiteTimestamp(firstFailedAt)
	state.LastFailedAt = parseSQLiteTimestamp(lastFailedAt)
	return state, true, nil
}

func parseSQLiteTimestamp(raw string) time.Time {
	if raw == "" {
		return time.Time{}
	}

	ts, err := time.ParseInLocation(sqliteTimestampLayout, raw, time.UTC)
	if err != nil {
		return time.Time{}
	}
	return ts
}
