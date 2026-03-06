package rules

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHandleApplyRule_MethodNotAllowed(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/rules/apply", nil)
	rr := httptest.NewRecorder()

	HandleApplyRule(nil, rr, req)

	if rr.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected %d got %d", http.StatusMethodNotAllowed, rr.Code)
	}
}

func TestHandleApplyRule_InvalidJSON(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/rules/apply", bytes.NewReader([]byte("not json")))
	rr := httptest.NewRecorder()

	HandleApplyRule(nil, rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected %d got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestHandleApplyRule_NoActions(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/rules/apply", bytes.NewReader([]byte(`{"name":"r","actions":[]}`)))
	rr := httptest.NewRecorder()

	HandleApplyRule(nil, rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected %d got %d", http.StatusBadRequest, rr.Code)
	}
}
