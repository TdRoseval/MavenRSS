package ai

import (
	"strings"
	"testing"
)

func TestOpenAIValidateResponseIncludesProviderErrorDetails(t *testing.T) {
	t.Parallel()

	handler := NewOpenAIHandler()
	body := []byte(`{"error":{"message":"request blocked by safety policy","type":"invalid_request_error","code":"1301"}}`)

	err := handler.ValidateResponse(400, body)
	if err == nil {
		t.Fatal("ValidateResponse returned nil error")
	}

	got := err.Error()
	for _, want := range []string{
		"bad request - check parameters",
		"request blocked by safety policy",
		"code=1301",
		"type=invalid_request_error",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("ValidateResponse error %q does not contain %q", got, want)
		}
	}
}

func TestOpenAIValidateResponseFallsBackToCompactBody(t *testing.T) {
	t.Parallel()

	handler := NewOpenAIHandler()
	body := []byte(`{"msg":"raw upstream failure"}`)

	err := handler.ValidateResponse(400, body)
	if err == nil {
		t.Fatal("ValidateResponse returned nil error")
	}

	if got := err.Error(); !strings.Contains(got, "raw upstream failure") {
		t.Fatalf("ValidateResponse error %q does not contain upstream body", got)
	}
}
