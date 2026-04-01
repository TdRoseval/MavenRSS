package settings

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"MavenRSS/internal/api/core"
	"MavenRSS/internal/auth"
	"MavenRSS/internal/feed"
	"MavenRSS/internal/middleware"
	"MavenRSS/internal/models"
	"MavenRSS/internal/store/sqlite"
)

func setupHandlerWithDB(t *testing.T) *core.Handler {
	t.Helper()
	db, err := sqlite.NewDB(":memory:")
	if err != nil {
		t.Fatalf("NewDB error: %v", err)
	}
	if err := db.Init(); err != nil {
		t.Fatalf("db Init error: %v", err)
	}
	return core.NewHandler(db, nil, nil, nil)
}

func TestHandleSettings_GET(t *testing.T) {
	h := setupHandlerWithDB(t)

	// Set a custom value
	h.DB.SetSetting("language", "xx-YY")

	req := httptest.NewRequest(http.MethodGet, "/api/settings", nil)
	w := httptest.NewRecorder()

	HandleSettings(h, w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", resp.StatusCode)
	}

	var data map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if data["language"] != "xx-YY" {
		t.Fatalf("expected language xx-YY, got %s", data["language"])
	}
}

func TestHandleSettings_POST(t *testing.T) {
	h := setupHandlerWithDB(t)

	payload := map[string]string{
		"update_interval":     "15",
		"translation_enabled": "true",
		"deepl_api_key":       "deadbeef",
	}
	body, _ := json.Marshal(payload)

	req := httptest.NewRequest(http.MethodPost, "/api/settings", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	HandleSettings(h, w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", resp.StatusCode)
	}

	// Verify settings saved
	v, _ := h.DB.GetSetting("update_interval")
	if v != "15" {
		t.Fatalf("expected update_interval 15, got %s", v)
	}

	v2, _ := h.DB.GetSetting("translation_enabled")
	if v2 != "true" {
		t.Fatalf("expected translation_enabled true, got %s", v2)
	}

	// Encrypted key should be retrievable via GetEncryptedSetting
	dec, err := h.DB.GetEncryptedSetting("deepl_api_key")
	if err != nil {
		t.Fatalf("GetEncryptedSetting error: %v", err)
	}
	if dec != "deadbeef" {
		t.Fatalf("expected deepl_api_key decrypted to be deadbeef, got %s", dec)
	}
}

func TestHandleSettings_POSTRejectsFrozenAISettings(t *testing.T) {
	db, err := sqlite.NewDB(":memory:")
	if err != nil {
		t.Fatalf("NewDB error: %v", err)
	}
	if err := db.Init(); err != nil {
		t.Fatalf("db Init error: %v", err)
	}

	fetcher := feed.NewFetcher(db)
	defer fetcher.Stop()

	h := core.NewHandler(db, fetcher, nil, nil)

	if err := db.SetSettingForUser(1, "ai_enhanced_mode", "true"); err != nil {
		t.Fatalf("SetSettingForUser(ai_enhanced_mode) error: %v", err)
	}

	feedID, err := db.AddFeedForUser(1, &models.Feed{
		Title:           "Test Feed",
		URL:             "https://example.com/feed",
		Type:            "rss",
		RefreshInterval: 60,
	})
	if err != nil {
		t.Fatalf("AddFeedForUser error: %v", err)
	}

	if _, err := db.Exec(
		`INSERT INTO articles (user_id, feed_id, title, url, published_at, summary, unique_id)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		1,
		feedID,
		"Pending Article",
		"https://example.com/articles/pending",
		time.Now(),
		"summary content",
		"pending-article",
	); err != nil {
		t.Fatalf("insert article error: %v", err)
	}

	payload := map[string]string{
		"ai_embedding_models": `[{"modelname":"embed-v1"}]`,
	}
	body, _ := json.Marshal(payload)

	req := httptest.NewRequest(http.MethodPost, "/api/settings", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(
		context.WithValue(
			req.Context(),
			middleware.UserContextKey,
			&auth.Claims{UserID: 1, Username: "tester", Role: "user"},
		),
	)

	w := httptest.NewRecorder()
	HandleSettings(h, w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("expected 409 Conflict, got %d", resp.StatusCode)
	}

	var data struct {
		Success bool `json:"success"`
		Error   struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if data.Error.Message == "" {
		t.Fatal("expected conflict error message, got empty string")
	}
}
