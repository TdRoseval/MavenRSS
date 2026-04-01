package ai

import (
	"testing"

	"MavenRSS/internal/models"
	"MavenRSS/internal/store/sqlite"
)

func TestGetConfigForFeatureForUserUsesUserProfileOnly(t *testing.T) {
	db := newProfileProviderTestDB(t)
	provider := NewProfileProvider(db)

	if _, err := db.CreateUser(&models.User{
		Username:     "user2",
		Email:        "user2@example.com",
		PasswordHash: "hash",
		Role:         models.RoleUser,
		Status:       "active",
	}); err != nil {
		t.Fatalf("CreateUser error: %v", err)
	}

	if _, err := db.CreateAIProfile(&models.AIProfile{
		UserID:   1,
		Name:     "summary",
		APIKey:   "user-key",
		Endpoint: "https://user.example.com",
		Model:    "user-model",
	}); err != nil {
		t.Fatalf("CreateAIProfile user error: %v", err)
	}

	if _, err := db.CreateAIProfile(&models.AIProfile{
		UserID:   2,
		Name:     "other",
		APIKey:   "other-key",
		Endpoint: "https://other.example.com",
		Model:    "other-model",
	}); err != nil {
		t.Fatalf("CreateAIProfile other error: %v", err)
	}

	cfg, err := provider.GetConfigForFeatureForUser(1, FeatureSummary)
	if err != nil {
		t.Fatalf("GetConfigForFeatureForUser error: %v", err)
	}
	if cfg == nil || cfg.APIKey != "user-key" || cfg.Endpoint != "https://user.example.com" || cfg.Model != "user-model" {
		t.Fatalf("GetConfigForFeatureForUser() = %#v, want user-scoped profile", cfg)
	}
}

func TestGetConfigForFeatureForUserDoesNotFallbackToGlobalSettings(t *testing.T) {
	db := newProfileProviderTestDB(t)
	provider := NewProfileProvider(db)

	if err := db.SetEncryptedSetting("ai_api_key", "global-key"); err != nil {
		t.Fatalf("SetEncryptedSetting error: %v", err)
	}
	if err := db.SetSetting("ai_endpoint", "https://global.example.com"); err != nil {
		t.Fatalf("SetSetting endpoint error: %v", err)
	}
	if err := db.SetSetting("ai_model", "global-model"); err != nil {
		t.Fatalf("SetSetting model error: %v", err)
	}

	cfg, err := provider.GetConfigForFeatureForUser(1, FeatureSummary)
	if err != nil {
		t.Fatalf("GetConfigForFeatureForUser error: %v", err)
	}
	if cfg != nil {
		t.Fatalf("GetConfigForFeatureForUser() = %#v, want nil without user-scoped config", cfg)
	}
}

func TestGetConfigForFeatureForUserFallsBackToUserLegacyOnly(t *testing.T) {
	db := newProfileProviderTestDB(t)
	provider := NewProfileProvider(db)

	if err := db.SetEncryptedSettingForUser(1, "ai_api_key", "legacy-key"); err != nil {
		t.Fatalf("SetEncryptedSettingForUser error: %v", err)
	}
	if err := db.SetSettingForUser(1, "ai_endpoint", "https://legacy.example.com"); err != nil {
		t.Fatalf("SetSettingForUser endpoint error: %v", err)
	}
	if err := db.SetSettingForUser(1, "ai_model", "legacy-model"); err != nil {
		t.Fatalf("SetSettingForUser model error: %v", err)
	}
	if err := db.SetSettingForUser(1, "ai_custom_headers", `{"X-Test":"1"}`); err != nil {
		t.Fatalf("SetSettingForUser headers error: %v", err)
	}

	cfg, err := provider.GetConfigForFeatureForUser(1, FeatureSummary)
	if err != nil {
		t.Fatalf("GetConfigForFeatureForUser error: %v", err)
	}
	if cfg == nil || cfg.APIKey != "legacy-key" || cfg.Endpoint != "https://legacy.example.com" || cfg.Model != "legacy-model" || cfg.CustomHeaders != `{"X-Test":"1"}` {
		t.Fatalf("GetConfigForFeatureForUser() = %#v, want user legacy config", cfg)
	}
}

func TestGetConfigForFeatureForUserSupportsFusionProfile(t *testing.T) {
	db := newProfileProviderTestDB(t)
	provider := NewProfileProvider(db)

	fusionProfileID, err := db.CreateAIProfile(&models.AIProfile{
		UserID:         1,
		Name:           "fusion",
		APIKey:         "fusion-key",
		Endpoint:       "https://fusion.example.com",
		Model:          "fusion-model",
		CustomHeaders:  `{"X-Fusion":"1"}`,
		UseGlobalProxy: true,
	})
	if err != nil {
		t.Fatalf("CreateAIProfile fusion error: %v", err)
	}

	if err := db.SetSettingForUser(1, "ai_fusion_profile_id", "999999"); err != nil {
		t.Fatalf("SetSettingForUser invalid fusion profile error: %v", err)
	}

	cfg, err := provider.GetConfigForFeatureForUser(1, FeatureFusion)
	if err != nil {
		t.Fatalf("GetConfigForFeatureForUser fusion fallback error: %v", err)
	}
	if cfg == nil || cfg.APIKey != "fusion-key" || cfg.Endpoint != "https://fusion.example.com" || cfg.Model != "fusion-model" || cfg.CustomHeaders != `{"X-Fusion":"1"}` {
		t.Fatalf("GetConfigForFeatureForUser() invalid fusion selected = %#v, want fallback user profile", cfg)
	}

	if err := db.SetSettingForUser(1, "ai_fusion_profile_id", "0"); err != nil {
		t.Fatalf("SetSettingForUser zero fusion profile error: %v", err)
	}

	cfg, err = provider.GetConfigForFeatureForUser(1, FeatureFusion)
	if err != nil {
		t.Fatalf("GetConfigForFeatureForUser fusion zero error: %v", err)
	}
	if cfg == nil || cfg.APIKey != "fusion-key" || cfg.Endpoint != "https://fusion.example.com" || cfg.Model != "fusion-model" || cfg.CustomHeaders != `{"X-Fusion":"1"}` {
		t.Fatalf("GetConfigForFeatureForUser() zero fusion selected = %#v, want fallback user profile", cfg)
	}

	if err := db.SetDefaultAIProfileForUser(1, fusionProfileID); err != nil {
		t.Fatalf("SetDefaultAIProfileForUser fusion error: %v", err)
	}
	if err := db.SetSettingForUser(1, "ai_fusion_profile_id", "9999999"); err != nil {
		t.Fatalf("SetSettingForUser invalid fusion profile error: %v", err)
	}

	cfg, err = provider.GetConfigForFeatureForUser(1, FeatureFusion)
	if err != nil {
		t.Fatalf("GetConfigForFeatureForUser fusion default error: %v", err)
	}
	if cfg == nil || cfg.APIKey != "fusion-key" || cfg.Endpoint != "https://fusion.example.com" || cfg.Model != "fusion-model" || cfg.CustomHeaders != `{"X-Fusion":"1"}` {
		t.Fatalf("GetConfigForFeatureForUser() fusion default = %#v, want selected fusion profile config", cfg)
	}
}

func newProfileProviderTestDB(t *testing.T) *sqlite.DB {
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
