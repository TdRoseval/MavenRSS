package media

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"MavenRSS/internal/api/core"
	"MavenRSS/internal/auth"
	"MavenRSS/internal/middleware"
	"MavenRSS/internal/models"
	"MavenRSS/internal/store/sqlite"
)

func withMediaTestUser(r *http.Request, userID int64) *http.Request {
	claims := &auth.Claims{
		UserID:   userID,
		Username: "tester",
		Role:     "user",
	}
	ctx := context.WithValue(r.Context(), middleware.UserContextKey, claims)
	return r.WithContext(ctx)
}

func TestHandleMediaCacheInfoAndCleanup(t *testing.T) {
	tmp := t.TempDir()
	// Ensure data dir resolves to temp dir
	_ = os.Setenv("APPDATA", tmp)
	_ = os.Setenv("HOME", tmp)
	_ = os.Setenv("XDG_DATA_HOME", tmp)

	db, err := sqlite.NewDB(":memory:")
	if err != nil {
		t.Fatalf("NewDB failed: %v", err)
	}
	if err := db.Init(); err != nil {
		t.Fatalf("db Init failed: %v", err)
	}
	if _, err := db.CreateUser(&models.User{
		Username:     "user2",
		Email:        "user2@example.com",
		PasswordHash: "hash",
		Role:         models.RoleUser,
		Status:       "active",
	}); err != nil {
		t.Fatalf("CreateUser user2 failed: %v", err)
	}

	h := core.NewHandler(db, nil, nil, nil)
	// Enable media cache and set small thresholds
	if err := h.DB.SetSetting("media_cache_enabled", "true"); err != nil {
		t.Fatalf("SetSetting failed: %v", err)
	}
	if err := h.DB.SetSetting("media_cache_max_age_days", "1"); err != nil {
		t.Fatalf("SetSetting failed: %v", err)
	}
	if err := h.DB.SetSetting("media_cache_max_size_mb", "1"); err != nil {
		t.Fatalf("SetSetting failed: %v", err)
	}

	// Get the cache directory
	cacheDir := filepath.Join(tmp, "MavenRSS", "media_cache")
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		t.Fatalf("Failed to create cache dir: %v", err)
	}
	user1CacheDir := filepath.Join(cacheDir, "user_1")
	user2CacheDir := filepath.Join(cacheDir, "user_2")
	if err := os.MkdirAll(user1CacheDir, 0755); err != nil {
		t.Fatalf("Failed to create user1 cache dir: %v", err)
	}
	if err := os.MkdirAll(user2CacheDir, 0755); err != nil {
		t.Fatalf("Failed to create user2 cache dir: %v", err)
	}

	// Create some test cache files for two users
	testDataUser1 := []byte("test image data")
	testDataUser2 := []byte("user2-only-cache")
	file1 := filepath.Join(user1CacheDir, "abc123.jpg")
	file2 := filepath.Join(user1CacheDir, "def456.png")
	file3 := filepath.Join(user2CacheDir, "ghi789.webp")
	if err := os.WriteFile(file1, testDataUser1, 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}
	if err := os.WriteFile(file2, testDataUser1, 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}
	if err := os.WriteFile(file3, testDataUser2, 0644); err != nil {
		t.Fatalf("Failed to create user2 test file: %v", err)
	}
	expectedUser1SizeMB := float64(len(testDataUser1)*2) / (1024 * 1024)

	// Call info (GET) as user 1 - should only show user 1 cache size
	req := withMediaTestUser(httptest.NewRequest(http.MethodGet, "/media/info", nil), 1)
	rr := httptest.NewRecorder()
	HandleMediaCacheInfo(h, rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 for info, got %d", rr.Code)
	}
	var info map[string]float64
	if err := json.NewDecoder(rr.Body).Decode(&info); err != nil {
		t.Fatalf("decode info failed: %v", err)
	}

	cacheSizeMB, ok := info["cache_size_mb"]
	if !ok {
		t.Fatalf("info missing cache_size_mb")
	}
	if cacheSizeMB != expectedUser1SizeMB {
		t.Errorf("Expected user 1 cache size %f MB, got %f", expectedUser1SizeMB, cacheSizeMB)
	}

	// Test 1: Cleanup without ?all=true parameter (automatic cleanup)
	// Should respect max_age_days setting and not clean new files
	req2 := withMediaTestUser(httptest.NewRequest(http.MethodPost, "/media/cleanup", nil), 1)
	rr2 := httptest.NewRecorder()
	HandleMediaCacheCleanup(h, rr2, req2)
	if rr2.Code != http.StatusOK {
		t.Fatalf("expected 200 for cleanup, got %d", rr2.Code)
	}
	var resp map[string]interface{}
	if err := json.NewDecoder(rr2.Body).Decode(&resp); err != nil {
		t.Fatalf("decode cleanup failed: %v", err)
	}
	if success, ok := resp["success"].(bool); !ok || !success {
		t.Fatalf("expected cleanup success true, got %v", resp)
	}
	filesCleaned := int(resp["files_cleaned"].(float64))
	if filesCleaned != 0 {
		t.Errorf("Expected 0 files cleaned (too new), got %d", filesCleaned)
	}

	// Test 2: Cleanup with ?all=true parameter (manual cleanup)
	// Should clean all files regardless of age
	req3 := withMediaTestUser(httptest.NewRequest(http.MethodPost, "/media/cleanup?all=true", nil), 1)
	rr3 := httptest.NewRecorder()
	HandleMediaCacheCleanup(h, rr3, req3)
	if rr3.Code != http.StatusOK {
		t.Fatalf("expected 200 for cleanup with all=true, got %d", rr3.Code)
	}
	var resp2 map[string]interface{}
	if err := json.NewDecoder(rr3.Body).Decode(&resp2); err != nil {
		t.Fatalf("decode cleanup failed: %v", err)
	}
	filesCleaned2 := int(resp2["files_cleaned"].(float64))
	if filesCleaned2 == 0 {
		t.Error("Expected files to be cleaned with ?all=true")
	}
	if filesCleaned2 != 2 {
		t.Errorf("Expected 2 files cleaned with ?all=true, got %d", filesCleaned2)
	}

	// Test 3: Verify user 1 cache is now empty
	req4 := withMediaTestUser(httptest.NewRequest(http.MethodGet, "/media/info", nil), 1)
	rr4 := httptest.NewRecorder()
	HandleMediaCacheInfo(h, rr4, req4)
	if rr4.Code != http.StatusOK {
		t.Fatalf("expected 200 for info, got %d", rr4.Code)
	}
	var info2 map[string]float64
	if err := json.NewDecoder(rr4.Body).Decode(&info2); err != nil {
		t.Fatalf("decode info failed: %v", err)
	}
	if info2["cache_size_mb"] != 0 {
		t.Errorf("Expected cache size 0 after cleanup, got %f", info2["cache_size_mb"])
	}

	// Test 4: Verify user 2 cache is untouched
	req5 := withMediaTestUser(httptest.NewRequest(http.MethodGet, "/media/info", nil), 2)
	rr5 := httptest.NewRecorder()
	HandleMediaCacheInfo(h, rr5, req5)
	if rr5.Code != http.StatusOK {
		t.Fatalf("expected 200 for user2 info, got %d", rr5.Code)
	}
	var info3 map[string]float64
	if err := json.NewDecoder(rr5.Body).Decode(&info3); err != nil {
		t.Fatalf("decode user2 info failed: %v", err)
	}
	expectedUser2SizeMB := float64(len(testDataUser2)) / (1024 * 1024)
	if info3["cache_size_mb"] != expectedUser2SizeMB {
		t.Errorf("Expected user 2 cache size %f MB, got %f", expectedUser2SizeMB, info3["cache_size_mb"])
	}
}
