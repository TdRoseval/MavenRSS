package database

import (
	"database/sql"
	"fmt"
	"time"

	"MrRSS/internal/crypto"
	"MrRSS/internal/models"
)

// CreateAIProfile creates a new AI profile
func (db *DB) CreateAIProfile(profile *models.AIProfile) (int64, error) {
	// Encrypt API key before storing
	encryptedKey := profile.APIKey
	if profile.APIKey != "" {
		encrypted, err := crypto.Encrypt(profile.APIKey)
		if err != nil {
			return 0, fmt.Errorf("encrypt API key: %w", err)
		}
		encryptedKey = encrypted
	}

	now := time.Now()
	result, err := db.Exec(`
		INSERT INTO ai_profiles (user_id, name, api_key, endpoint, model, custom_headers, is_default, use_global_proxy, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, profile.UserID, profile.Name, encryptedKey, profile.Endpoint, profile.Model, profile.CustomHeaders, profile.IsDefault, profile.UseGlobalProxy, now, now)
	if err != nil {
		return 0, fmt.Errorf("insert ai profile: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("get last insert id: %w", err)
	}

	// If this is set as default, unset other defaults
	if profile.IsDefault {
		_, _ = db.Exec(`UPDATE ai_profiles SET is_default = 0 WHERE user_id = ? AND id != ?`, profile.UserID, id)
	}

	return id, nil
}

// GetAIProfile retrieves an AI profile by ID
func (db *DB) GetAIProfile(id int64) (*models.AIProfile, error) {
	var profile models.AIProfile
	var encryptedKey string
	err := db.QueryRow(`
		SELECT id, user_id, name, api_key, endpoint, model, custom_headers, is_default, use_global_proxy, created_at, updated_at
		FROM ai_profiles WHERE id = ?
	`, id).Scan(
		&profile.ID, &profile.UserID, &profile.Name, &encryptedKey, &profile.Endpoint,
		&profile.Model, &profile.CustomHeaders, &profile.IsDefault, &profile.UseGlobalProxy,
		&profile.CreatedAt, &profile.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("query ai profile: %w", err)
	}

	// Decrypt API key
	if encryptedKey != "" {
		decrypted, err := crypto.Decrypt(encryptedKey)
		if err == nil {
			profile.APIKey = decrypted
		}
	}

	return &profile, nil
}

// GetAIProfileForUser retrieves an AI profile by ID for a specific user
func (db *DB) GetAIProfileForUser(userID, id int64) (*models.AIProfile, error) {
	var profile models.AIProfile
	var encryptedKey string
	err := db.QueryRow(`
		SELECT id, user_id, name, api_key, endpoint, model, custom_headers, is_default, use_global_proxy, created_at, updated_at
		FROM ai_profiles WHERE user_id = ? AND id = ?
	`, userID, id).Scan(
		&profile.ID, &profile.UserID, &profile.Name, &encryptedKey, &profile.Endpoint,
		&profile.Model, &profile.CustomHeaders, &profile.IsDefault, &profile.UseGlobalProxy,
		&profile.CreatedAt, &profile.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("query ai profile: %w", err)
	}

	// Decrypt API key
	if encryptedKey != "" {
		decrypted, err := crypto.Decrypt(encryptedKey)
		if err == nil {
			profile.APIKey = decrypted
		}
	}

	return &profile, nil
}

// GetAllAIProfiles retrieves all AI profiles
func (db *DB) GetAllAIProfiles() ([]models.AIProfile, error) {
	rows, err := db.Query(`
		SELECT id, user_id, name, api_key, endpoint, model, custom_headers, is_default, use_global_proxy, created_at, updated_at
		FROM ai_profiles ORDER BY is_default DESC, name ASC
	`)
	if err != nil {
		return nil, fmt.Errorf("query ai profiles: %w", err)
	}
	defer rows.Close()

	var profiles []models.AIProfile
	for rows.Next() {
		var profile models.AIProfile
		var encryptedKey string
		err := rows.Scan(
			&profile.ID, &profile.UserID, &profile.Name, &encryptedKey, &profile.Endpoint,
			&profile.Model, &profile.CustomHeaders, &profile.IsDefault, &profile.UseGlobalProxy,
			&profile.CreatedAt, &profile.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scan ai profile: %w", err)
		}

		// Decrypt API key
		if encryptedKey != "" {
			decrypted, err := crypto.Decrypt(encryptedKey)
			if err == nil {
				profile.APIKey = decrypted
			}
		}

		profiles = append(profiles, profile)
	}

	return profiles, nil
}

// GetAllAIProfilesForUser retrieves all AI profiles for a specific user
func (db *DB) GetAllAIProfilesForUser(userID int64) ([]models.AIProfile, error) {
	rows, err := db.Query(`
		SELECT id, user_id, name, api_key, endpoint, model, custom_headers, is_default, use_global_proxy, created_at, updated_at
		FROM ai_profiles WHERE user_id = ? ORDER BY is_default DESC, name ASC
	`, userID)
	if err != nil {
		return nil, fmt.Errorf("query ai profiles: %w", err)
	}
	defer rows.Close()

	var profiles []models.AIProfile
	for rows.Next() {
		var profile models.AIProfile
		var encryptedKey string
		err := rows.Scan(
			&profile.ID, &profile.UserID, &profile.Name, &encryptedKey, &profile.Endpoint,
			&profile.Model, &profile.CustomHeaders, &profile.IsDefault, &profile.UseGlobalProxy,
			&profile.CreatedAt, &profile.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scan ai profile: %w", err)
		}

		// Decrypt API key
		if encryptedKey != "" {
			decrypted, err := crypto.Decrypt(encryptedKey)
			if err == nil {
				profile.APIKey = decrypted
			}
		}

		profiles = append(profiles, profile)
	}

	return profiles, nil
}

// GetAllAIProfilesWithoutKeys retrieves all AI profiles without decrypting keys (for list display)
func (db *DB) GetAllAIProfilesWithoutKeys() ([]models.AIProfile, error) {
	rows, err := db.Query(`
		SELECT id, user_id, name, endpoint, model, custom_headers, is_default, use_global_proxy, created_at, updated_at
		FROM ai_profiles ORDER BY is_default DESC, name ASC
	`)
	if err != nil {
		return nil, fmt.Errorf("query ai profiles: %w", err)
	}
	defer rows.Close()

	var profiles []models.AIProfile
	for rows.Next() {
		var profile models.AIProfile
		err := rows.Scan(
			&profile.ID, &profile.UserID, &profile.Name, &profile.Endpoint,
			&profile.Model, &profile.CustomHeaders, &profile.IsDefault, &profile.UseGlobalProxy,
			&profile.CreatedAt, &profile.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scan ai profile: %w", err)
		}
		// API key is intentionally omitted
		profiles = append(profiles, profile)
	}

	return profiles, nil
}

// GetAllAIProfilesWithoutKeysForUser retrieves all AI profiles without decrypting keys for a specific user
func (db *DB) GetAllAIProfilesWithoutKeysForUser(userID int64) ([]models.AIProfile, error) {
	rows, err := db.Query(`
		SELECT id, user_id, name, endpoint, model, custom_headers, is_default, use_global_proxy, created_at, updated_at
		FROM ai_profiles WHERE user_id = ? ORDER BY is_default DESC, name ASC
	`, userID)
	if err != nil {
		return nil, fmt.Errorf("query ai profiles: %w", err)
	}
	defer rows.Close()

	var profiles []models.AIProfile
	for rows.Next() {
		var profile models.AIProfile
		err := rows.Scan(
			&profile.ID, &profile.UserID, &profile.Name, &profile.Endpoint,
			&profile.Model, &profile.CustomHeaders, &profile.IsDefault, &profile.UseGlobalProxy,
			&profile.CreatedAt, &profile.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scan ai profile: %w", err)
		}
		// API key is intentionally omitted
		profiles = append(profiles, profile)
	}

	return profiles, nil
}

// UpdateAIProfile updates an existing AI profile
func (db *DB) UpdateAIProfile(profile *models.AIProfile) error {
	var encryptedKey string
	var err error

	// If new API key is provided, encrypt it; otherwise, keep the existing one
	if profile.APIKey != "" {
		encryptedKey, err = crypto.Encrypt(profile.APIKey)
		if err != nil {
			return fmt.Errorf("encrypt API key: %w", err)
		}
	} else {
		// Get the existing encrypted key from database
		var existingEncryptedKey string
		err := db.QueryRow(`SELECT api_key FROM ai_profiles WHERE user_id = ? AND id = ?`, profile.UserID, profile.ID).Scan(&existingEncryptedKey)
		if err != nil {
			if err == sql.ErrNoRows {
				encryptedKey = ""
			} else {
				return fmt.Errorf("get existing api key: %w", err)
			}
		} else {
			encryptedKey = existingEncryptedKey
		}
	}

	_, err = db.Exec(`
		UPDATE ai_profiles
		SET name = ?, api_key = ?, endpoint = ?, model = ?, custom_headers = ?, is_default = ?, use_global_proxy = ?, updated_at = ?
		WHERE user_id = ? AND id = ?
	`, profile.Name, encryptedKey, profile.Endpoint, profile.Model, profile.CustomHeaders, profile.IsDefault, profile.UseGlobalProxy, time.Now(), profile.UserID, profile.ID)
	if err != nil {
		return fmt.Errorf("update ai profile: %w", err)
	}

	// If this is set as default, unset other defaults
	if profile.IsDefault {
		_, _ = db.Exec(`UPDATE ai_profiles SET is_default = 0 WHERE user_id = ? AND id != ?`, profile.UserID, profile.ID)
	}

	return nil
}

// DeleteAIProfile deletes an AI profile by ID
func (db *DB) DeleteAIProfile(id int64) error {
	_, err := db.Exec(`DELETE FROM ai_profiles WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete ai profile: %w", err)
	}
	return nil
}

