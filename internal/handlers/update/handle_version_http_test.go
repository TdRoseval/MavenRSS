package update

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"MavenRSS/internal/handlers/core"
)

func TestHandleVersion_GET(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/version", nil)
	rr := httptest.NewRecorder()

	// handler does not use fields, so an empty Handler is fine
	HandleVersion(&core.Handler{}, rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rr.Code)
	}

	var resp map[string]string
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if _, ok := resp["version"]; !ok {
		t.Fatalf("response missing version field")
	}
}

func TestHandleVersion_MethodNotAllowed(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/version", nil)
	rr := httptest.NewRecorder()

	HandleVersion(&core.Handler{}, rr, req)

	if rr.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected status %d, got %d", http.StatusMethodNotAllowed, rr.Code)
	}
}
