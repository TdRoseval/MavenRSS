package sqlite

import "log"

func (db *DB) getClusterCount(userID int64, clusterCondition string, args ...interface{}) (int, error) {
	db.WaitForReady()

	query := `SELECT COUNT(*) FROM clusters WHERE is_hidden = 0`
	queryArgs := make([]interface{}, 0, len(args)+1)
	if userID > 0 {
		query += ` AND user_id = ?`
		queryArgs = append(queryArgs, userID)
	}
	if clusterCondition != "" {
		query += ` AND ` + clusterCondition
		queryArgs = append(queryArgs, args...)
	}

	var count int
	if err := db.QueryRow(query, queryArgs...).Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

func (db *DB) getClusterCountsForAllFeeds(userID int64, articleCondition string, clusterCondition string, args ...interface{}) (map[int64]int, error) {
	db.WaitForReady()

	query := `
		SELECT a.feed_id, COUNT(DISTINCT c.id)
		FROM clusters c
		JOIN articles a ON a.cluster_id = c.id
		WHERE c.is_hidden = 0`
	queryArgs := make([]interface{}, 0, len(args)+1)
	if userID > 0 {
		query += ` AND c.user_id = ?`
		queryArgs = append(queryArgs, userID)
	}
	if clusterCondition != "" {
		query += ` AND ` + clusterCondition
		queryArgs = append(queryArgs, args...)
	}
	if articleCondition != "" {
		query += ` AND ` + articleCondition
	}
	query += ` GROUP BY a.feed_id`

	rows, err := db.Query(query, queryArgs...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	counts := make(map[int64]int)
	for rows.Next() {
		var feedID int64
		var count int
		if err := rows.Scan(&feedID, &count); err != nil {
			log.Println("Error scanning cluster count:", err)
			continue
		}
		counts[feedID] = count
	}
	return counts, rows.Err()
}

func (db *DB) GetTotalUnreadCount(userID int64) (int, error) {
	db.WaitForReady()
	var count int
	var err error
	if userID > 0 {
		err = db.QueryRow("SELECT COUNT(*) FROM articles WHERE user_id = ? AND is_read = 0 AND is_hidden = 0", userID).Scan(&count)
	} else {
		err = db.QueryRow("SELECT COUNT(*) FROM articles WHERE is_read = 0 AND is_hidden = 0").Scan(&count)
	}
	if err != nil {
		return 0, err
	}
	return count, nil
}

func (db *DB) GetTotalUnreadClusterCount(userID int64) (int, error) {
	return db.getClusterCount(userID, "is_read = 0")
}

func (db *DB) GetUnreadCountByFeed(feedID int64, userID int64) (int, error) {
	db.WaitForReady()
	var count int
	var err error
	if userID > 0 {
		err = db.QueryRow("SELECT COUNT(*) FROM articles WHERE user_id = ? AND feed_id = ? AND is_read = 0 AND is_hidden = 0", userID, feedID).Scan(&count)
	} else {
		err = db.QueryRow("SELECT COUNT(*) FROM articles WHERE feed_id = ? AND is_read = 0 AND is_hidden = 0", feedID).Scan(&count)
	}
	if err != nil {
		return 0, err
	}
	return count, nil
}

func (db *DB) GetUnreadCountsForAllFeeds(userID int64) (map[int64]int, error) {
	db.WaitForReady()
	var rows interface {
		Scan(dest ...interface{}) error
		Next() bool
		Err() error
		Close() error
	}
	var err error
	if userID > 0 {
		rows, err = db.Query(`
			SELECT feed_id, COUNT(*)
			FROM articles
			WHERE user_id = ? AND is_read = 0 AND is_hidden = 0
			GROUP BY feed_id
		`, userID)
	} else {
		rows, err = db.Query(`
			SELECT feed_id, COUNT(*)
			FROM articles
			WHERE is_read = 0 AND is_hidden = 0
			GROUP BY feed_id
		`)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	counts := make(map[int64]int)
	for rows.Next() {
		var feedID int64
		var count int
		if err := rows.Scan(&feedID, &count); err != nil {
			log.Println("Error scanning unread count:", err)
			continue
		}
		counts[feedID] = count
	}
	return counts, rows.Err()
}

func (db *DB) GetUnreadClusterCountsForAllFeeds(userID int64) (map[int64]int, error) {
	return db.getClusterCountsForAllFeeds(userID, "", "c.is_read = 0")
}

// ClusterFilterCountsByFeed holds per-feed cluster counts for every filter
// type, computed in a single joined scan.
type ClusterFilterCountsByFeed struct {
	Unread           map[int64]int
	Favorites        map[int64]int
	FavoritesUnread  map[int64]int
	ReadLater        map[int64]int
	ReadLaterUnread  map[int64]int
	Images           map[int64]int
	ImagesUnread     map[int64]int
}

// GetAllClusterFilterCountsForAllFeeds computes per-feed cluster counts for all
// filter types (unread, favorites, read-later, images) in a single joined scan
// with conditional aggregation, instead of running seven separate
// COUNT(DISTINCT) queries over the same join.
func (db *DB) GetAllClusterFilterCountsForAllFeeds(userID int64) (*ClusterFilterCountsByFeed, error) {
	db.WaitForReady()

	query := `
		SELECT a.feed_id,
			COUNT(DISTINCT CASE WHEN c.is_read = 0 THEN c.id END),
			COUNT(DISTINCT CASE WHEN c.is_favorite = 1 THEN c.id END),
			COUNT(DISTINCT CASE WHEN c.is_favorite = 1 AND c.is_read = 0 THEN c.id END),
			COUNT(DISTINCT CASE WHEN c.is_read_later = 1 THEN c.id END),
			COUNT(DISTINCT CASE WHEN c.is_read_later = 1 AND c.is_read = 0 THEN c.id END),
			COUNT(DISTINCT CASE WHEN (a.image_url IS NOT NULL AND a.image_url != '') THEN c.id END),
			COUNT(DISTINCT CASE WHEN (a.image_url IS NOT NULL AND a.image_url != '') AND c.is_read = 0 THEN c.id END)
		FROM clusters c
		JOIN articles a ON a.cluster_id = c.id
		WHERE c.is_hidden = 0`
	queryArgs := make([]interface{}, 0, 1)
	if userID > 0 {
		query += ` AND c.user_id = ?`
		queryArgs = append(queryArgs, userID)
	}
	query += ` GROUP BY a.feed_id`

	rows, err := db.Query(query, queryArgs...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	counts := &ClusterFilterCountsByFeed{
		Unread:          make(map[int64]int),
		Favorites:        make(map[int64]int),
		FavoritesUnread:  make(map[int64]int),
		ReadLater:        make(map[int64]int),
		ReadLaterUnread:  make(map[int64]int),
		Images:           make(map[int64]int),
		ImagesUnread:     make(map[int64]int),
	}

	for rows.Next() {
		var feedID int64
		var unread, favorite, favoriteUnread, readLater, readLaterUnread, image, imageUnread int
		if err := rows.Scan(&feedID, &unread, &favorite, &favoriteUnread, &readLater, &readLaterUnread, &image, &imageUnread); err != nil {
			log.Println("Error scanning cluster filter counts:", err)
			continue
		}
		counts.Unread[feedID] = unread
		counts.Favorites[feedID] = favorite
		counts.FavoritesUnread[feedID] = favoriteUnread
		counts.ReadLater[feedID] = readLater
		counts.ReadLaterUnread[feedID] = readLaterUnread
		counts.Images[feedID] = image
		counts.ImagesUnread[feedID] = imageUnread
	}
	return counts, rows.Err()
}

func (db *DB) GetFavoriteCountsForAllFeeds(userID int64) (map[int64]int, error) {
	db.WaitForReady()
	var rows interface {
		Scan(dest ...interface{}) error
		Next() bool
		Err() error
		Close() error
	}
	var err error
	if userID > 0 {
		rows, err = db.Query(`
			SELECT feed_id, COUNT(*)
			FROM articles
			WHERE user_id = ? AND is_favorite = 1 AND is_hidden = 0
			GROUP BY feed_id
		`, userID)
	} else {
		rows, err = db.Query(`
			SELECT feed_id, COUNT(*)
			FROM articles
			WHERE is_favorite = 1 AND is_hidden = 0
			GROUP BY feed_id
		`)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	counts := make(map[int64]int)
	for rows.Next() {
		var feedID int64
		var count int
		if err := rows.Scan(&feedID, &count); err != nil {
			log.Println("Error scanning favorite count:", err)
			continue
		}
		counts[feedID] = count
	}
	return counts, rows.Err()
}

func (db *DB) GetReadLaterCountsForAllFeeds(userID int64) (map[int64]int, error) {
	db.WaitForReady()
	var rows interface {
		Scan(dest ...interface{}) error
		Next() bool
		Err() error
		Close() error
	}
	var err error
	if userID > 0 {
		rows, err = db.Query(`
			SELECT feed_id, COUNT(*)
			FROM articles
			WHERE user_id = ? AND is_read_later = 1 AND is_hidden = 0
			GROUP BY feed_id
		`, userID)
	} else {
		rows, err = db.Query(`
			SELECT feed_id, COUNT(*)
			FROM articles
			WHERE is_read_later = 1 AND is_hidden = 0
			GROUP BY feed_id
		`)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	counts := make(map[int64]int)
	for rows.Next() {
		var feedID int64
		var count int
		if err := rows.Scan(&feedID, &count); err != nil {
			log.Println("Error scanning read_later count:", err)
			continue
		}
		counts[feedID] = count
	}
	return counts, rows.Err()
}

func (db *DB) GetImageModeCountsForAllFeeds(userID int64) (map[int64]int, error) {
	db.WaitForReady()
	var rows interface {
		Scan(dest ...interface{}) error
		Next() bool
		Err() error
		Close() error
	}
	var err error
	if userID > 0 {
		rows, err = db.Query(`
			SELECT feed_id, COUNT(*)
			FROM articles
			WHERE user_id = ? AND (image_url IS NOT NULL AND image_url != '') AND is_hidden = 0
			GROUP BY feed_id
		`, userID)
	} else {
		rows, err = db.Query(`
			SELECT feed_id, COUNT(*)
			FROM articles
			WHERE (image_url IS NOT NULL AND image_url != '') AND is_hidden = 0
			GROUP BY feed_id
		`)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	counts := make(map[int64]int)
	for rows.Next() {
		var feedID int64
		var count int
		if err := rows.Scan(&feedID, &count); err != nil {
			log.Println("Error scanning image mode count:", err)
			continue
		}
		counts[feedID] = count
	}
	return counts, rows.Err()
}

func (db *DB) GetImageUnreadCountsForAllFeeds(userID int64) (map[int64]int, error) {
	db.WaitForReady()
	var rows interface {
		Scan(dest ...interface{}) error
		Next() bool
		Err() error
		Close() error
	}
	var err error
	if userID > 0 {
		rows, err = db.Query(`
			SELECT feed_id, COUNT(*)
			FROM articles
			WHERE user_id = ? AND (image_url IS NOT NULL AND image_url != '') AND is_read = 0 AND is_hidden = 0
			GROUP BY feed_id
		`, userID)
	} else {
		rows, err = db.Query(`
			SELECT feed_id, COUNT(*)
			FROM articles
			WHERE (image_url IS NOT NULL AND image_url != '') AND is_read = 0 AND is_hidden = 0
			GROUP BY feed_id
		`)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	counts := make(map[int64]int)
	for rows.Next() {
		var feedID int64
		var count int
		if err := rows.Scan(&feedID, &count); err != nil {
			log.Println("Error scanning image unread count:", err)
			continue
		}
		counts[feedID] = count
	}
	return counts, rows.Err()
}

func (db *DB) GetFavoriteUnreadCountsForAllFeeds(userID int64) (map[int64]int, error) {
	db.WaitForReady()
	var rows interface {
		Scan(dest ...interface{}) error
		Next() bool
		Err() error
		Close() error
	}
	var err error
	if userID > 0 {
		rows, err = db.Query(`
			SELECT feed_id, COUNT(*)
			FROM articles
			WHERE user_id = ? AND is_favorite = 1 AND is_read = 0 AND is_hidden = 0
			GROUP BY feed_id
		`, userID)
	} else {
		rows, err = db.Query(`
			SELECT feed_id, COUNT(*)
			FROM articles
			WHERE is_favorite = 1 AND is_read = 0 AND is_hidden = 0
			GROUP BY feed_id
		`)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	counts := make(map[int64]int)
	for rows.Next() {
		var feedID int64
		var count int
		if err := rows.Scan(&feedID, &count); err != nil {
			log.Println("Error scanning favorite unread count:", err)
			continue
		}
		counts[feedID] = count
	}
	return counts, rows.Err()
}

func (db *DB) GetReadLaterUnreadCountsForAllFeeds(userID int64) (map[int64]int, error) {
	db.WaitForReady()
	var rows interface {
		Scan(dest ...interface{}) error
		Next() bool
		Err() error
		Close() error
	}
	var err error
	if userID > 0 {
		rows, err = db.Query(`
			SELECT feed_id, COUNT(*)
			FROM articles
			WHERE user_id = ? AND is_read_later = 1 AND is_read = 0 AND is_hidden = 0
			GROUP BY feed_id
		`, userID)
	} else {
		rows, err = db.Query(`
			SELECT feed_id, COUNT(*)
			FROM articles
			WHERE is_read_later = 1 AND is_read = 0 AND is_hidden = 0
			GROUP BY feed_id
		`)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	counts := make(map[int64]int)
	for rows.Next() {
		var feedID int64
		var count int
		if err := rows.Scan(&feedID, &count); err != nil {
			log.Println("Error scanning read_later unread count:", err)
			continue
		}
		counts[feedID] = count
	}
	return counts, rows.Err()
}
