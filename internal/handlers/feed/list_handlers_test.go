package feed_test

import (
	"encoding/json"
	"net/http/httptest"
	"testing"

	fh "MavenRSS/internal/handlers/feed"
	"MavenRSS/internal/models"
)

func TestHandleFeeds_ReturnsList(t *testing.T) {
	h := setupHandler(t)

	// add two feeds
	if _, err := h.DB.AddFeed(&models.Feed{Title: "a", URL: "http://x/1"}); err != nil {
		t.Fatalf("add feed: %v", err)
	}
	if _, err := h.DB.AddFeed(&models.Feed{Title: "b", URL: "http://x/2"}); err != nil {
		t.Fatalf("add feed: %v", err)
	}

	req := httptest.NewRequest("GET", "/api/feeds", nil)
	w := httptest.NewRecorder()

	fh.HandleFeeds(h, w, req)
	res := w.Result()
	if res.StatusCode != 200 {
		t.Fatalf("expected 200 OK, got %d", res.StatusCode)
	}

	var feeds []models.Feed
	if err := json.NewDecoder(res.Body).Decode(&feeds); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(feeds) < 2 {
		t.Fatalf("expected at least 2 feeds, got %d", len(feeds))
	}
}
