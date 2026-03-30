package summary

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"MavenRSS/internal/api/core"
	"MavenRSS/internal/feed"
	"MavenRSS/internal/models"
	"MavenRSS/internal/store/sqlite"
)

func TestHandleSummarizeArticle_MethodNotAllowed(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/summary/article", nil)
	rr := httptest.NewRecorder()

	HandleSummarizeArticle(nil, rr, req)

	if rr.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected %d got %d", http.StatusMethodNotAllowed, rr.Code)
	}
}

func TestHandleSummarizeArticle_InvalidLength(t *testing.T) {
	payload := []byte(`{"article_id": 1, "length": "bad"}`)
	req := httptest.NewRequest(http.MethodPost, "/summary/article", bytes.NewReader(payload))
	rr := httptest.NewRecorder()

	// Use a nil handler pointer; length validation happens before DB access
	HandleSummarizeArticle(nil, rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected %d got %d", http.StatusBadRequest, rr.Code)
	}
}

// Test successful summarization using the local summarizer and a mocked feed parser.
func TestHandleSummarizeArticle_Success(t *testing.T) {
	// Setup in-memory DB
	db, err := sqlite.NewDB(":memory:")
	if err != nil {
		t.Fatalf("failed to create db: %v", err)
	}
	if err := db.Init(); err != nil {
		t.Fatalf("db init failed: %v", err)
	}

	// Add feed
	feedID, err := db.AddFeed(&models.Feed{Title: "T", URL: "http://example.com/feed"})
	if err != nil {
		t.Fatalf("AddFeed failed: %v", err)
	}

	// Add article
	art := &models.Article{
		FeedID:      feedID,
		Title:       "A",
		URL:         "http://example.com/article/1",
		PublishedAt: time.Now(),
	}
	if err := db.SaveArticle(art); err != nil {
		t.Fatalf("SaveArticle failed: %v", err)
	}

	// Get the inserted article ID
	var articleID int64
	if err := db.QueryRow("SELECT id FROM articles WHERE url = ?", art.URL).Scan(&articleID); err != nil {
		t.Fatalf("failed to query article id: %v", err)
	}

	// Use provided content to avoid network fetching/parsing in tests.
	f := feed.NewFetcher(db)
	h := core.NewHandler(db, f, nil, nil)

	content := "This is a test content. It has multiple sentences. Useful for summarization."
	payload := []byte(`{"article_id": ` + fmt.Sprintf("%d", articleID) + `, "length": "short", "content": "` + content + `"}`)
	req := httptest.NewRequest(http.MethodPost, "/summary/article", bytes.NewReader(payload))
	rr := httptest.NewRecorder()

	HandleSummarizeArticle(h, rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected %d got %d; body: %s", http.StatusOK, rr.Code, rr.Body.String())
	}
	if !bytes.Contains(rr.Body.Bytes(), []byte("summary")) {
		t.Fatalf("expected response to contain summary, got: %s", rr.Body.String())
	}
}

// Note: We intentionally avoid mocking feed parsing here by providing `content`
// in the request payload, which bypasses article content fetching.