// DeleteAIProfileForUser deletes an AI profile by ID for a specific user
func (db *DB) DeleteAIProfileForUser(userID, id int64) error {
	_, err := db.Exec(`DELETE FROM ai_profiles WHERE user_id = ? AND id = ?`, userID, id)
	if err != nil {
		return fmt.Errorf("delete ai profile: %w", err)
	}
	return nil
}

// GetDefaultAIProfile retrieves the default AI profile
func (db *DB) GetDefaultAIProfile() (*models.AIProfile, error) {
	var profile models.AIProfile
	var encryptedKey string
	err := db.QueryRow(`
		SELECT id, user_id, name, api_key, endpoint, model, custom_headers, is_default, use_global_proxy, created_at, updated_at
		FROM ai_profiles WHERE is_default = 1 LIMIT 1
	`).Scan(
		&profile.ID, &profile.UserID, &profile.Name, &encryptedKey, &profile.Endpoint,
		&profile.Model, &profile.CustomHeaders, &profile.IsDefault, &profile.UseGlobalProxy,
		&profile.CreatedAt, &profile.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			// No default profile, try to get any profile
			return db.getFirstAIProfile()
		}
		return nil, fmt.Errorf("query default ai profile: %w", err)
	}

	// Decrypt API key
	if encryptedKey != "" {
		decrypted, err := crypto.Decrypt(encryptedKey)
		if err == nil {
			profile.APIKey = decrypted
		}
	}

	return &profile, nil
}

