package sqlite

import (
	"database/sql"
	"fmt"
	"log"
	"strings"
	"time"

	"MavenRSS/internal/models"
)

type DailyRecommendationCandidate struct {
	Cluster     models.Cluster
	PublishedAt time.Time
	Distance    float64
}

type DailyRecommendationResult struct {
	Recommendation    models.DailyRecommendation
	Cluster           models.Cluster
	LatestPublishedAt time.Time
}

func (db *DB) CountDailyRecommendations(userID int64, recommendationDate string) (int, error) {
	db.WaitForReady()
	var count int
	err := db.QueryRow(
		`SELECT COUNT(*) FROM daily_recommendations WHERE user_id = ? AND recommendation_date = ?`,
		userID, recommendationDate,
	).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count daily recommendations: %w", err)
	}
	return count, nil
}

func (db *DB) HasDailyRecommendations(userID int64, recommendationDate string) (bool, error) {
	count, err := db.CountDailyRecommendations(userID, recommendationDate)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func (db *DB) CountDailyRecommendationReadyClusters(userID int64, dayStart, dayEnd time.Time) (int, error) {
	db.WaitForReady()
	var count int
	err := db.QueryRow(`
		SELECT COUNT(DISTINCT c.id)
		FROM clusters c
		JOIN articles a ON a.cluster_id = c.id
		WHERE c.user_id = ?
		  AND c.status = 'complete'
		  AND c.is_hidden = 0
		  AND a.published_at >= ?
		  AND a.published_at < ?
	`, userID, dayStart, dayEnd).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count ready recommendation clusters: %w", err)
	}
	return count, nil
}

