package sqlite

import (
	"database/sql"

	"MavenRSS/internal/utils/textutil"
)

// articleContents are re-delivered by feed polls with an identical body, so the
// common case is a no-op. The previous implementation used "INSERT OR REPLACE"
// for every poll: SQLite implements that as DELETE + INSERT, which allocates a
// fresh AUTOINCREMENT id each time, frees and reallocates pages and pushes
// sqlite_sequence into the hundreds of millions for a table holding a few
// hundred thousand rows.
//
// Writes now go through an explicit read-then-write decision:
//   - identical content and a recent row: no statement is executed at all;
//   - existing row: UPDATE in place, keeping its id;
//   - missing row: INSERT (ON CONFLICT still covers a concurrent insert).
const (
	articleContentInsertSQL = `
		INSERT INTO article_contents (article_id, content, fetched_at)
		VALUES (?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(article_id) DO UPDATE SET
			content = excluded.content,
			fetched_at = CURRENT_TIMESTAMP`

	articleContentUpdateSQL = `
		UPDATE article_contents
		SET content = ?, fetched_at = CURRENT_TIMESTAMP
		WHERE article_id = ?`
)

// articleContentState describes the cached row for an article relative to the
// content that is about to be stored.
type articleContentState struct {
	exists bool
	same   bool
	fresh  bool // fetched_at is less than a day old
}

func (db *DB) getArticleContentState(articleID int64, content string) articleContentState {
	var stored string
	var fresh bool
	err := db.QueryRow(`
		SELECT content, fetched_at >= datetime('now', '-1 day')
		FROM article_contents
		WHERE article_id = ?`, articleID).Scan(&stored, &fresh)
	if err != nil {
		// sql.ErrNoRows (nothing cached yet) or an unparsable legacy
		// timestamp: treat as "needs a write", which is always correct.
		return articleContentState{}
	}
	return articleContentState{exists: true, same: stored == content, fresh: fresh}
}

// storeArticleContent writes content through exec, skipping the statement when
// the cached copy is already identical and recent. fetched_at therefore means
// "last time this content was confirmed", refreshed at most once a day for
// unchanged articles instead of on every poll.
func (db *DB) storeArticleContent(exec func(query string, args ...any) (sql.Result, error), articleID int64, content string) error {
	state := db.getArticleContentState(articleID, content)
	if state.exists {
		if state.same && state.fresh {
			return nil
		}
		_, err := exec(articleContentUpdateSQL, content, articleID)
		return err
	}
	_, err := exec(articleContentInsertSQL, articleID, content)
	return err
}

// ArticleContent represents a cached article content entry
type ArticleContent struct {
	ID        int64
	ArticleID int64
	Content   string
	FetchedAt string
}

// GetArticleContent retrieves cached content for an article
func (db *DB) GetArticleContent(articleID int64) (string, bool, error) {
	db.WaitForReady()
	var content string
	err := db.QueryRow(
		`SELECT content FROM article_contents WHERE article_id = ?`,
		articleID,
	).Scan(&content)

	if err == sql.ErrNoRows {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	content = textutil.NormalizeArticleContent(content)
	return content, true, nil
}

// SetArticleContent stores or updates content for an article
func (db *DB) SetArticleContent(articleID int64, content string) error {
	db.WaitForReady()
	content = textutil.NormalizeArticleContent(content)
	return db.storeArticleContent(db.Exec, articleID, content)
}

// SetArticleContentBackground is SetArticleContent at background write
// priority; it yields to waiting interactive writes.
func (db *DB) SetArticleContentBackground(articleID int64, content string) error {
	db.WaitForReady()
	content = textutil.NormalizeArticleContent(content)
	exec := func(query string, args ...any) (sql.Result, error) {
		return db.execWithPriority(writePriorityBackground, query, args...)
	}
	return db.storeArticleContent(exec, articleID, content)
}

// DeleteArticleContent removes cached content for an article
func (db *DB) DeleteArticleContent(articleID int64) error {
	db.WaitForReady()
	_, err := db.Exec(
		`DELETE FROM article_contents WHERE article_id = ?`,
		articleID,
	)
	return err
}

// CleanupOldArticleContents removes article content cache entries older than maxAgeDays
// If userID > 0, only clean up for that user; otherwise clean up for all users
func (db *DB) CleanupOldArticleContents(maxAgeDays int, userID int64) (int64, error) {
	db.WaitForReady()
	var result sql.Result
	var err error
	if userID > 0 {
		result, err = db.Exec(`
			DELETE FROM article_contents 
			WHERE fetched_at < datetime('now', '-' || ? || ' days')
			AND article_id IN (SELECT id FROM articles WHERE user_id = ?)
		`, maxAgeDays, userID)
	} else {
		result, err = db.Exec(
			`DELETE FROM article_contents WHERE fetched_at < datetime('now', '-' || ? || ' days')`,
			maxAgeDays,
		)
	}
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

// GetArticleContentCount returns the total number of cached article content entries for a specific user
func (db *DB) GetArticleContentCount(userID int64) (int64, error) {
	db.WaitForReady()
	var count int64
	err := db.QueryRow(`
		SELECT COUNT(*) FROM article_contents 
		WHERE article_id IN (SELECT id FROM articles WHERE user_id = ?)
	`, userID).Scan(&count)
	if err != nil {
		return 0, err
	}
	return count, nil
}
