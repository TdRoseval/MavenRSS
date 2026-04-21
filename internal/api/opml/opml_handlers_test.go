//go:build !server

package opml

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	corepkg "MavenRSS/internal/api/core"
	"MavenRSS/internal/auth"
	"MavenRSS/internal/feed"
	"MavenRSS/internal/middleware"
	"MavenRSS/internal/models"
	"MavenRSS/internal/store/sqlite"
)

func seedUsers(t *testing.T, db *sqlite.DB) (int64, int64) {
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

func withUser(req *http.Request, userID int64) *http.Request {
	claims := &auth.Claims{UserID: userID, Username: "tester", Role: "admin"}
	return req.WithContext(context.WithValue(req.Context(), middleware.UserContextKey, claims))
}

func TestHandleOPMLImport_RawBody(t *testing.T) {
	xmlData := `<?xml version="1.0"?>
<opml version="1.0">
  <head><title>Test</title></head>
  <body>
    <outline text="Tech" title="Tech">
      <outline type="rss" text="Hacker News" title="Hacker News" xmlUrl="https://news.ycombinator.com/rss" />
    </outline>
    <outline type="rss" text="Go Blog" title="Go Blog" xmlUrl="https://blog.golang.org/feed.atom" />
  </body>
</opml>`

	// Use a real fetcher that writes to an in-memory DB (ImportSubscription uses DB.AddFeed)
	var importUserID int64
	db := func() *sqlite.DB {
		db, err := sqlite.NewDB(":memory:")
		if err != nil {
			t.Fatalf("failed to create db: %v", err)
		}
		if err := db.Init(); err != nil {
			t.Fatalf("failed to init db: %v", err)
		}
		_, importUserID = seedUsers(t, db)
		return db
	}()

	f := feed.NewFetcher(db)
	h := &corepkg.Handler{DB: db, Fetcher: f}

	req := httptest.NewRequest(http.MethodPost, "/opml/import", strings.NewReader(xmlData))
	req = withUser(req, importUserID)
	rr := httptest.NewRecorder()

	HandleOPMLImport(h, rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 got %d", rr.Code)
	}

	// Verify feeds were added
	feeds, err := db.GetFeeds()
	if err != nil {
		t.Fatalf("GetFeeds failed: %v", err)
	}
	if len(feeds) != 2 {
		t.Fatalf("expected 2 feeds in DB, got %d", len(feeds))
	}
	for _, feed := range feeds {
		if feed.UserID != importUserID {
			t.Fatalf("expected imported feed to belong to user %d, got user %d", importUserID, feed.UserID)
		}
	}
}

func TestHandleOPMLImport_XPathFeed(t *testing.T) {
	xmlData := `<?xml version="1.0"?>
<opml version="1.0">
  <head><title>Test XPath</title></head>
  <body>
    <outline text="CH3NYANG&#39;S BLOG" title="CH3NYANG&#39;S BLOG" type="HTML+XPath" xmlUrl="https://blog.ch3nyang.top/" htmlUrl="" description="" category="" frss:xPathItem="//a[contains(@class, &#39;pagination__item-wrapper&#39;)]" frss:xPathItemTitle=".//div[contains(@class, &#39;pagination__item-title-text&#39;)]" frss:xPathItemContent=".//div[contains(@class, &#39;pagination__item-summary&#39;)]" frss:xPathItemUri="./@href" frss:xPathItemAuthor="" frss:xPathItemTimestamp=".//div[contains(@class, &#39;pagination__item-time&#39;)]" frss:xPathItemTimeFormat="2006-01" frss:xPathItemThumbnail="" frss:xPathItemCategories="" frss:xPathItemUid=""></outline>
  </body>
</opml>`

	// Use a real fetcher that writes to an in-memory DB
	var importUserID int64
	db := func() *sqlite.DB {
		db, err := sqlite.NewDB(":memory:")
		if err != nil {
			t.Fatalf("failed to create db: %v", err)
		}
		if err := db.Init(); err != nil {
			t.Fatalf("failed to init db: %v", err)
		}
		_, importUserID = seedUsers(t, db)
		return db
	}()

	f := feed.NewFetcher(db)
	h := &corepkg.Handler{DB: db, Fetcher: f}

	req := httptest.NewRequest(http.MethodPost, "/opml/import", strings.NewReader(xmlData))
	req = withUser(req, importUserID)
	rr := httptest.NewRecorder()

	HandleOPMLImport(h, rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 got %d", rr.Code)
	}

	// Verify XPath feed was added
	feeds, err := db.GetFeeds()
	if err != nil {
		t.Fatalf("GetFeeds failed: %v", err)
	}
	if len(feeds) != 1 {
		t.Fatalf("expected 1 feed in DB, got %d", len(feeds))
	}

	feed := feeds[0]
	if feed.Type != "HTML+XPath" {
		t.Errorf("expected feed type 'HTML+XPath', got '%s'", feed.Type)
	}
	if feed.XPathItem != "//a[contains(@class, 'pagination__item-wrapper')]" {
		t.Errorf("expected XPathItem to be set, got '%s'", feed.XPathItem)
	}
	if feed.XPathItemTitle != ".//div[contains(@class, 'pagination__item-title-text')]" {
		t.Errorf("expected XPathItemTitle to be set, got '%s'", feed.XPathItemTitle)
	}
	if feed.XPathItemTimeFormat != "2006-01" {
		t.Errorf("expected XPathItemTimeFormat '2006-01', got '%s'", feed.XPathItemTimeFormat)
	}
	if feed.UserID != importUserID {
		t.Errorf("expected imported feed to belong to user %d, got %d", importUserID, feed.UserID)
	}
}

func TestHandleOPMLImport_DoesNotOverwriteAnotherUsersFeed(t *testing.T) {
	xmlData := `<?xml version="1.0"?>
<opml version="1.0">
  <head><title>Test</title></head>
  <body>
    <outline type="rss" text="Imported Feed" title="Imported Feed" xmlUrl="https://example.com/shared.xml" />
  </body>
</opml>`

	var existingUserID int64
	var importUserID int64
	db := func() *sqlite.DB {
		db, err := sqlite.NewDB(":memory:")
		if err != nil {
			t.Fatalf("failed to create db: %v", err)
		}
		if err := db.Init(); err != nil {
			t.Fatalf("failed to init db: %v", err)
		}
		existingUserID, importUserID = seedUsers(t, db)
		return db
	}()

	if _, err := db.AddFeedForUser(existingUserID, &models.Feed{
		UserID: existingUserID,
		Title:  "Existing Feed",
		URL:    "https://example.com/shared.xml",
	}); err != nil {
		t.Fatalf("seed AddFeedForUser failed: %v", err)
	}

	f := feed.NewFetcher(db)
	h := &corepkg.Handler{DB: db, Fetcher: f}

	req := httptest.NewRequest(http.MethodPost, "/opml/import", strings.NewReader(xmlData))
	req = withUser(req, importUserID)
	rr := httptest.NewRecorder()

	HandleOPMLImport(h, rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 got %d", rr.Code)
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
		t.Fatalf("expected 1 imported feed for user %d, got %d", importUserID, len(userTwoFeeds))
	}
	if userTwoFeeds[0].Title != "Imported Feed" {
		t.Fatalf("expected user 2 imported feed title, got %q", userTwoFeeds[0].Title)
	}
}

func TestHandleOPMLExport(t *testing.T) {
	var exportUserID int64
	db := func() *sqlite.DB {
		db, err := sqlite.NewDB(":memory:")
		if err != nil {
			t.Fatalf("failed to create db: %v", err)
		}
		if err := db.Init(); err != nil {
			t.Fatalf("failed to init db: %v", err)
		}
		exportUserID, _ = seedUsers(t, db)
		return db
	}()

	// Insert a feed for user 1
	_, err := db.AddFeed(&models.Feed{UserID: exportUserID, Title: "F1", URL: "http://f1"})
	if err != nil {
		t.Fatalf("AddFeed failed: %v", err)
	}
	// Sanity-check DB: try GetFeeds before calling handler
	if feeds, err := db.GetFeeds(); err != nil {
		t.Fatalf("GetFeeds before handler failed: %v", err)
	} else if len(feeds) == 0 {
		// continue â€?handler should still return data
	}

	h := &corepkg.Handler{DB: db}

	req := httptest.NewRequest(http.MethodGet, "/opml/export", nil)
	req = withUser(req, exportUserID)
	rr := httptest.NewRecorder()

	HandleOPMLExport(h, rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 got %d", rr.Code)
	}

	ct := rr.Header().Get("Content-Type")
	if !strings.Contains(ct, "text/xml") {
		t.Fatalf("expected text/xml content type, got %s", ct)
	}

	body := rr.Body.String()
	if !strings.Contains(body, "http://f1") {
		t.Fatalf("exported OPML missing feed URL: %s", body)
	}
}
