package database

import (
	"database/sql"

	"MrRSS/internal/crypto"
	"fmt"
	"log"
)

// GetSettingWithFallback retrieves a setting, preferring user setting if available, falling back to global.
func (db *DB) GetSettingWithFallback(userID int64, key string) (string, error) {
	if userID > 0 {
		value, err := db.GetSettingForUser(userID, key)
		if err == nil && value != "" {
			return value, nil
		}
	}
	return db.GetSetting(key)
}

// GetEncryptedSettingWithFallback retrieves an encrypted setting, preferring user setting if available, falling back to global.
func (db *DB) GetEncryptedSettingWithFallback(userID int64, key string) (string, error) {
	if userID > 0 {
		value, err := db.GetEncryptedSettingForUser(userID, key)
		if err == nil && value != "" {
			return value, nil
		}
	}
	return db.GetEncryptedSetting(key)
}

// GetSetting retrieves a setting value by key.
func (db *DB) GetSetting(key string) (string, error) {
	db.WaitForReady()
	var value string
	err := db.QueryRow("SELECT value FROM settings WHERE key = ?", key).Scan(&value)
	if err == sql.ErrNoRows {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return value, nil
}

// GetSettingForUser retrieves a setting value by key for a specific user.
func (db *DB) GetSettingForUser(userID int64, key string) (string, error) {
	db.WaitForReady()
	var value string
	err := db.QueryRow("SELECT value FROM user_settings WHERE user_id = ? AND key = ?", userID, key).Scan(&value)
	if err == sql.ErrNoRows {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return value, nil
}

// SetSetting stores a setting value.
func (db *DB) SetSetting(key, value string) error {
	db.WaitForReady()
	_, err := db.Exec("INSERT OR REPLACE INTO settings (key, value) VALUES (?, ?)", key, value)
	return err
}

// SetSettingForUser stores a setting value for a specific user.
func (db *DB) SetSettingForUser(userID int64, key, value string) error {
	db.WaitForReady()
	_, err := db.Exec("INSERT OR REPLACE INTO user_settings (user_id, key, value) VALUES (?, ?, ?)", userID, key, value)
	return err
}

// GetEncryptedSetting retrieves and decrypts a sensitive setting value.
// If the value is not encrypted (plain text), it will be automatically encrypted
// and stored back to support migration from old versions.
func (db *DB) GetEncryptedSetting(key string) (string, error) {
	db.WaitForReady()

	// Get the stored value
	storedValue, err := db.GetSetting(key)
	if err != nil {
		return "", err
	}

	// Empty value - return as is
	if storedValue == "" {
		return "", nil
	}

	// Check if the value is already encrypted
	if crypto.IsEncrypted(storedValue) {
		// Decrypt and return
		decrypted, err := crypto.Decrypt(storedValue)
		if err != nil {
			return "", fmt.Errorf("failed to decrypt setting %s: %w", key, err)
		}
		return decrypted, nil
	}

	// Value is plain text - migrate it to encrypted format
	log.Printf("Migrating plain text setting to encrypted storage")

	// Encrypt the plain text value
	encrypted, err := crypto.Encrypt(storedValue)
	if err != nil {
		// If encryption fails, return an error to the caller
		log.Printf("Warning: Failed to encrypt setting during migration: %v", err)
		return "", fmt.Errorf("failed to encrypt setting during migration: %w", err)
	}

	// Store the encrypted value back
	if err := db.SetSetting(key, encrypted); err != nil {
		// If storage fails, return an error to the caller
		log.Printf("Warning: Failed to store encrypted setting: %v", err)
		return "", fmt.Errorf("failed to store encrypted setting: %w", err)
	}

	// Return the original plain text value
	return storedValue, nil
}

// GetEncryptedSettingForUser retrieves and decrypts a sensitive setting value for a specific user.
func (db *DB) GetEncryptedSettingForUser(userID int64, key string) (string, error) {
	db.WaitForReady()

	// Get the stored value
	storedValue, err := db.GetSettingForUser(userID, key)
	if err != nil {
		return "", err
	}

	// Empty value - return as is
	if storedValue == "" {
		return "", nil
	}

	// Check if the value is already encrypted
	if crypto.IsEncrypted(storedValue) {
		// Decrypt and return
		decrypted, err := crypto.Decrypt(storedValue)
		if err != nil {
			return "", fmt.Errorf("failed to decrypt setting %s: %w", key, err)
		}
		return decrypted, nil
	}

	// Value is plain text - migrate it to encrypted format
	log.Printf("Migrating plain text setting to encrypted storage for user %d", userID)

	// Encrypt the plain text value
	encrypted, err := crypto.Encrypt(storedValue)
	if err != nil {
		// If encryption fails, return an error to the caller
		log.Printf("Warning: Failed to encrypt setting during migration: %v", err)
		return "", fmt.Errorf("failed to encrypt setting during migration: %w", err)
	}

	// Store the encrypted value back
	if err := db.SetSettingForUser(userID, key, encrypted); err != nil {
		// If storage fails, return an error to the caller
		log.Printf("Warning: Failed to store encrypted setting: %v", err)
		return "", fmt.Errorf("failed to store encrypted setting: %w", err)
	}

	// Return the original plain text value
	return storedValue, nil
}

// SetEncryptedSetting encrypts and stores a sensitive setting value.
func (db *DB) SetEncryptedSetting(key, value string) error {
	db.WaitForReady()

	// Empty value - store as is
	if value == "" {
		return db.SetSetting(key, value)
	}

	// Encrypt the value
	encrypted, err := crypto.Encrypt(value)
	if err != nil {
		return fmt.Errorf("failed to encrypt setting %s: %w", key, err)
	}

	// Store the encrypted value
	return db.SetSetting(key, encrypted)
}

// SetEncryptedSettingForUser encrypts and stores a sensitive setting value for a specific user.
func (db *DB) SetEncryptedSettingForUser(userID int64, key, value string) error {
	db.WaitForReady()

	// Empty value - store as is
	if value == "" {
		return db.SetSettingForUser(userID, key, value)
	}

	// Encrypt the value
	encrypted, err := crypto.Encrypt(value)
	if err != nil {
		return fmt.Errorf("failed to encrypt setting %s: %w", key, err)
	}

	// Store the encrypted value
	return db.SetSettingForUser(userID, key, encrypted)
}
