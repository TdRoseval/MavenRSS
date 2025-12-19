package translation

import (
	"crypto/sha256"
	"encoding/hex"
	"log"
)

// TranslationCache is an interface for caching translations
type TranslationCache interface {
	GetCachedTranslation(sourceTextHash, targetLang, provider string) (string, bool, error)
	SetCachedTranslation(sourceTextHash, sourceText, targetLang, translatedText, provider string) error
}

// CachedTranslator wraps a translator with caching functionality
type CachedTranslator struct {
	translator Translator
	cache      TranslationCache
	provider   string
}

// NewCachedTranslator creates a new cached translator
func NewCachedTranslator(translator Translator, cache TranslationCache, provider string) *CachedTranslator {
	return &CachedTranslator{
		translator: translator,
		cache:      cache,
		provider:   provider,
	}
}

// Translate translates text, using cache when available
func (ct *CachedTranslator) Translate(text, targetLang string) (string, error) {
	if text == "" {
		return "", nil
	}

	// Generate hash for cache lookup
	textHash := hashText(text)

	// Try to get from cache first
	if ct.cache != nil {
		if cached, found, err := ct.cache.GetCachedTranslation(textHash, targetLang, ct.provider); err == nil && found {
			return cached, nil
		}
	}

	// Not in cache, perform translation
	translated, err := ct.translator.Translate(text, targetLang)
	if err != nil {
		return "", err
	}

	// Cache the result (including when source == translation, meaning no translation needed)
	if ct.cache != nil {
		if cacheErr := ct.cache.SetCachedTranslation(textHash, text, targetLang, translated, ct.provider); cacheErr != nil {
			// Log but don't fail - caching is optional
			log.Printf("Warning: failed to cache translation: %v", cacheErr)
		}
	}

	return translated, nil
}

// hashText creates a SHA256 hash of the text for cache lookup
func hashText(text string) string {
	h := sha256.New()
	h.Write([]byte(text))
	return hex.EncodeToString(h.Sum(nil))
}
