package article_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"MavenRSS/internal/api/article"
	"MavenRSS/internal/api/core"
	"MavenRSS/internal/auth"
	ff "MavenRSS/internal/feed"
	"MavenRSS/internal/middleware"
	"MavenRSS/internal/models"
	"MavenRSS/internal/store/sqlite"
)

func setupHandler(t *testing.T) *core.Handler {
	t.Helper()
	db, err := sqlite.NewDB(":memory:")
	if err != nil {
		t.Fatalf("NewDB error: %v", err)
	}
	if err := db.Init(); err != nil {
		t.Fatalf("db Init error: %v", err)
	}
	f := ff.NewFetcher(db)
	t.Cleanup(func() {
		f.Stop()
	})
	return core.NewHandler(db, f, nil, nil)
}

func withTestUser(r *http.Request) *http.Request {
	return withUserID(r, 1)
}

func withUserID(r *http.Request, userID int64) *http.Request {
	role := "user"
	username := fmt.Sprintf("user-%d", userID)
	if userID == 1 {
		role = "admin"
		username = "admin"
	}

	claims := &auth.Claims{
		UserID:   userID,
		Username: username,
		Role:     role,
	}
	ctx := context.WithValue(r.Context(), middleware.UserContextKey, claims)
	return r.WithContext(ctx)
}

func waitForCondition(t *testing.T, timeout time.Duration, condition func() bool) {
	t.Helper()

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if condition() {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}

	t.Fatalf("condition not met within %v", timeout)
}

func TestHandleArticles_ListAndImageGallery(t *testing.T) {
	h := setupHandler(t)

	// Add a feed and articles
	feedID, err := h.DB.AddFeed(&models.Feed{Title: "F", URL: "http://x"})
	if err != nil {
		t.Fatalf("AddFeed: %v", err)
	}

	articles := []*models.Article{
		{FeedID: feedID, Title: "a1", URL: "u1", PublishedAt: time.Now()},
		{FeedID: feedID, Title: "a2", URL: "u2", PublishedAt: time.Now()},
	}
	if err := h.DB.SaveArticles(context.Background(), articles); err != nil {
		t.Fatalf("SaveArticles: %v", err)
	}

	// Call HandleArticles
	req := withTestUser(httptest.NewRequest(http.MethodGet, "/api/articles", nil))
	w := httptest.NewRecorder()
	article.HandleArticles(h, w, req)
	if w.Result().StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Result().StatusCode)
	}
	var got []models.Article
	if err := json.NewDecoder(w.Result().Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(got) < 2 {
		t.Fatalf("expected >=2 articles, got %d", len(got))
	}

	// Image gallery: mark feed as image mode and add image article
	if err := h.DB.UpdateFeed(feedID, "F", "http://x", "", "", false, "", false, 0, true, "", "", "", "", "", "", "", "", "", "", "", "", "", "", "", 0, "", "", "", false); err != nil {
		t.Fatalf("UpdateFeed: %v", err)
	}
	imgArticle := &models.Article{FeedID: feedID, Title: "img", URL: "iu", ImageURL: "http://img", PublishedAt: time.Now()}
	if err := h.DB.SaveArticles(context.Background(), []*models.Article{imgArticle}); err != nil {
		t.Fatalf("SaveArticles img: %v", err)
	}

	req2 := withTestUser(httptest.NewRequest(http.MethodGet, "/api/articles/image_gallery", nil))
	w2 := httptest.NewRecorder()
	article.HandleImageGalleryArticles(h, w2, req2)
	if w2.Result().StatusCode != http.StatusOK {
		t.Fatalf("expected 200 image gallery, got %d", w2.Result().StatusCode)
	}
	var imgs []models.Article
	if err := json.NewDecoder(w2.Result().Body).Decode(&imgs); err != nil {
		t.Fatalf("decode imgs: %v", err)
	}
	if len(imgs) == 0 {
		t.Fatalf("expected image articles, got 0")
	}
}

