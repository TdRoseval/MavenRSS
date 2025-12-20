package translation

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"MrRSS/internal/database"
	corepkg "MrRSS/internal/handlers/core"
	transpkg "MrRSS/internal/translation"
)

func setupDB(t *testing.T) *database.DB {
	t.Helper()
	db, err := database.NewDB(":memory:")
	if err != nil {
		t.Fatalf("failed to create db: %v", err)
	}
	if err := db.Init(); err != nil {
		t.Fatalf("failed to init db: %v", err)
	}
	return db
}

func TestHandleTranslateText_Success(t *testing.T) {
	db := setupDB(t)
	h := &corepkg.Handler{DB: db, Translator: transpkg.NewMockTranslator()}

	body := map[string]string{"text": "Hello", "target_language": "fr"}
	b, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/translate/text", bytes.NewReader(b))
	rr := httptest.NewRecorder()

	HandleTranslateText(h, rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 got %d", rr.Code)
	}

	var resp map[string]string
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode failed: %v", err)
	}

	if resp["translated_text"] != "[FR] Hello" {
		t.Fatalf("unexpected translation: %v", resp["translated_text"])
	}
}

func TestHandleTranslateArticle_SuccessAndDBUpdate(t *testing.T) {
	db := setupDB(t)

	// insert an article
	res, err := db.Exec("INSERT INTO articles (feed_id, title, url, published_at) VALUES (1, 't', 'u', datetime('now'))")
	if err != nil {
		t.Fatalf("insert article failed: %v", err)
	}
	id, _ := res.LastInsertId()

	h := &corepkg.Handler{DB: db, Translator: transpkg.NewMockTranslator()}

	body := map[string]interface{}{"article_id": id, "title": "Hello", "target_language": "es"}
	b, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/translate/article", bytes.NewReader(b))
	rr := httptest.NewRecorder()

	HandleTranslateArticle(h, rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 got %d", rr.Code)
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode failed: %v", err)
	}

	if resp["translated_title"] != "[ES] Hello" {
		t.Fatalf("unexpected translation: %v", resp["translated_title"])
	}

	// verify in DB
	var stored string
	if err := db.QueryRow("SELECT translated_title FROM articles WHERE id = ?", id).Scan(&stored); err != nil {
		t.Fatalf("query failed: %v", err)
	}
	if stored != "[ES] Hello" {
		t.Fatalf("db value mismatch: %v", stored)
	}
}

func TestHandleClearTranslations(t *testing.T) {
	db := setupDB(t)

	// insert an article with translated title
	res, err := db.Exec("INSERT INTO articles (feed_id, title, url, translated_title, published_at) VALUES (1, 't', 'u', 'x', datetime('now'))")
	if err != nil {
		t.Fatalf("insert article failed: %v", err)
	}
	_, _ = res.LastInsertId()

	h := &corepkg.Handler{DB: db, Translator: transpkg.NewMockTranslator()}

	req := httptest.NewRequest(http.MethodPost, "/translate/clear", nil)
	rr := httptest.NewRecorder()

	HandleClearTranslations(h, rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 got %d", rr.Code)
	}

	// verify cleared
	var count int
	if err := db.QueryRow("SELECT COUNT(*) FROM articles WHERE translated_title != ''").Scan(&count); err != nil {
		t.Fatalf("query failed: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected 0 translations remaining, got %d", count)
	}
}