func (db *DB) ListAIRecommendedClusterIDsExcludingDate(userID int64, recommendationDate string) ([]int64, error) {
	db.WaitForReady()
	rows, err := db.Query(`
		SELECT DISTINCT cluster_id
		FROM daily_recommendations
		WHERE user_id = ? AND recommendation_date <> ?
	`, userID, recommendationDate)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	ids := make([]int64, 0)
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			continue
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func (db *DB) ListDailyRecommendationDates(userID int64) ([]string, error) {
	db.WaitForReady()
	rows, err := db.Query(`
		SELECT recommendation_date
		FROM daily_recommendations
		WHERE user_id = ?
		GROUP BY recommendation_date
		ORDER BY recommendation_date DESC
	`, userID)
	if err != nil {
		return nil, fmt.Errorf("list daily recommendation dates: %w", err)
	}
	defer rows.Close()

	dates := make([]string, 0)
	for rows.Next() {
		var recommendationDate string
		if err := rows.Scan(&recommendationDate); err != nil {
			return nil, fmt.Errorf("scan daily recommendation date: %w", err)
		}
		dates = append(dates, recommendationDate)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate daily recommendation dates: %w", err)
	}

	return dates, nil
}

func (db *DB) GetDailyRecommendationsByDate(userID int64, recommendationDate string) ([]DailyRecommendationResult, error) {
	db.WaitForReady()
	rows, err := db.Query(`
		SELECT
			dr.id,
			dr.user_id,
			dr.cluster_id,
			dr.recommendation_date,
			dr.recommendation_score,
			dr.recommendation_rank,
			dr.recommendation_profile_id,
			dr.created_at,
			c.id,
			c.user_id,
			c.status,
			c.merged_title,
			c.merged_summary,
			c.merged_content,
			c.recommendation_archive_date,
			c.recommendation_score,
			c.is_ai_recommended,
			c.recommendation_profile_id,
			c.article_count,
			c.created_at,
			c.updated_at,
			c.is_read,
			c.is_favorite,
			c.is_read_later,
			c.is_hidden,
			MAX(a.published_at) AS latest_published_at
		FROM daily_recommendations dr
		JOIN clusters c ON c.id = dr.cluster_id AND c.user_id = dr.user_id
		LEFT JOIN articles a ON a.cluster_id = c.id
		WHERE dr.user_id = ? AND dr.recommendation_date = ?
		GROUP BY
			dr.id,
			dr.user_id,
			dr.cluster_id,
			dr.recommendation_date,
			dr.recommendation_score,
			dr.recommendation_rank,
			dr.recommendation_profile_id,
			dr.created_at,
			c.id,
			c.user_id,
			c.status,
			c.merged_title,
			c.merged_summary,
			c.merged_content,
			c.recommendation_archive_date,
			c.recommendation_score,
			c.is_ai_recommended,
			c.recommendation_profile_id,
			c.article_count,
			c.created_at,
			c.updated_at,
			c.is_read,
			c.is_favorite,
			c.is_read_later,
			c.is_hidden
		ORDER BY dr.recommendation_rank ASC, dr.recommendation_score DESC, latest_published_at DESC
	`, userID, recommendationDate)
	if err != nil {
		return nil, fmt.Errorf("query daily recommendations by date: %w", err)
	}
	defer rows.Close()

	results := make([]DailyRecommendationResult, 0)
	for rows.Next() {
		var item DailyRecommendationResult
		var latestPublishedAt sql.NullString
		if err := rows.Scan(
			&item.Recommendation.ID,
			&item.Recommendation.UserID,
			&item.Recommendation.ClusterID,
			&item.Recommendation.RecommendationDate,
			&item.Recommendation.RecommendationScore,
			&item.Recommendation.RecommendationRank,
			&item.Recommendation.RecommendationProfileID,
			&item.Recommendation.CreatedAt,
			&item.Cluster.ID,
			&item.Cluster.UserID,
			&item.Cluster.Status,
			&item.Cluster.MergedTitle,
			&item.Cluster.MergedSummary,
			&item.Cluster.MergedContent,
			&item.Cluster.RecommendationArchiveDate,
			&item.Cluster.RecommendationScore,
			&item.Cluster.IsAIRecommended,
			&item.Cluster.RecommendationProfileID,
			&item.Cluster.ArticleCount,
			&item.Cluster.CreatedAt,
			&item.Cluster.UpdatedAt,
			&item.Cluster.IsRead,
			&item.Cluster.IsFavorite,
			&item.Cluster.IsReadLater,
			&item.Cluster.IsHidden,
			&latestPublishedAt,
		); err != nil {
			return nil, fmt.Errorf("scan daily recommendation by date: %w", err)
		}
		if parsedTime, ok := parseNullableSQLiteTime(latestPublishedAt); ok {
			item.LatestPublishedAt = parsedTime
		}
		db.populateClusterMeta(&item.Cluster)
		results = append(results, item)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate daily recommendations by date: %w", err)
	}

	return results, nil
}

func (db *DB) ListAIRecommendedClusterIDs(userID int64) ([]int64, error) {
	db.WaitForReady()
	rows, err := db.Query(`SELECT id FROM clusters WHERE user_id = ? AND is_ai_recommended = 1`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	ids := make([]int64, 0)
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			continue
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func (db *DB) GetDailyRecommendationCandidatesByVector(
	userID int64,
	dayStart time.Time,
	dayEnd time.Time,
	interestVecBlob []byte,
	excludeIDs []int64,
	limit int,
) ([]DailyRecommendationCandidate, error) {
	db.WaitForReady()
	if limit <= 0 {
		limit = 100
	}

	args := []interface{}{userID, dayStart, dayEnd}
	excludeClause := ""
	if len(excludeIDs) > 0 {
		placeholders := make([]string, len(excludeIDs))
		for i, id := range excludeIDs {
			placeholders[i] = "?"
			args = append(args, id)
		}
		excludeClause = fmt.Sprintf(" AND c.id NOT IN (%s)", strings.Join(placeholders, ","))
	}
	args = append(args, interestVecBlob, limit)

	query := fmt.Sprintf(`
		SELECT c.id, c.user_id, c.status, c.merged_title, c.merged_summary, c.merged_content,
			c.recommendation_archive_date, c.recommendation_score, c.is_ai_recommended, c.recommendation_profile_id,
			c.article_count, c.created_at, c.updated_at, c.is_read, c.is_favorite, c.is_read_later, c.is_hidden,
			MAX(a.published_at) AS published_at,
			ce.distance
		FROM cluster_embeddings ce
		JOIN clusters c ON ce.cluster_id = c.id
		JOIN articles a ON a.cluster_id = c.id
		WHERE c.user_id = ?
		  AND c.status = 'complete'
		  AND c.is_hidden = 0
		  AND a.published_at >= ?
		  AND a.published_at < ?%s
		  AND ce.summary_embedding MATCH ? AND k = ?
		GROUP BY c.id, c.user_id, c.status, c.merged_title, c.merged_summary, c.merged_content,
			c.recommendation_archive_date, c.recommendation_score, c.is_ai_recommended, c.recommendation_profile_id,
			c.article_count, c.created_at, c.updated_at, c.is_read, c.is_favorite, c.is_read_later, c.is_hidden,
			ce.distance
		ORDER BY ce.distance ASC
	`, excludeClause)

	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("query recommendation candidates by vector: %w", err)
	}
	defer rows.Close()

	return scanDailyRecommendationCandidates(db, rows, true)
}

func (db *DB) GetDailyRecommendationCandidatesChronological(
	userID int64,
	dayStart time.Time,
	dayEnd time.Time,
	excludeIDs []int64,
	limit int,
) ([]DailyRecommendationCandidate, error) {
	db.WaitForReady()
	if limit <= 0 {
		limit = 100
	}

	args := []interface{}{userID, dayStart, dayEnd}
	excludeClause := ""
	if len(excludeIDs) > 0 {
		placeholders := make([]string, len(excludeIDs))
		for i, id := range excludeIDs {
			placeholders[i] = "?"
			args = append(args, id)
		}
		excludeClause = fmt.Sprintf(" AND c.id NOT IN (%s)", strings.Join(placeholders, ","))
	}
	args = append(args, limit)

	query := fmt.Sprintf(`
		SELECT c.id, c.user_id, c.status, c.merged_title, c.merged_summary, c.merged_content,
			c.recommendation_archive_date, c.recommendation_score, c.is_ai_recommended, c.recommendation_profile_id,
			c.article_count, c.created_at, c.updated_at, c.is_read, c.is_favorite, c.is_read_later, c.is_hidden,
			MAX(a.published_at) AS published_at
		FROM clusters c
		JOIN articles a ON a.cluster_id = c.id
		WHERE c.user_id = ?
		  AND c.status = 'complete'
		  AND c.is_hidden = 0
		  AND a.published_at >= ?
		  AND a.published_at < ?%s
		GROUP BY c.id, c.user_id, c.status, c.merged_title, c.merged_summary, c.merged_content,
			c.recommendation_archive_date, c.recommendation_score, c.is_ai_recommended, c.recommendation_profile_id,
			c.article_count, c.created_at, c.updated_at, c.is_read, c.is_favorite, c.is_read_later, c.is_hidden
		ORDER BY published_at DESC
		LIMIT ?
	`, excludeClause)

	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("query recommendation candidates chronologically: %w", err)
	}
	defer rows.Close()

	return scanDailyRecommendationCandidates(db, rows, false)
}

func (db *DB) SaveDailyRecommendations(userID int64, recommendationDate string, recommendations []models.DailyRecommendation) error {
	db.WaitForReady()
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	rows, err := tx.Query(
		`SELECT cluster_id FROM daily_recommendations WHERE user_id = ? AND recommendation_date = ?`,
		userID, recommendationDate,
	)
	if err != nil {
		return err
	}
	previousIDs := make([]int64, 0)
	for rows.Next() {
		var clusterID int64
		if err := rows.Scan(&clusterID); err == nil {
			previousIDs = append(previousIDs, clusterID)
		}
	}
	rows.Close()

	if _, err := tx.Exec(
		`DELETE FROM daily_recommendations WHERE user_id = ? AND recommendation_date = ?`,
		userID, recommendationDate,
	); err != nil {
		return err
	}

	for _, clusterID := range previousIDs {
		var remaining int
		if err := tx.QueryRow(
			`SELECT COUNT(*) FROM daily_recommendations WHERE user_id = ? AND cluster_id = ?`,
			userID, clusterID,
		).Scan(&remaining); err != nil {
			return err
		}
		if remaining == 0 {
			if _, err := tx.Exec(
				`UPDATE clusters
				 SET recommendation_archive_date = '', recommendation_score = 0, recommendation_profile_id = 0, is_ai_recommended = 0, updated_at = CURRENT_TIMESTAMP
				 WHERE id = ? AND user_id = ?`,
				clusterID, userID,
			); err != nil {
				return err
			}
		}
	}

	for _, rec := range recommendations {
		if _, err := tx.Exec(
			`INSERT INTO daily_recommendations (user_id, cluster_id, recommendation_date, recommendation_score, recommendation_rank, recommendation_profile_id)
			 VALUES (?, ?, ?, ?, ?, ?)`,
			rec.UserID, rec.ClusterID, rec.RecommendationDate, rec.RecommendationScore, rec.RecommendationRank, rec.RecommendationProfileID,
		); err != nil {
			return err
		}
		if _, err := tx.Exec(
			`UPDATE clusters
			 SET recommendation_archive_date = ?, recommendation_score = ?, recommendation_profile_id = ?, is_ai_recommended = 1, updated_at = CURRENT_TIMESTAMP
			 WHERE id = ? AND user_id = ?`,
			rec.RecommendationDate, rec.RecommendationScore, rec.RecommendationProfileID, rec.ClusterID, userID,
		); err != nil {
			return err
		}
	}

	return tx.Commit()
}

func parseNullableSQLiteTime(value sql.NullString) (time.Time, bool) {
	if !value.Valid || value.String == "" {
		return time.Time{}, false
	}

	formats := []string{
		"2006-01-02 15:04:05 -0700 MST",
		time.RFC3339,
		"2006-01-02T15:04:05Z",
		"2006-01-02T15:04:05",
		"2006-01-02 15:04:05",
	}

	for _, format := range formats {
		parsedTime, err := time.Parse(format, value.String)
		if err == nil {
			return parsedTime, true
		}
	}

	return time.Time{}, false
}

func scanDailyRecommendationCandidates(db *DB, rows *sql.Rows, withDistance bool) ([]DailyRecommendationCandidate, error) {
	results := make([]DailyRecommendationCandidate, 0)
	for rows.Next() {
		var candidate DailyRecommendationCandidate
		var publishedAt sql.NullString
		if withDistance {
			if err := rows.Scan(
				&candidate.Cluster.ID,
				&candidate.Cluster.UserID,
				&candidate.Cluster.Status,
				&candidate.Cluster.MergedTitle,
				&candidate.Cluster.MergedSummary,
				&candidate.Cluster.MergedContent,
				&candidate.Cluster.RecommendationArchiveDate,
				&candidate.Cluster.RecommendationScore,
				&candidate.Cluster.IsAIRecommended,
				&candidate.Cluster.RecommendationProfileID,
				&candidate.Cluster.ArticleCount,
				&candidate.Cluster.CreatedAt,
				&candidate.Cluster.UpdatedAt,
				&candidate.Cluster.IsRead,
				&candidate.Cluster.IsFavorite,
				&candidate.Cluster.IsReadLater,
				&candidate.Cluster.IsHidden,
				&publishedAt,
				&candidate.Distance,
			); err != nil {
				log.Printf("scan recommendation candidate with distance: %v", err)
				continue
			}
		} else {
			if err := rows.Scan(
				&candidate.Cluster.ID,
				&candidate.Cluster.UserID,
				&candidate.Cluster.Status,
				&candidate.Cluster.MergedTitle,
				&candidate.Cluster.MergedSummary,
				&candidate.Cluster.MergedContent,
				&candidate.Cluster.RecommendationArchiveDate,
				&candidate.Cluster.RecommendationScore,
				&candidate.Cluster.IsAIRecommended,
				&candidate.Cluster.RecommendationProfileID,
				&candidate.Cluster.ArticleCount,
				&candidate.Cluster.CreatedAt,
				&candidate.Cluster.UpdatedAt,
				&candidate.Cluster.IsRead,
				&candidate.Cluster.IsFavorite,
				&candidate.Cluster.IsReadLater,
				&candidate.Cluster.IsHidden,
				&publishedAt,
			); err != nil {
				log.Printf("scan recommendation candidate: %v", err)
				continue
			}
		}
		if parsedTime, ok := parseNullableSQLiteTime(publishedAt); ok {
			candidate.PublishedAt = parsedTime
		}
		db.populateClusterMeta(&candidate.Cluster)
		results = append(results, candidate)
	}
	return results, rows.Err()
}