func TestHandleArticleContentCacheInfoAndCleanup(t *testing.T) {
	h := setupHandler(t)

	if _, err := h.DB.CreateUser(&models.User{
		Username:     "user2",
		Email:        "user2@example.com",
		PasswordHash: "hash",
		Role:         models.RoleUser,
		Status:       "active",
	}); err != nil {
		t.Fatalf("create user 2: %v", err)
	}

	if _, err := h.DB.Exec(`INSERT INTO articles (id, user_id, title, url, published_at) VALUES (101, 1, 'U1-A1', 'https://example.com/1', CURRENT_TIMESTAMP)`); err != nil {
		t.Fatalf("insert article 101: %v", err)
	}
	if _, err := h.DB.Exec(`INSERT INTO articles (id, user_id, title, url, published_at) VALUES (102, 1, 'U1-A2', 'https://example.com/2', CURRENT_TIMESTAMP)`); err != nil {
		t.Fatalf("insert article 102: %v", err)
	}
	if _, err := h.DB.Exec(`INSERT INTO articles (id, user_id, title, url, published_at) VALUES (201, 2, 'U2-A1', 'https://example.com/3', CURRENT_TIMESTAMP)`); err != nil {
		t.Fatalf("insert article 201: %v", err)
	}

	if err := h.DB.SetArticleContent(101, "<p>cached 1</p>"); err != nil {
		t.Fatalf("set article content 101: %v", err)
	}
	if err := h.DB.SetArticleContent(102, "<p>cached 2</p>"); err != nil {
		t.Fatalf("set article content 102: %v", err)
	}
	if err := h.DB.SetArticleContent(201, "<p>cached 3</p>"); err != nil {
		t.Fatalf("set article content 201: %v", err)
	}

	req := withTestUser(httptest.NewRequest(http.MethodGet, "/api/articles/content-cache-info", nil))
	w := httptest.NewRecorder()
	article.HandleGetArticleContentCacheInfo(h, w, req)
	if w.Result().StatusCode != http.StatusOK {
		t.Fatalf("expected 200 for cache info, got %d", w.Result().StatusCode)
	}

	var info map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&info); err != nil {
		t.Fatalf("decode cache info: %v", err)
	}

	if got := int(info["cached_articles"].(float64)); got != 2 {
		t.Fatalf("expected 2 cached articles for user 1, got %d", got)
	}
	if got := int(info["count"].(float64)); got != 2 {
		t.Fatalf("expected count alias to equal 2, got %d", got)
	}

	cleanupReq := withTestUser(httptest.NewRequest(http.MethodPost, "/api/articles/cleanup-content", nil))
	cleanupW := httptest.NewRecorder()
	article.HandleCleanupArticleContents(h, cleanupW, cleanupReq)
	if cleanupW.Result().StatusCode != http.StatusOK {
		t.Fatalf("expected 200 for cleanup, got %d", cleanupW.Result().StatusCode)
	}

	var cleanupResp map[string]interface{}
	if err := json.NewDecoder(cleanupW.Body).Decode(&cleanupResp); err != nil {
		t.Fatalf("decode cleanup response: %v", err)
	}

	if got := int(cleanupResp["deleted"].(float64)); got != 2 {
		t.Fatalf("expected deleted to equal 2, got %d", got)
	}
	if got := int(cleanupResp["entries_cleaned"].(float64)); got != 2 {
		t.Fatalf("expected entries_cleaned to equal 2, got %d", got)
	}

	count, err := h.DB.GetArticleContentCount(1)
	if err != nil {
		t.Fatalf("GetArticleContentCount user 1: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected user 1 cache count to be 0 after cleanup, got %d", count)
	}

	otherUserCount, err := h.DB.GetArticleContentCount(2)
	if err != nil {
		t.Fatalf("GetArticleContentCount user 2: %v", err)
	}
	if otherUserCount != 1 {
		t.Fatalf("expected user 2 cache count to remain 1, got %d", otherUserCount)
	}
}

