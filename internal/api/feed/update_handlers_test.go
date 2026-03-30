package feed_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http/httptest"
	"testing"

	"MavenRSS/internal/auth"
	fh "MavenRSS/internal/api/feed"
	"MavenRSS/internal/middleware"
	"MavenRSS/internal/models"
)

// reuse setupHandler from feed_handlers_test.go

func TestHandleUpdateFeed_ValidAndInvalid(t *testing.T) {
	h := setupHandler(t)

	// create a feed
	id, err := h.DB.AddFeed(&models.Feed{Title: "old", URL: "http://example.com/feed"})
	if err != nil {
		t.Fatalf("AddFeed error: %v", err)
	}

	// valid update
	payload := map[string]interface{}{"id": id, "title": "new", "url": "http://example.com/feed", "category": "c"}
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest("POST", "/api/feeds/update", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	claims := &auth.Claims{UserID: 1, Username: "admin", Role: "admin"}
	req = req.WithContext(context.WithValue(req.Context(), middleware.UserContextKey, claims))

	fh.HandleUpdateFeed(h, w, req)
	if w.Result().StatusCode != 200 {
		t.Fatalf("expected 200 OK for valid update, got %d", w.Result().StatusCode)
	}

	// invalid payload
	badReq := httptest.NewRequest("POST", "/api/feeds/update", bytes.NewReader([]byte("notjson")))
	badReq = badReq.WithContext(context.WithValue(badReq.Context(), middleware.UserContextKey, claims))
	w2 := httptest.NewRecorder()
	fh.HandleUpdateFeed(h, w2, badReq)
	if w2.Result().StatusCode != 400 {
		t.Fatalf("expected 400 for invalid payload, got %d", w2.Result().StatusCode)
	}
}
