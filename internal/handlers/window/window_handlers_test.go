package window

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"MavenRSS/internal/database"
	corepkg "MavenRSS/internal/handlers/core"
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

func TestHandleGetWindowState_Empty(t *testing.T) {
	db := setupDB(t)
	h := &corepkg.Handler{DB: db}

	req := httptest.NewRequest(http.MethodGet, "/window/state", nil)
	rr := httptest.NewRecorder()

	HandleGetWindowState(h, rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 got %d", rr.Code)
	}

	var resp map[string]string
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode failed: %v", err)
	}

	// Defaults from config may be empty strings; ensure keys exist
	expectedKeys := []string{"x", "y", "width", "height", "maximized"}
	for _, k := range expectedKeys {
		if _, ok := resp[k]; !ok {
			t.Fatalf("missing key %s in response", k)
		}
	}
}

func TestHandleSaveWindowState_SaveAndPersist(t *testing.T) {
	db := setupDB(t)
	h := &corepkg.Handler{DB: db}

	state := WindowState{X: 10, Y: 20, Width: 800, Height: 600, Maximized: true}
	b, _ := json.Marshal(state)

	req := httptest.NewRequest(http.MethodPost, "/window/state", bytes.NewReader(b))
	rr := httptest.NewRecorder()

	HandleSaveWindowState(h, rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 got %d", rr.Code)
	}

	// Verify settings stored in DB
	v, err := db.GetSetting("window_x")
	if err != nil {
		t.Fatalf("GetSetting window_x failed: %v", err)
	}
	if v != "10" {
		t.Fatalf("window_x expected '10', got %q", v)
	}

	v, _ = db.GetSetting("window_maximized")
	if v != "true" {
		t.Fatalf("window_maximized expected 'true', got %q", v)
	}
}

func TestHandleSaveWindowState_MethodNotAllowed(t *testing.T) {
	db := setupDB(t)
	h := &corepkg.Handler{DB: db}

	req := httptest.NewRequest(http.MethodGet, "/window/state", nil)
	rr := httptest.NewRecorder()

	HandleSaveWindowState(h, rr, req)

	if rr.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected %d got %d", http.StatusMethodNotAllowed, rr.Code)
	}
}
