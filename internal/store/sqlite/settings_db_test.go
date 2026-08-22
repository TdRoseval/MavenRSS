package sqlite_test

import (
	"testing"

	"MavenRSS/internal/config"
	"MavenRSS/internal/crypto"
	dbpkg "MavenRSS/internal/store/sqlite"
)

func setupTestDB(t *testing.T) *dbpkg.DB {
	t.Helper()
	db, err := dbpkg.NewDB(":memory:")
	if err != nil {
		t.Fatalf("NewDB() error = %v", err)
	}
	if err := db.Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	return db
}

func TestSetGetSetting(t *testing.T) {
	db := setupTestDB(t)

	key := "test_key"
	val := "value123"

	if err := db.SetSetting(key, val); err != nil {
		t.Fatalf("SetSetting() error = %v", err)
	}

	got, err := db.GetSetting(key)
	if err != nil {
		t.Fatalf("GetSetting() error = %v", err)
	}
	if got != val {
		t.Fatalf("GetSetting() = %q, want %q", got, val)
	}
}

func TestSetEncryptedAndGetEncrypted(t *testing.T) {
	db := setupTestDB(t)

	key := "secret_key"
	secret := "s3cr3t!"

	if err := db.SetEncryptedSetting(key, secret); err != nil {
		t.Fatalf("SetEncryptedSetting() error = %v", err)
	}

	// GetEncryptedSetting should return the original plaintext
	got, err := db.GetEncryptedSetting(key)
	if err != nil {
		t.Fatalf("GetEncryptedSetting() error = %v", err)
	}
	if got != secret {
		t.Fatalf("GetEncryptedSetting() = %q, want %q", got, secret)
	}

	// Underlying stored value should be encrypted
	stored, err := db.GetSetting(key)
	if err != nil {
		t.Fatalf("GetSetting() error = %v", err)
	}
	if !crypto.IsEncrypted(stored) {
		t.Fatalf("expected stored value to be encrypted, got %q", stored)
	}
}

func TestGetEncryptedSettingMigration(t *testing.T) {
	db := setupTestDB(t)

	key := "plain_key"
	plain := "plain-password"

	// Store as plain text using SetSetting to simulate old version
	if err := db.SetSetting(key, plain); err != nil {
		t.Fatalf("SetSetting() error = %v", err)
	}

	// Calling GetEncryptedSetting should return the plain text and migrate to encrypted storage
	got, err := db.GetEncryptedSetting(key)
	if err != nil {
		t.Fatalf("GetEncryptedSetting() error = %v", err)
	}
	if got != plain {
		t.Fatalf("GetEncryptedSetting() = %q, want %q", got, plain)
	}

	// Now the stored value should be encrypted
	stored, err := db.GetSetting(key)
	if err != nil {
		t.Fatalf("GetSetting() error = %v", err)
	}
	if !crypto.IsEncrypted(stored) {
		t.Fatalf("expected migrated stored value to be encrypted, got %q", stored)
	}
}

func TestInitInsertsDefaults(t *testing.T) {
	db := setupTestDB(t)

	// Pick a known default key from config
	key := "language"
	want := config.GetString(key)

	got, err := db.GetSetting(key)
	if err != nil {
		t.Fatalf("GetSetting() error = %v", err)
	}
	if got != want {
		t.Fatalf("default setting %s = %q, want %q", key, got, want)
	}
}

func TestSettingsRevisionIncrementsOnWrite(t *testing.T) {
	db := setupTestDB(t)

	before := dbpkg.SettingsRevision()
	if err := db.SetSetting("rev_global_key", "1"); err != nil {
		t.Fatalf("SetSetting() error = %v", err)
	}
	mid := dbpkg.SettingsRevision()
	if mid <= before {
		t.Fatalf("SettingsRevision() = %d after SetSetting, want > %d", mid, before)
	}

	if err := db.SetSettingForUser(1, "rev_user_key", "2"); err != nil {
		t.Fatalf("SetSettingForUser() error = %v", err)
	}
	after := dbpkg.SettingsRevision()
	if after <= mid {
		t.Fatalf("SettingsRevision() = %d after SetSettingForUser, want > %d", after, mid)
	}
}

func TestDBConnectionPoolSize(t *testing.T) {
	db, err := dbpkg.NewDB(":memory:")
	if err != nil {
		t.Fatalf("NewDB() error = %v", err)
	}
	defer db.Close()

	stats := db.Stats()
	if stats.MaxOpenConnections != 8 {
		t.Fatalf("MaxOpenConnections = %d, want 8", stats.MaxOpenConnections)
	}
}
