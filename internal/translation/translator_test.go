package translation

import (
	"testing"
)

func TestMockTranslator(t *testing.T) {
	translator := NewMockTranslator()
	text := "Hello"
	targetLang := "es"

	translated, err := translator.Translate(text, targetLang)
	if err != nil {
		t.Fatalf("Translate failed: %v", err)
	}

	expected := "[ES] Hello"
	if translated != expected {
		t.Errorf("Expected '%s', got '%s'", expected, translated)
	}

	// Test idempotency (mock implementation detail)
	translated2, err := translator.Translate(translated, targetLang)
	if err != nil {
		t.Fatalf("Translate failed: %v", err)
	}
	if translated2 != expected {
		t.Errorf("Expected '%s', got '%s'", expected, translated2)
	}
}
