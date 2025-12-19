package translation

import (
	"fmt"
	"sync"
)

// SettingsProvider is an interface for retrieving translation settings.
type SettingsProvider interface {
	GetSetting(key string) (string, error)
	GetEncryptedSetting(key string) (string, error)
}

// CacheProvider is an interface for translation caching
type CacheProvider interface {
	GetCachedTranslation(sourceTextHash, targetLang, provider string) (string, bool, error)
	SetCachedTranslation(sourceTextHash, sourceText, targetLang, translatedText, provider string) error
}

// DynamicTranslator is a translator that dynamically selects the translation provider
// based on user settings. It creates the appropriate translator at translation time.
type DynamicTranslator struct {
	settings SettingsProvider
	cache    CacheProvider
	mu       sync.RWMutex
	// Cache the current translator to avoid recreating it for every translation
	cachedTranslator Translator
	cachedProvider   string
	cachedAPIKey     string
	cachedAppID      string
	cachedSecretKey  string
	cachedEndpoint   string
	cachedModel      string
	cachedPrompt     string
}

// NewDynamicTranslator creates a new dynamic translator that uses the given settings provider.
func NewDynamicTranslator(settings SettingsProvider) *DynamicTranslator {
	return &DynamicTranslator{
		settings: settings,
	}
}

// NewDynamicTranslatorWithCache creates a new dynamic translator with translation caching.
func NewDynamicTranslatorWithCache(settings SettingsProvider, cache CacheProvider) *DynamicTranslator {
	return &DynamicTranslator{
		settings: settings,
		cache:    cache,
	}
}

// Translate translates text using the currently configured translation provider.
func (t *DynamicTranslator) Translate(text, targetLang string) (string, error) {
	if text == "" {
		return "", nil
	}

	translator, provider, err := t.getTranslatorWithProvider()
	if err != nil {
		return "", err
	}

	// Wrap with caching if cache is available
	if t.cache != nil {
		cachedTranslator := NewCachedTranslator(translator, t.cache, provider)
		return cachedTranslator.Translate(text, targetLang)
	}

	return translator.Translate(text, targetLang)
}

// getTranslatorWithProvider returns the appropriate translator and provider name based on current settings.
// It caches the translator and only recreates it if settings have changed.
func (t *DynamicTranslator) getTranslatorWithProvider() (Translator, string, error) {
	provider, _ := t.settings.GetSetting("translation_provider")
	if provider == "" {
		provider = "google" // Default to Google Free
	}

	// Get provider-specific settings (use encrypted methods for sensitive credentials)
	var apiKey, appID, secretKey, endpoint, model, systemPrompt string
	switch provider {
	case "deepl":
		apiKey, _ = t.settings.GetEncryptedSetting("deepl_api_key")
		endpoint, _ = t.settings.GetSetting("deepl_endpoint")
	case "baidu":
		appID, _ = t.settings.GetSetting("baidu_app_id")
		secretKey, _ = t.settings.GetEncryptedSetting("baidu_secret_key")
	case "ai":
		apiKey, _ = t.settings.GetEncryptedSetting("ai_api_key")
		endpoint, _ = t.settings.GetSetting("ai_endpoint")
		model, _ = t.settings.GetSetting("ai_model")
		systemPrompt, _ = t.settings.GetSetting("ai_system_prompt")
	}

	// Check if we can reuse the cached translator
	t.mu.RLock()
	if t.cachedTranslator != nil &&
		t.cachedProvider == provider &&
		t.cachedAPIKey == apiKey &&
		t.cachedAppID == appID &&
		t.cachedSecretKey == secretKey &&
		t.cachedEndpoint == endpoint &&
		t.cachedModel == model &&
		t.cachedPrompt == systemPrompt {
		translator := t.cachedTranslator
		t.mu.RUnlock()
		return translator, provider, nil
	}
	t.mu.RUnlock()

	// Create new translator
	t.mu.Lock()
	defer t.mu.Unlock()

	var translator Translator
	switch provider {
	case "google":
		translator = NewGoogleFreeTranslator()
	case "deepl":
		// For deeplx self-hosted, endpoint is required but API key is optional
		if endpoint == "" && apiKey == "" {
			return nil, "", fmt.Errorf("DeepL API key is required (or provide a custom endpoint for deeplx)")
		}
		if endpoint != "" {
			translator = NewDeepLTranslatorWithEndpoint(apiKey, endpoint)
		} else {
			translator = NewDeepLTranslator(apiKey)
		}
	case "baidu":
		if appID == "" || secretKey == "" {
			return nil, "", fmt.Errorf("Baidu App ID and Secret Key are required")
		}
		translator = NewBaiduTranslator(appID, secretKey)
	case "ai":
		if apiKey == "" {
			return nil, "", fmt.Errorf("AI API key is required")
		}
		aiTranslator := NewAITranslator(apiKey, endpoint, model)
		if systemPrompt != "" {
			aiTranslator.SetSystemPrompt(systemPrompt)
		}
		translator = aiTranslator
	default:
		translator = NewGoogleFreeTranslator()
	}

	// Cache the translator
	t.cachedTranslator = translator
	t.cachedProvider = provider
	t.cachedAPIKey = apiKey
	t.cachedAppID = appID
	t.cachedSecretKey = secretKey
	t.cachedEndpoint = endpoint
	t.cachedModel = model
	t.cachedPrompt = systemPrompt

	return translator, provider, nil
}
