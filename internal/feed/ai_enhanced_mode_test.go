package feed

import (
	"testing"

	"MavenRSS/internal/ai"
	"MavenRSS/internal/models"
	"MavenRSS/internal/store/sqlite"
)

func TestShouldProcessUsesUserProfilesWithoutGlobalAIKey(t *testing.T) {
	db := newAIEnhancedModeTestDB(t)

	if _, err := db.CreateAIProfile(&models.AIProfile{
		UserID:   1,
		Name:     "summary",
		APIKey:   "summary-key",
		Endpoint: "https://summary.example.com",
		Model:    "summary-model",
	}); err != nil {
		t.Fatalf("CreateAIProfile summary error: %v", err)
	}

	if _, err := db.CreateAIProfile(&models.AIProfile{
		UserID:   1,
		Name:     "translation",
		APIKey:   "translation-key",
		Endpoint: "https://translation.example.com",
		Model:    "translation-model",
	}); err != nil {
		t.Fatalf("CreateAIProfile translation error: %v", err)
	}

	if _, err := db.CreateAIProfile(&models.AIProfile{
		UserID:   1,
		Name:     "search",
		APIKey:   "search-key",
		Endpoint: "https://search.example.com",
		Model:    "search-model",
	}); err != nil {
		t.Fatalf("CreateAIProfile search error: %v", err)
	}

	if _, err := db.CreateAIProfile(&models.AIProfile{
		UserID:   1,
		Name:     "chat",
		APIKey:   "chat-key",
		Endpoint: "https://chat.example.com",
		Model:    "chat-model",
	}); err != nil {
		t.Fatalf("CreateAIProfile chat error: %v", err)
	}

	mustSetUserSetting(t, db, 1, "ai_enhanced_mode", "true")
	mustSetUserSetting(t, db, 1, "summary_enabled", "true")
	mustSetUserSetting(t, db, 1, "summary_provider", "ai")
	mustSetUserSetting(t, db, 1, "translation_enabled", "true")
	mustSetUserSetting(t, db, 1, "ai_search_enabled", "true")
	mustSetUserSetting(t, db, 1, "ai_chat_enabled", "true")

	if !ShouldProcess(db, 1) {
		t.Fatal("ShouldProcess() = false, want true when user profiles are configured")
	}
}

func TestShouldProcessIgnoresGlobalAIKeyWithoutUserConfig(t *testing.T) {
	db := newAIEnhancedModeTestDB(t)

	mustSetGlobalSetting(t, db, "ai_enhanced_mode", "true")
	mustSetGlobalSetting(t, db, "summary_enabled", "true")
	mustSetGlobalSetting(t, db, "summary_provider", "ai")
	mustSetGlobalSetting(t, db, "translation_enabled", "true")
	mustSetGlobalSetting(t, db, "ai_search_enabled", "true")
	mustSetGlobalSetting(t, db, "ai_chat_enabled", "true")
	mustSetGlobalEncryptedSetting(t, db, "ai_api_key", "global-key")
	mustSetGlobalSetting(t, db, "ai_endpoint", "https://global.example.com")
	mustSetGlobalSetting(t, db, "ai_model", "global-model")

	if ShouldProcess(db, 1) {
		t.Fatal("ShouldProcess() = true, want false when only global AI config exists")
	}
}

func TestGetUserFeatureAIConfigPrefersUserProfileAndFallsBackToUserLegacy(t *testing.T) {
	db := newAIEnhancedModeTestDB(t)

	profileID, err := db.CreateAIProfile(&models.AIProfile{
		UserID:   1,
		Name:     "summary",
		APIKey:   "profile-key",
		Endpoint: "https://profile.example.com",
		Model:    "profile-model",
	})
	if err != nil {
		t.Fatalf("CreateAIProfile error: %v", err)
	}

	mustSetUserSetting(t, db, 1, "ai_summary_profile_id", "999999")

	cfg, err := getUserFeatureAIConfig(db, 1, ai.FeatureSummary)
	if err != nil {
		t.Fatalf("getUserFeatureAIConfig invalid selected profile error: %v", err)
	}
	if cfg == nil || cfg.APIKey != "profile-key" || cfg.Endpoint != "https://profile.example.com" || cfg.Model != "profile-model" {
		t.Fatalf("getUserFeatureAIConfig() with default profile = %#v, want default user profile config", cfg)
	}

	mustSetUserSetting(t, db, 1, "ai_summary_profile_id", "")
	if err := db.DeleteAIProfileForUser(1, profileID); err != nil {
		t.Fatalf("DeleteAIProfileForUser error: %v", err)
	}
	mustSetUserEncryptedSetting(t, db, 1, "ai_api_key", "legacy-key")
	mustSetUserSetting(t, db, 1, "ai_endpoint", "https://legacy.example.com")
	mustSetUserSetting(t, db, 1, "ai_model", "legacy-model")
	mustSetUserSetting(t, db, 1, "ai_custom_headers", `{"X-Test":"1"}`)

	cfg, err = getUserFeatureAIConfig(db, 1, ai.FeatureSummary)
	if err != nil {
		t.Fatalf("getUserFeatureAIConfig legacy error: %v", err)
	}
	if cfg == nil || cfg.APIKey != "legacy-key" || cfg.Endpoint != "https://legacy.example.com" || cfg.Model != "legacy-model" || cfg.CustomHeaders != `{"X-Test":"1"}` {
		t.Fatalf("getUserFeatureAIConfig() legacy = %#v, want user legacy config", cfg)
	}
}

func newAIEnhancedModeTestDB(t *testing.T) *sqlite.DB {
	t.Helper()

	db, err := sqlite.NewDB(":memory:")
	if err != nil {
		t.Fatalf("NewDB error: %v", err)
	}
	if err := db.Init(); err != nil {
		t.Fatalf("db Init error: %v", err)
	}

	return db
}

func mustSetGlobalSetting(t *testing.T, db *sqlite.DB, key, value string) {
	t.Helper()
	if err := db.SetSetting(key, value); err != nil {
		t.Fatalf("SetSetting(%s) error: %v", key, err)
	}
}

func mustSetUserSetting(t *testing.T, db *sqlite.DB, userID int64, key, value string) {
	t.Helper()
	if err := db.SetSettingForUser(userID, key, value); err != nil {
		t.Fatalf("SetSettingForUser(%s) error: %v", key, err)
	}
}

func mustSetGlobalEncryptedSetting(t *testing.T, db *sqlite.DB, key, value string) {
	t.Helper()
	if err := db.SetEncryptedSetting(key, value); err != nil {
		t.Fatalf("SetEncryptedSetting(%s) error: %v", key, err)
	}
}

func mustSetUserEncryptedSetting(t *testing.T, db *sqlite.DB, userID int64, key, value string) {
	t.Helper()
	if err := db.SetEncryptedSettingForUser(userID, key, value); err != nil {
		t.Fatalf("SetEncryptedSettingForUser(%s) error: %v", key, err)
	}
}
