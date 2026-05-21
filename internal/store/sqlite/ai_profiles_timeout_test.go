package sqlite

import (
	"testing"

	"MavenRSS/internal/models"
)

func TestAIProfileCRUDPreservesTimeoutSeconds(t *testing.T) {
	db, err := NewDB(":memory:")
	if err != nil {
		t.Fatalf("NewDB error: %v", err)
	}
	if err := db.Init(); err != nil {
		t.Fatalf("Init error: %v", err)
	}

	id, err := db.CreateAIProfile(&models.AIProfile{
		UserID:         1,
		Name:           "slow",
		APIKey:         "key",
		Endpoint:       "https://api.example.com/v1/chat/completions",
		Model:          "model",
		TimeoutSeconds: 600,
		UseGlobalProxy: true,
	})
	if err != nil {
		t.Fatalf("CreateAIProfile error: %v", err)
	}

	profile, err := db.GetAIProfileForUser(1, id)
	if err != nil {
		t.Fatalf("GetAIProfileForUser error: %v", err)
	}
	if profile == nil || profile.TimeoutSeconds != 600 {
		t.Fatalf("created profile timeout = %#v, want 600", profile)
	}

	profile.APIKey = ""
	profile.TimeoutSeconds = 900
	if err := db.UpdateAIProfile(profile); err != nil {
		t.Fatalf("UpdateAIProfile error: %v", err)
	}

	updated, err := db.GetAIProfileForUser(1, id)
	if err != nil {
		t.Fatalf("GetAIProfileForUser updated error: %v", err)
	}
	if updated == nil || updated.TimeoutSeconds != 900 {
		t.Fatalf("updated profile timeout = %#v, want 900", updated)
	}
	if updated.APIKey != "key" {
		t.Fatalf("API key = %q, want existing key preserved", updated.APIKey)
	}
}
