package database

import (
	"database/sql"
	"errors"
	"strconv"
	"time"

	"MrRSS/internal/models"
)

func (db *DB) CreateUser(user *models.User) (int64, error) {
	query := `
		INSERT INTO users (username, email, password_hash, role, status)
		VALUES (?, ?, ?, ?, ?)
	`
	result, err := db.Exec(query, user.Username, user.Email, user.PasswordHash, user.Role, user.Status)
	if err != nil {
		return 0, err
	}
	return result.LastInsertId()
}

func (db *DB) GetUserByID(id int64) (*models.User, error) {
	query := `
		SELECT id, username, email, password_hash, role, status, 
			   inherited_from, has_inherited, created_at, updated_at
		FROM users WHERE id = ?
	`
	var user models.User
	var inheritedFrom sql.NullInt64
	err := db.QueryRow(query, id).Scan(
		&user.ID, &user.Username, &user.Email, &user.PasswordHash, &user.Role, &user.Status,
		&inheritedFrom, &user.HasInherited, &user.CreatedAt, &user.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	if inheritedFrom.Valid {
		user.InheritedFrom = &inheritedFrom.Int64
	}
	return &user, nil
}

func (db *DB) GetUserByUsername(username string) (*models.User, error) {
	query := `
		SELECT id, username, email, password_hash, role, status, 
			   inherited_from, has_inherited, created_at, updated_at
		FROM users WHERE username = ?
	`
	var user models.User
	var inheritedFrom sql.NullInt64
	err := db.QueryRow(query, username).Scan(
		&user.ID, &user.Username, &user.Email, &user.PasswordHash, &user.Role, &user.Status,
		&inheritedFrom, &user.HasInherited, &user.CreatedAt, &user.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	if inheritedFrom.Valid {
		user.InheritedFrom = &inheritedFrom.Int64
	}
	return &user, nil
}

func (db *DB) GetUserByEmail(email string) (*models.User, error) {
	query := `
		SELECT id, username, email, password_hash, role, status, 
			   inherited_from, has_inherited, created_at, updated_at
		FROM users WHERE email = ?
	`
	var user models.User
	var inheritedFrom sql.NullInt64
	err := db.QueryRow(query, email).Scan(
		&user.ID, &user.Username, &user.Email, &user.PasswordHash, &user.Role, &user.Status,
		&inheritedFrom, &user.HasInherited, &user.CreatedAt, &user.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	if inheritedFrom.Valid {
		user.InheritedFrom = &inheritedFrom.Int64
	}
	return &user, nil
}

func (db *DB) UpdateUser(user *models.User) error {
	query := `
		UPDATE users 
		SET username = ?, email = ?, role = ?, status = ?, 
		    inherited_from = ?, has_inherited = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`
	_, err := db.Exec(query, user.Username, user.Email, user.Role, user.Status, user.InheritedFrom, user.HasInherited, user.ID)
	return err
}

func (db *DB) DeleteUser(id int64) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Delete user's data explicitly (double insurance even if foreign keys are not working)
	_, _ = tx.Exec(`DELETE FROM chat_messages WHERE session_id IN (SELECT id FROM chat_sessions WHERE user_id = ?)`, id)
	_, _ = tx.Exec(`DELETE FROM chat_sessions WHERE user_id = ?`, id)
	_, _ = tx.Exec(`DELETE FROM article_contents WHERE article_id IN (SELECT id FROM articles WHERE user_id = ?)`, id)
	_, _ = tx.Exec(`DELETE FROM articles WHERE user_id = ?`, id)
	_, _ = tx.Exec(`DELETE FROM feeds WHERE user_id = ?`, id)
	_, _ = tx.Exec(`DELETE FROM saved_filters WHERE user_id = ?`, id)
	_, _ = tx.Exec(`DELETE FROM tags WHERE user_id = ?`, id)
	_, _ = tx.Exec(`DELETE FROM ai_profiles WHERE user_id = ?`, id)
	_, _ = tx.Exec(`DELETE FROM user_settings WHERE user_id = ?`, id)
	_, _ = tx.Exec(`DELETE FROM user_quota WHERE user_id = ?`, id)
	_, _ = tx.Exec(`DELETE FROM user_sessions WHERE user_id = ?`, id)

	// Finally delete the user
	_, err = tx.Exec(`DELETE FROM users WHERE id = ?`, id)
	if err != nil {
		return err
	}

	return tx.Commit()
}

func (db *DB) ListUsers() ([]*models.User, error) {
	query := `
		SELECT id, username, email, password_hash, role, status, 
			   inherited_from, has_inherited, created_at, updated_at
		FROM users ORDER BY created_at DESC
	`
	rows, err := db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []*models.User
	for rows.Next() {
		var user models.User
		var inheritedFrom sql.NullInt64
		err := rows.Scan(
			&user.ID, &user.Username, &user.Email, &user.PasswordHash, &user.Role, &user.Status,
			&inheritedFrom, &user.HasInherited, &user.CreatedAt, &user.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		if inheritedFrom.Valid {
			user.InheritedFrom = &inheritedFrom.Int64
		}
		users = append(users, &user)
	}
	return users, nil
}

func (db *DB) ListUsersPaginated(page, pageSize int) ([]*models.User, int, error) {
	offset := (page - 1) * pageSize

	var total int
	err := db.QueryRow("SELECT COUNT(*) FROM users").Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	query := `
		SELECT id, username, email, password_hash, role, status, 
			   inherited_from, has_inherited, created_at, updated_at
		FROM users ORDER BY created_at DESC LIMIT ? OFFSET ?
	`
	rows, err := db.Query(query, pageSize, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var users []*models.User
	for rows.Next() {
		var user models.User
		var inheritedFrom sql.NullInt64
		err := rows.Scan(
			&user.ID, &user.Username, &user.Email, &user.PasswordHash, &user.Role, &user.Status,
			&inheritedFrom, &user.HasInherited, &user.CreatedAt, &user.UpdatedAt,
		)
		if err != nil {
			return nil, 0, err
		}
		if inheritedFrom.Valid {
			user.InheritedFrom = &inheritedFrom.Int64
		}
		users = append(users, &user)
	}
	return users, total, nil
}

func (db *DB) CreatePendingRegistration(reg *models.PendingRegistration) (int64, error) {
	query := `
		INSERT INTO pending_registrations (username, email, password_hash)
		VALUES (?, ?, ?)
	`
	result, err := db.Exec(query, reg.Username, reg.Email, reg.PasswordHash)
	if err != nil {
		return 0, err
	}
	return result.LastInsertId()
}

func (db *DB) GetPendingRegistrationByID(id int64) (*models.PendingRegistration, error) {
	query := `
		SELECT id, username, email, password_hash, created_at
		FROM pending_registrations WHERE id = ?
	`
	var reg models.PendingRegistration
	err := db.QueryRow(query, id).Scan(&reg.ID, &reg.Username, &reg.Email, &reg.PasswordHash, &reg.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &reg, nil
}

func (db *DB) GetPendingRegistrationByUsername(username string) (*models.PendingRegistration, error) {
	query := `
		SELECT id, username, email, password_hash, created_at
		FROM pending_registrations WHERE username = ?
	`
	var reg models.PendingRegistration
	err := db.QueryRow(query, username).Scan(&reg.ID, &reg.Username, &reg.Email, &reg.PasswordHash, &reg.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &reg, nil
}

func (db *DB) DeletePendingRegistration(id int64) error {
	query := `DELETE FROM pending_registrations WHERE id = ?`
	_, err := db.Exec(query, id)
	return err
}

func (db *DB) ListPendingRegistrations() ([]*models.PendingRegistration, error) {
	query := `
		SELECT id, username, email, password_hash, created_at
		FROM pending_registrations ORDER BY created_at DESC
	`
	rows, err := db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var regs []*models.PendingRegistration
	for rows.Next() {
		var reg models.PendingRegistration
		err := rows.Scan(&reg.ID, &reg.Username, &reg.Email, &reg.PasswordHash, &reg.CreatedAt)
		if err != nil {
			return nil, err
		}
		regs = append(regs, &reg)
	}
	return regs, nil
}

func (db *DB) ListPendingRegistrationsPaginated(page, pageSize int) ([]*models.PendingRegistration, int, error) {
	offset := (page - 1) * pageSize
	var total int
	err := db.QueryRow("SELECT COUNT(*) FROM pending_registrations").Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	query := `
		SELECT id, username, email, password_hash, created_at
		FROM pending_registrations ORDER BY created_at DESC
		LIMIT ? OFFSET ?
	`
	rows, err := db.Query(query, pageSize, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var regs []*models.PendingRegistration
	for rows.Next() {
		var reg models.PendingRegistration
		err := rows.Scan(&reg.ID, &reg.Username, &reg.Email, &reg.PasswordHash, &reg.CreatedAt)
		if err != nil {
			return nil, 0, err
		}
		regs = append(regs, &reg)
	}
	return regs, total, nil
}

func (db *DB) CreateUserSession(session *models.UserSession) (int64, error) {
	query := `
		INSERT INTO user_sessions (user_id, refresh_token, user_agent, ip_address, expires_at)
		VALUES (?, ?, ?, ?, ?)
	`
	result, err := db.Exec(query, session.UserID, session.RefreshToken, session.UserAgent, session.IPAddress, session.ExpiresAt)
	if err != nil {
		return 0, err
	}
	return result.LastInsertId()
}

func (db *DB) GetUserSessionByToken(token string) (*models.UserSession, error) {
	query := `
		SELECT id, user_id, refresh_token, user_agent, ip_address, expires_at, created_at
		FROM user_sessions WHERE refresh_token = ?
	`
	var session models.UserSession
	err := db.QueryRow(query, token).Scan(
		&session.ID, &session.UserID, &session.RefreshToken, &session.UserAgent, 
		&session.IPAddress, &session.ExpiresAt, &session.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &session, nil
}

func (db *DB) DeleteUserSession(id int64) error {
	query := `DELETE FROM user_sessions WHERE id = ?`
	_, err := db.Exec(query, id)
	return err
}

func (db *DB) DeleteUserSessions(userID int64) error {
	query := `DELETE FROM user_sessions WHERE user_id = ?`
	_, err := db.Exec(query, userID)
	return err
}

func (db *DB) CleanupExpiredSessions() error {
	query := `DELETE FROM user_sessions WHERE expires_at < ?`
	_, err := db.Exec(query, time.Now())
	return err
}

func (db *DB) CreateUserQuota(quota *models.UserQuota) (int64, error) {
	query := `
		INSERT INTO user_quota (user_id, max_feeds, max_articles, max_ai_tokens, max_ai_concurrency, max_feed_fetch_concurrency, max_db_query_concurrency, max_media_cache_concurrency, max_rss_discovery_concurrency, max_rss_path_check_concurrency, max_translation_concurrency, max_storage_mb)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`
	result, err := db.Exec(query, quota.UserID, quota.MaxFeeds, quota.MaxArticles, quota.MaxAITokens, quota.MaxAIConcurrency, quota.MaxFeedFetchConcurrency, quota.MaxDBQueryConcurrency, quota.MaxMediaCacheConcurrency, quota.MaxRSSDiscoveryConcurrency, quota.MaxRSSPathCheckConcurrency, quota.MaxTranslationConcurrency, quota.MaxStorageMB)
	if err != nil {
		return 0, err
	}
	return result.LastInsertId()
}

func (db *DB) GetUserQuota(userID int64) (*models.UserQuota, error) {
	query := `
		SELECT id, user_id, max_feeds, max_articles, max_ai_tokens, max_ai_concurrency, max_feed_fetch_concurrency, max_db_query_concurrency, max_media_cache_concurrency, max_rss_discovery_concurrency, max_rss_path_check_concurrency, max_translation_concurrency, max_storage_mb,
			   used_feeds, used_articles, used_ai_tokens, used_storage_mb,
			   created_at, updated_at
		FROM user_quota WHERE user_id = ?
	`
	var quota models.UserQuota
	err := db.QueryRow(query, userID).Scan(
		&quota.ID, &quota.UserID, &quota.MaxFeeds, &quota.MaxArticles, &quota.MaxAITokens, &quota.MaxAIConcurrency, &quota.MaxFeedFetchConcurrency, &quota.MaxDBQueryConcurrency, &quota.MaxMediaCacheConcurrency, &quota.MaxRSSDiscoveryConcurrency, &quota.MaxRSSPathCheckConcurrency, &quota.MaxTranslationConcurrency, &quota.MaxStorageMB,
		&quota.UsedFeeds, &quota.UsedArticles, &quota.UsedAITokens, &quota.UsedStorageMB,
		&quota.CreatedAt, &quota.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	var feedCount int
	err = db.QueryRow("SELECT COUNT(*) FROM feeds WHERE user_id = ?", userID).Scan(&feedCount)
	if err != nil {
		feedCount = 0
	}
	quota.UsedFeeds = feedCount

	var articleCount int64
	err = db.QueryRow("SELECT COUNT(*) FROM articles WHERE user_id = ?", userID).Scan(&articleCount)
	if err != nil {
		articleCount = 0
	}
	quota.UsedArticles = articleCount

	var totalSizeBytes int64
	err = db.QueryRow(`
		SELECT IFNULL(SUM(LENGTH(content)), 0) 
		FROM article_contents 
		WHERE article_id IN (SELECT id FROM articles WHERE user_id = ?)
	`, userID).Scan(&totalSizeBytes)
	if err != nil {
		totalSizeBytes = 0
	}
	quota.UsedStorageMB = int(totalSizeBytes / (1024 * 1024))

	return &quota, nil
}

func (db *DB) UpdateUserQuota(quota *models.UserQuota) error {
	query := `
		UPDATE user_quota
		SET max_feeds = ?, max_articles = ?, max_ai_tokens = ?, max_ai_concurrency = ?, max_feed_fetch_concurrency = ?, max_db_query_concurrency = ?, max_media_cache_concurrency = ?, max_rss_discovery_concurrency = ?, max_rss_path_check_concurrency = ?, max_translation_concurrency = ?, max_storage_mb = ?,
		    used_feeds = ?, used_articles = ?, used_ai_tokens = ?, used_storage_mb = ?,
		    updated_at = CURRENT_TIMESTAMP
		WHERE user_id = ?
	`
	_, err := db.Exec(query, 
		quota.MaxFeeds, quota.MaxArticles, quota.MaxAITokens, quota.MaxAIConcurrency, quota.MaxFeedFetchConcurrency, quota.MaxDBQueryConcurrency, quota.MaxMediaCacheConcurrency, quota.MaxRSSDiscoveryConcurrency, quota.MaxRSSPathCheckConcurrency, quota.MaxTranslationConcurrency, quota.MaxStorageMB,
		quota.UsedFeeds, quota.UsedArticles, quota.UsedAITokens, quota.UsedStorageMB,
		quota.UserID,
	)
	return err
}

func (db *DB) IncrementAICalls(userID int64) error {
	query := `
		UPDATE user_quota
		SET used_ai_calls_today = used_ai_calls_today + 1, updated_at = CURRENT_TIMESTAMP
		WHERE user_id = ?
	`
	_, err := db.Exec(query, userID)
	return err
}

func (db *DB) ResetDailyAICalls() error {
	query := `
		UPDATE user_quota
		SET used_ai_calls_today = 0, updated_at = CURRENT_TIMESTAMP
	`
	_, err := db.Exec(query)
	return err
}

func (db *DB) GetTemplateUser() (*models.User, error) {
	query := `
		SELECT id, username, email, password_hash, role, status, 
			   inherited_from, has_inherited, created_at, updated_at
		FROM users WHERE role = ? LIMIT 1
	`
	var user models.User
	var inheritedFrom sql.NullInt64
	err := db.QueryRow(query, models.RoleTemplate).Scan(
		&user.ID, &user.Username, &user.Email, &user.PasswordHash, &user.Role, &user.Status,
		&inheritedFrom, &user.HasInherited, &user.CreatedAt, &user.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	if inheritedFrom.Valid {
		user.InheritedFrom = &inheritedFrom.Int64
	}
	return &user, nil
}

var (
	ErrQuotaExceededFeeds     = errors.New("quota exceeded: maximum number of feeds reached")
	ErrQuotaExceededArticles  = errors.New("quota exceeded: maximum number of articles reached")
	ErrQuotaExceededStorage   = errors.New("quota exceeded: maximum storage reached")
)

func (db *DB) CheckFeedQuota(userID int64) (bool, error) {
	quota, err := db.GetUserQuota(userID)
	if err != nil {
		return true, nil
	}
	
	var currentCount int
	err = db.QueryRow("SELECT COUNT(*) FROM feeds WHERE user_id = ?", userID).Scan(&currentCount)
	if err != nil {
		return true, nil
	}
	
	if quota.MaxFeeds > 0 && currentCount >= quota.MaxFeeds {
		return false, ErrQuotaExceededFeeds
	}
	
	return true, nil
}

func (db *DB) CheckArticleQuota(userID int64, additionalCount int64) (bool, error) {
	quota, err := db.GetUserQuota(userID)
	if err != nil {
		return true, nil
	}
	
	var currentCount int64
	err = db.QueryRow("SELECT COUNT(*) FROM articles WHERE user_id = ?", userID).Scan(&currentCount)
	if err != nil {
		return true, nil
	}
	
	if quota.MaxArticles > 0 && currentCount+additionalCount > quota.MaxArticles {
		return false, ErrQuotaExceededArticles
	}
	
	return true, nil
}

func (db *DB) CheckStorageQuota(userID int64) (bool, error) {
	quota, err := db.GetUserQuota(userID)
	if err != nil {
		return true, nil
	}
	
	if quota.MaxStorageMB <= 0 {
		return true, nil
	}
	
	var totalSizeBytes int64
	err = db.QueryRow(`
		SELECT IFNULL(SUM(LENGTH(content)), 0) 
		FROM article_contents 
		WHERE article_id IN (SELECT id FROM articles WHERE user_id = ?)
	`, userID).Scan(&totalSizeBytes)
	if err != nil {
		return true, nil
	}
	
	currentSizeMB := int(totalSizeBytes / (1024 * 1024))
	if currentSizeMB >= quota.MaxStorageMB {
		return false, ErrQuotaExceededStorage
	}
	
	return true, nil
}

func (db *DB) GetEffectiveMaxCacheSizeMB(userID int64) int {
	quota, err := db.GetUserQuota(userID)
	if err != nil || quota.MaxStorageMB <= 0 {
		defaultSizeStr, _ := db.GetSetting("max_cache_size_mb")
		if defaultSize, err := strconv.Atoi(defaultSizeStr); err == nil && defaultSize > 0 {
			return defaultSize
		}
		return 500
	}
	
	userSettingStr, _ := db.GetSettingForUser(userID, "max_cache_size_mb")
	if userSettingSize, err := strconv.Atoi(userSettingStr); err == nil && userSettingSize > 0 {
		if userSettingSize <= quota.MaxStorageMB {
			return userSettingSize
		}
	}
	
	return quota.MaxStorageMB
}
