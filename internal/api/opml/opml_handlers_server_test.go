//go:build server

package opml

import (
	"bytes"
	"context"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	corepkg "MavenRSS/internal/api/core"
	"MavenRSS/internal/auth"
	"MavenRSS/internal/middleware"
	"MavenRSS/internal/models"
	"MavenRSS/internal/store/sqlite"
)

func seedServerUsers(t *testing.T, db *sqlite.DB) (int64, int64) {
	t.Helper()

	users := []*models.User{
		{
			Username:     "user1",
			Email:        "user1@example.com",
			PasswordHash: "hash",
			Role:         models.RoleUser,
			Status:       "active",
		},
		{
			Username:     "user2",
			Email:        "user2@example.com",
			PasswordHash: "hash",
			Role:         models.RoleAdmin,
			Status:       "active",
		},
	}

	var ids [2]int64
	for i, user := range users {
		id, err := db.CreateUser(user)
		if err != nil {
			t.Fatalf("CreateUser(%d) failed: %v", i+1, err)
		}
		ids[i] = id
	}

	return ids[0], ids[1]
}

func withServerUser(req *http.Request, userID int64) *http.Request {
	claims := &auth.Claims{UserID: userID, Username: "tester", Role: "admin"}
	return req.WithContext(context.WithValue(req.Context(), middleware.UserContextKey, claims))
}

func TestHandleOPMLImport_ServerDoesNotOverwriteAnotherUsersFeed(t *testing.T) {
	xmlData := `<?xml version="1.0"?>
<opml version="1.0">
  <head><title>Test</title></head>
  <body>
    <outline type="rss" text="Imported Feed" title="Imported Feed" xmlUrl="https://example.com/shared.xml" />
  </body>
</opml>`

	db, err := sqlite.NewDB(":memory:")
	if err != nil {
		t.Fatalf("failed to create db: %v", err)
	}
	if err := db.Init(); err != nil {
		t.Fatalf("failed to init db: %v", err)
	}
	existingUserID, importUserID := seedServerUsers(t, db)

	if _, err := db.AddFeedForUser(existingUserID, &models.Feed{
		UserID: existingUserID,
		Title:  "Existing Feed",
		URL:    "https://example.com/shared.xml",
	}); err != nil {
		t.Fatalf("seed AddFeedForUser failed: %v", err)
	}

	h := &corepkg.Handler{DB: db}

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", "subscriptions.opml")
	if err != nil {
		t.Fatalf("CreateFormFile failed: %v", err)
	}
	if _, err := part.Write([]byte(xmlData)); err != nil {
		t.Fatalf("writing multipart body failed: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("closing multipart writer failed: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/opml/import", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req = withServerUser(req, importUserID)
	rr := httptest.NewRecorder()

	HandleOPMLImport(h, rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 got %d", rr.Code)
	}

	var result map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &result); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if got := int(result["imported"].(float64)); got != 1 {
		t.Fatalf("expected imported=1, got %d", got)
	}

	userOneFeeds, err := db.GetFeedsForUser(existingUserID)
	if err != nil {
		t.Fatalf("GetFeedsForUser(%d) failed: %v", existingUserID, err)
	}
	if len(userOneFeeds) != 1 {
		t.Fatalf("expected 1 feed for existing user %d, got %d", existingUserID, len(userOneFeeds))
	}
	if userOneFeeds[0].Title != "Existing Feed" {
		t.Fatalf("expected user 1 feed title to remain unchanged, got %q", userOneFeeds[0].Title)
	}

	userTwoFeeds, err := db.GetFeedsForUser(importUserID)
	if err != nil {
		t.Fatalf("GetFeedsForUser(%d) failed: %v", importUserID, err)
	}
	if len(userTwoFeeds) != 1 {
		t.Fatalf("expected 1 feed for importing user %d, got %d", importUserID, len(userTwoFeeds))
	}
	if userTwoFeeds[0].Title != "Imported Feed" {
		t.Fatalf("expected imported title for user 2, got %q", userTwoFeeds[0].Title)
	}
}
