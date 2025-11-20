package translation

import (
	"fmt"
	"strings"
)

// Translator defines the interface for translation services
type Translator interface {
	Translate(text, targetLang string) (string, error)
}

// MockTranslator is a simple translator for demonstration
type MockTranslator struct{}

func NewMockTranslator() *MockTranslator {
	return &MockTranslator{}
}

func (t *MockTranslator) Translate(text, targetLang string) (string, error) {
	// In a real application, this would call an external API (Google, DeepL, etc.)
	// For now, we simulate translation by appending the language code.
	// We can also do some simple word replacements to make it look "translated"

	prefix := fmt.Sprintf("[%s] ", strings.ToUpper(targetLang))
	if strings.HasPrefix(text, prefix) {
		return text, nil
	}

	return prefix + text, nil
}
