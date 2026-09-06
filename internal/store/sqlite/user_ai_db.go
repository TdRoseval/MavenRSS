package sqlite

import (
	"context"
	"database/sql"
)

// GetUserAIReadStats retrieves the AI reading statistics for a user.
func (db *DB) GetUserAIReadStats(userID int64) (readCount, totalReadTime int64, err error) {
	db.WaitForReady()
	err = db.QueryRow(
		`SELECT COALESCE(ai_read_count, 0), COALESCE(ai_total_read_time, 0) FROM users WHERE id = ?`,
		userID,
	).Scan(&readCount, &totalReadTime)
	return
}

// UpdateUserAIReadStats updates the AI reading statistics for a user.
func (db *DB) UpdateUserAIReadStats(userID int64, readCount, totalReadTime int64) error {
	db.WaitForReady()
	_, err := db.Exec(
		`UPDATE users SET ai_read_count = ?, ai_total_read_time = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`,
		readCount, totalReadTime, userID,
	)
	return err
}

// GetUserInterestVector retrieves the raw interest vector blob for a user.
func (db *DB) GetUserInterestVector(userID int64) ([]byte, error) {
	db.WaitForReady()
	var vec []byte
	err := db.QueryRow(
		`SELECT interest_vector FROM users WHERE id = ?`,
		userID,
	).Scan(&vec)
	if err != nil {
		return nil, err
	}
	return vec, nil
}

// UpdateUserInterestVector persists the interest vector blob to the users table
// and keeps the user_interest_embeddings vec table in sync.
func (db *DB) UpdateUserInterestVector(userID int64, vectorBlob []byte) error {
	return db.updateUserInterestVector(writePriorityInteractive, userID, vectorBlob)
}

// UpdateUserInterestVectorBackground is UpdateUserInterestVector at background
// write priority; it yields to waiting interactive writes.
func (db *DB) UpdateUserInterestVectorBackground(userID int64, vectorBlob []byte) error {
	return db.updateUserInterestVector(writePriorityBackground, userID, vectorBlob)
}

func (db *DB) updateUserInterestVector(priority writePriority, userID int64, vectorBlob []byte) error {
	db.WaitForReady()
	err := db.withWriteTxPriority(context.Background(), priority, func(tx *sql.Tx) error {
		if _, err := tx.Exec(
			`UPDATE users SET interest_vector = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`,
			vectorBlob, userID,
		); err != nil {
			return err
		}

		if _, err := tx.Exec(`DELETE FROM user_interest_embeddings WHERE user_id = ?`, userID); err != nil {
			return err
		}

		if len(vectorBlob) > 0 {
			if _, err := tx.Exec(
				`INSERT INTO user_interest_embeddings (user_id, interest_embedding) VALUES (?, ?)`,
				userID, vectorBlob,
			); err != nil {
				return err
			}
		}

		if _, err := tx.Exec(`DELETE FROM cluster_feed_first_page_cache WHERE user_id = ?`, userID); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return err
	}
	db.clearClusterFeedFirstPageMemoryCache(userID)
	return nil
}

// GetFavoriteClusterSummaryEmbeddings retrieves summary embedding blobs for all
// favorited clusters of a user (used for cold-start interest vector initialization).
func (db *DB) GetFavoriteClusterSummaryEmbeddings(userID int64) ([][]byte, error) {
	db.WaitForReady()
	rows, err := db.Query(`
		SELECT ce.summary_embedding
		FROM cluster_embeddings ce
		JOIN clusters c ON ce.cluster_id = c.id
		WHERE c.user_id = ? AND c.is_favorite = 1 AND ce.summary_embedding IS NOT NULL
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var blobs [][]byte
	for rows.Next() {
		var blob []byte
		if err := rows.Scan(&blob); err != nil {
			continue
		}
		if len(blob) > 0 {
			blobs = append(blobs, blob)
		}
	}
	return blobs, nil
}