// GetDefaultAIProfileForUser retrieves the default AI profile for a specific user
func (db *DB) GetDefaultAIProfileForUser(userID int64) (*models.AIProfile, error) {
	var profile models.AIProfile
	var encryptedKey string
	err := db.QueryRow(`
		SELECT id, user_id, name, api_key, endpoint, model, custom_headers, is_default, use_global_proxy, created_at, updated_at
		FROM ai_profiles WHERE user_id = ? AND is_default = 1 LIMIT 1
	`, userID).Scan(
		&profile.ID, &profile.UserID, &profile.Name, &encryptedKey, &profile.Endpoint,
		&profile.Model, &profile.CustomHeaders, &profile.IsDefault, &profile.UseGlobalProxy,
		&profile.CreatedAt, &profile.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			// No default profile, try to get any profile for this user
			return db.getFirstAIProfileForUser(userID)
		}
		return nil, fmt.Errorf("query default ai profile: %w", err)
	}

	// Decrypt API key
	if encryptedKey != "" {
		decrypted, err := crypto.Decrypt(encryptedKey)
		if err == nil {
			profile.APIKey = decrypted
		}
	}

	return &profile, nil
}

// getFirstAIProfile retrieves the first AI profile (fallback when no default is set)
func (db *DB) getFirstAIProfile() (*models.AIProfile, error) {
	var profile models.AIProfile
	var encryptedKey string
	err := db.QueryRow(`
		SELECT id, user_id, name, api_key, endpoint, model, custom_headers, is_default, use_global_proxy, created_at, updated_at
		FROM ai_profiles ORDER BY id ASC LIMIT 1
	`).Scan(
		&profile.ID, &profile.UserID, &profile.Name, &encryptedKey, &profile.Endpoint,
		&profile.Model, &profile.CustomHeaders, &profile.IsDefault, &profile.UseGlobalProxy,
		&profile.CreatedAt, &profile.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("query first ai profile: %w", err)
	}

	// Decrypt API key
	if encryptedKey != "" {
		decrypted, err := crypto.Decrypt(encryptedKey)
		if err == nil {
			profile.APIKey = decrypted
		}
	}

	return &profile, nil
}