func TestArticleActions_MarkRead_Favorite_Hide_ReadLater(t *testing.T) {
	h := setupHandler(t)
	feedID, _ := h.DB.AddFeed(&models.Feed{Title: "F2", URL: "http://y"})

	a := &models.Article{FeedID: feedID, Title: "act", URL: "u", PublishedAt: time.Now()}
	if err := h.DB.SaveArticles(context.Background(), []*models.Article{a}); err != nil {
		t.Fatalf("SaveArticles: %v", err)
	}
	// fetch saved article id
	arts, err := h.DB.GetArticles("", feedID, "", true, 10, 0)
	if err != nil || len(arts) == 0 {
		t.Fatalf("GetArticles: %v", err)
	}
	id := arts[0].ID

	// Mark unread -> read
	req := withTestUser(httptest.NewRequest(http.MethodPost, "/api/articles/mark-read-sync?id="+fmt.Sprint(id)+"&read=true", nil))
	w := httptest.NewRecorder()
	article.HandleMarkReadWithImmediateSync(h, w, req)
	if w.Result().StatusCode != http.StatusOK {
		t.Fatalf("mark read failed: %d", w.Result().StatusCode)
	}

	// Toggle favorite
	req2 := withTestUser(httptest.NewRequest(http.MethodPost, "/api/articles/toggle-favorite-sync?id="+fmt.Sprint(id), nil))
	w2 := httptest.NewRecorder()
	article.HandleToggleFavoriteWithImmediateSync(h, w2, req2)
	if w2.Result().StatusCode != http.StatusOK {
		t.Fatalf("toggle fav failed: %d", w2.Result().StatusCode)
	}

	// Toggle hide (invalid method GET -> 405)
	req3 := withTestUser(httptest.NewRequest(http.MethodGet, "/api/articles/toggle_hide?id="+fmt.Sprint(id), nil))
	w3 := httptest.NewRecorder()
	article.HandleToggleHideArticle(h, w3, req3)
	if w3.Result().StatusCode != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405 for GET hide, got %d", w3.Result().StatusCode)
	}

	// Proper POST hide
	req4 := withTestUser(httptest.NewRequest(http.MethodPost, "/api/articles/toggle_hide?id="+fmt.Sprint(id), nil))
	w4 := httptest.NewRecorder()
	article.HandleToggleHideArticle(h, w4, req4)
	if w4.Result().StatusCode != http.StatusOK {
		t.Fatalf("toggle hide failed: %d", w4.Result().StatusCode)
	}

	// Toggle read later (POST)
	req5 := withTestUser(httptest.NewRequest(http.MethodPost, "/api/articles/toggle_read_later?id="+fmt.Sprint(id), nil))
	w5 := httptest.NewRecorder()
	article.HandleToggleReadLater(h, w5, req5)
	if w5.Result().StatusCode != http.StatusOK {
		t.Fatalf("toggle read later failed: %d", w5.Result().StatusCode)
	}
}

