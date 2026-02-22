package database

import (
	"os"
	"testing"

	"MavenRSS/internal/crypto"
)

func TestEncryptedSettings(t *testing.T) {
	// Create temporary database
	dbFile := "test_encrypted_settings.db"
	defer os.Remove(dbFile)

	db, err := NewDB(dbFile)
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer db.Close()

	err = db.Init()
	if err != nil {
		t.Fatalf("Failed to initialize database: %v", err)
	}

	tests := []struct {
		name  string
		key   string
		value string
	}{
		{"api key", "test_api_key", "sk-1234567890abcdefghijklmnopqrstuvwxyz"},
		{"password", "test_password", "P@ssw0rd!#$%"},
		{"empty value", "test_empty", ""},
		{"unicode", "test_unicode", "密码123"},
		{"long value", "test_long", "very-long-secret-key-with-many-characters-1234567890"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set encrypted setting
			err := db.SetEncryptedSetting(tt.key, tt.value)
			if err != nil {
				t.Fatalf("SetEncryptedSetting() error = %v", err)
			}

			// Get encrypted setting
			retrieved, err := db.GetEncryptedSetting(tt.key)
			if err != nil {
				t.Fatalf("GetEncryptedSetting() error = %v", err)
			}

			// Should match original value
			if retrieved != tt.value {
				t.Errorf("GetEncryptedSetting() = %v, want %v", retrieved, tt.value)
			}

			// Verify the stored value is actually encrypted (unless empty)
			if tt.value != "" {
				storedValue, _ := db.GetSetting(tt.key)
				if !crypto.IsEncrypted(storedValue) {
					t.Errorf("Stored value should be encrypted, got plain text: %v", storedValue)
				}
				if storedValue == tt.value {
					t.Errorf("Stored value should not equal plain text value")
				}
			}
		})
	}
}

func TestMigrationFromPlainText(t *testing.T) {
	// Create temporary database
	dbFile := "test_migration.db"
	defer os.Remove(dbFile)

	db, err := NewDB(dbFile)
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer db.Close()

	err = db.Init()
	if err != nil {
		t.Fatalf("Failed to initialize database: %v", err)
	}

	// Store plain text values (simulating old version)
	plainTextValues := map[string]string{
		"deepl_api_key":    "plain-text-api-key-123",
		"baidu_secret_key": "plain-baidu-secret",
		"proxy_password":   "plain-proxy-pass",
	}

	for key, value := range plainTextValues {
		err := db.SetSetting(key, value)
		if err != nil {
			t.Fatalf("Failed to set plain text setting %s: %v", key, err)
		}
	}

	// Now retrieve using encrypted method (should trigger migration)
	for key, expectedValue := range plainTextValues {
		retrieved, err := db.GetEncryptedSetting(key)
		if err != nil {
			t.Errorf("GetEncryptedSetting(%s) error = %v", key, err)
		}

		// Should get back the original plain text value
		if retrieved != expectedValue {
			t.Errorf("GetEncryptedSetting(%s) = %v, want %v", key, retrieved, expectedValue)
		}

		// After first read, the value should now be encrypted in the database
		storedValue, _ := db.GetSetting(key)
		if !crypto.IsEncrypted(storedValue) {
			t.Errorf("After migration, %s should be encrypted in database", key)
		}

		// Second read should still work and return the same value
		retrieved2, err := db.GetEncryptedSetting(key)
		if err != nil {
			t.Errorf("Second GetEncryptedSetting(%s) error = %v", key, err)
		}
		if retrieved2 != expectedValue {
			t.Errorf("Second GetEncryptedSetting(%s) = %v, want %v", key, retrieved2, expectedValue)
		}
	}
}

func TestGetEncryptedSettingNotFound(t *testing.T) {
	// Create temporary database
	dbFile := "test_not_found.db"
	defer os.Remove(dbFile)

	db, err := NewDB(dbFile)
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer db.Close()

	err = db.Init()
	if err != nil {
		t.Fatalf("Failed to initialize database: %v", err)
	}

	// Try to get a non-existent key
	_, err = db.GetEncryptedSetting("non_existent_key_xyz")
	if err == nil {
		t.Error("GetEncryptedSetting() should return error for non-existent key")
	}
}

func TestSetEncryptedSettingUpdate(t *testing.T) {
	// Create temporary database
	dbFile := "test_update.db"
	defer os.Remove(dbFile)

	db, err := NewDB(dbFile)
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer db.Close()

	err = db.Init()
	if err != nil {
		t.Fatalf("Failed to initialize database: %v", err)
	}

	key := "test_update_key"
	value1 := "original-secret-123"
	value2 := "updated-secret-456"

	// Set initial value
	err = db.SetEncryptedSetting(key, value1)
	if err != nil {
		t.Fatalf("SetEncryptedSetting() error = %v", err)
	}

	// Verify initial value
	retrieved, _ := db.GetEncryptedSetting(key)
	if retrieved != value1 {
		t.Errorf("Initial value = %v, want %v", retrieved, value1)
	}

	// Update to new value
	err = db.SetEncryptedSetting(key, value2)
	if err != nil {
		t.Fatalf("SetEncryptedSetting() update error = %v", err)
	}

	// Verify updated value
	retrieved, _ = db.GetEncryptedSetting(key)
	if retrieved != value2 {
		t.Errorf("Updated value = %v, want %v", retrieved, value2)
	}
}

func BenchmarkSetEncryptedSetting(b *testing.B) {
	dbFile := "bench_set.db"
	defer os.Remove(dbFile)

	db, _ := NewDB(dbFile)
	defer db.Close()
	db.Init()

	value := "sk-1234567890abcdefghijklmnopqrstuvwxyz"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = db.SetEncryptedSetting("bench_key", value)
	}
}

func BenchmarkGetEncryptedSetting(b *testing.B) {
	dbFile := "bench_get.db"
	defer os.Remove(dbFile)

	db, _ := NewDB(dbFile)
	defer db.Close()
	db.Init()

	value := "sk-1234567890abcdefghijklmnopqrstuvwxyz"
	db.SetEncryptedSetting("bench_key", value)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = db.GetEncryptedSetting("bench_key")
	}
}