// getFirstAIProfileForUser retrieves the first AI profile for a specific user
func (db *DB) getFirstAIProfileForUser(userID int64) (*models.AIProfile, error) {
	var profile models.AIProfile
	var encryptedKey string
	err := db.QueryRow(`
		SELECT id, user_id, name, api_key, endpoint, model, custom_headers, is_default, use_global_proxy, created_at, updated_at
		FROM ai_profiles WHERE user_id = ? ORDER BY id ASC LIMIT 1
	`, userID).Scan(
		&profile.ID, &profile.UserID, &profile.Name, &encryptedKey, &profile.Endpoint,
		&profile.Model, &profile.CustomHeaders, &profile.IsDefault, &profile.UseGlobalProxy,
		&profile.CreatedAt, &profile.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("query first ai profile: %w", err)
	}

	// Decrypt API key
	if encryptedKey != "" {
		decrypted, err := crypto.Decrypt(encryptedKey)
		if err == nil {
			profile.APIKey = decrypted
		}
	}

	return &profile, nil
}

// SetDefaultAIProfile sets a profile as the default
func (db *DB) SetDefaultAIProfile(id int64) error {
	// Unset all defaults first
	_, err := db.Exec(`UPDATE ai_profiles SET is_default = 0`)
	if err != nil {
		return fmt.Errorf("unset defaults: %w", err)
	}

	// Set the new default
	_, err = db.Exec(`UPDATE ai_profiles SET is_default = 1 WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("set default: %w", err)
	}

	return nil
}

// SetDefaultAIProfileForUser sets a profile as the default for a specific user
func (db *DB) SetDefaultAIProfileForUser(userID, id int64) error {
	// Unset all defaults first for this user
	_, err := db.Exec(`UPDATE ai_profiles SET is_default = 0 WHERE user_id = ?`, userID)
	if err != nil {
		return fmt.Errorf("unset defaults: %w", err)
	}

	// Set the new default for this user
	_, err = db.Exec(`UPDATE ai_profiles SET is_default = 1 WHERE user_id = ? AND id = ?`, userID, id)
	if err != nil {
		return fmt.Errorf("set default: %w", err)
	}

	return nil
}

// HasAPIKeySet checks if an AI profile has an API key configured
func (db *DB) HasAPIKeySet(id int64) (bool, error) {
	var apiKey string
	err := db.QueryRow(`SELECT api_key FROM ai_profiles WHERE id = ?`, id).Scan(&apiKey)
	if err != nil {
		if err == sql.ErrNoRows {
			return false, nil
		}
		return false, fmt.Errorf("query api key: %w", err)
	}
	return apiKey != "", nil
}