func TestHandleExportToObsidian(t *testing.T) {
	h := setupHandler(t)

	// Enable Obsidian integration
	if err := h.DB.SetSetting("obsidian_enabled", "true"); err != nil {
		t.Fatalf("SetSetting obsidian_enabled: %v", err)
	}
	if err := h.DB.SetSetting("obsidian_vault_path", t.TempDir()); err != nil {
		t.Fatalf("SetSetting obsidian_vault_path: %v", err)
	}

	// Add a feed and article
	feedID, err := h.DB.AddFeed(&models.Feed{Title: "Test Feed", URL: "http://example.com"})
	if err != nil {
		t.Fatalf("AddFeed: %v", err)
	}

	articleModel := &models.Article{
		FeedID:      feedID,
		Title:       "Test Article",
		URL:         "http://example.com/article",
		PublishedAt: time.Now(),
	}
	if err := h.DB.SaveArticles(context.Background(), []*models.Article{articleModel}); err != nil {
		t.Fatalf("SaveArticles: %v", err)
	}

	// Get the article ID
	articles, err := h.DB.GetArticles("", feedID, "", false, 10, 0)
	if err != nil || len(articles) == 0 {
		t.Fatalf("GetArticles: %v", err)
	}
	articleID := articles[0].ID

	// Test export request
	reqBody := fmt.Sprintf(`{"article_id": %d}`, articleID)
	req := httptest.NewRequest(http.MethodPost, "/api/articles/export/obsidian", strings.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	article.HandleExportToObsidian(h, w, req)

	if w.Result().StatusCode != http.StatusOK {
		t.Fatalf("Export failed: %d, body: %s", w.Result().StatusCode, w.Body.String())
	}

	// Verify response
	var response map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if response["success"] != "true" {
		t.Fatalf("Export not successful: %v", response)
	}
}

func TestHandleRefresh_UsesDetachedUserScopedRefresh(t *testing.T) {
	h := setupHandler(t)

	if _, err := h.DB.CreateUser(&models.User{
		Username:     "user2",
		Email:        "user2@example.com",
		PasswordHash: "hash",
		Role:         models.RoleUser,
		Status:       "active",
	}); err != nil {
		t.Fatalf("create user 2: %v", err)
	}

	if err := h.DB.SetSetting("retry_timeout_seconds", "1"); err != nil {
		t.Fatalf("set retry timeout: %v", err)
	}

	if _, err := h.DB.AddFeedForUser(1, &models.Feed{
		Title: "User 1 Feed",
		URL:   "://invalid-user-1",
	}); err != nil {
		t.Fatalf("add user 1 feed: %v", err)
	}

	if _, err := h.DB.AddFeedForUser(2, &models.Feed{
		Title: "User 2 Feed",
		URL:   "://invalid-user-2",
	}); err != nil {
		t.Fatalf("add user 2 feed: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	req := httptest.NewRequest(http.MethodPost, "/api/refresh", nil).WithContext(ctx)
	req = withUserID(req, 1)
	w := httptest.NewRecorder()

	article.HandleRefresh(h, w, req)

	if w.Result().StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Result().StatusCode)
	}

	waitForCondition(t, 2*time.Second, func() bool {
		progress := h.Fetcher.GetProgressWithStatsForUser(1)
		return !progress.IsRunning && progress.QueueTaskCount == 0 && progress.PoolTaskCount == 0
	})

	user2Progress := h.Fetcher.GetProgressWithStatsForUser(2)
	if user2Progress.IsRunning {
		t.Fatalf("expected user 2 refresh to stay idle, got %+v", user2Progress)
	}
	if user2Progress.QueueTaskCount != 0 || user2Progress.PoolTaskCount != 0 {
		t.Fatalf("expected user 2 to have no tasks, got %+v", user2Progress)
	}
}

func TestHandleStopRefresh_StopsOnlyCurrentUser(t *testing.T) {
	h := setupHandler(t)

	if _, err := h.DB.CreateUser(&models.User{
		Username:     "user2",
		Email:        "user2@example.com",
		PasswordHash: "hash",
		Role:         models.RoleUser,
		Status:       "active",
	}); err != nil {
		t.Fatalf("create user 2: %v", err)
	}

	feed1ID, err := h.DB.AddFeedForUser(1, &models.Feed{
		Title: "User 1 Feed",
		URL:   "https://example.com/user1.xml",
	})
	if err != nil {
		t.Fatalf("add user 1 feed: %v", err)
	}

	feed2ID, err := h.DB.AddFeedForUser(2, &models.Feed{
		Title: "User 2 Feed",
		URL:   "https://example.com/user2.xml",
	})
	if err != nil {
		t.Fatalf("add user 2 feed: %v", err)
	}

	feed1, err := h.DB.GetFeedByIDForUser(1, feed1ID)
	if err != nil || feed1 == nil {
		t.Fatalf("get user 1 feed: %v", err)
	}
	feed2, err := h.DB.GetFeedByIDForUser(2, feed2ID)
	if err != nil || feed2 == nil {
		t.Fatalf("get user 2 feed: %v", err)
	}

	stuckCtx, cancel := context.WithCancel(context.Background())
	cancel()

	tm := h.Fetcher.GetTaskManager()
	tm.AddGlobalRefreshForUser(stuckCtx, []models.Feed{*feed1}, 1)
	tm.AddGlobalRefreshForUser(stuckCtx, []models.Feed{*feed2}, 2)

	waitForCondition(t, time.Second, func() bool {
		progress1 := h.Fetcher.GetProgressWithStatsForUser(1)
		progress2 := h.Fetcher.GetProgressWithStatsForUser(2)
		return progress1.QueueTaskCount == 1 && progress2.QueueTaskCount == 1
	})

	req := withUserID(httptest.NewRequest(http.MethodPost, "/api/refresh/stop", nil), 1)
	w := httptest.NewRecorder()

	article.HandleStopRefresh(h, w, req)

	if w.Result().StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Result().StatusCode)
	}

	progress1 := h.Fetcher.GetProgressWithStatsForUser(1)
	if progress1.IsRunning || progress1.QueueTaskCount != 0 || progress1.PoolTaskCount != 0 {
		t.Fatalf("expected user 1 tasks to be stopped, got %+v", progress1)
	}

	progress2 := h.Fetcher.GetProgressWithStatsForUser(2)
	if !progress2.IsRunning || progress2.QueueTaskCount != 1 {
		t.Fatalf("expected user 2 tasks to remain, got %+v", progress2)
	}
}
