package translation

import (
	"fmt"
	"sync"
)

// SettingsProvider is an interface for retrieving translation settings.
type SettingsProvider interface {
	GetSetting(key string) (string, error)
}

// DynamicTranslator is a translator that dynamically selects the translation provider
// based on user settings. It creates the appropriate translator at translation time.
type DynamicTranslator struct {
	settings SettingsProvider
	mu       sync.RWMutex
	// Cache the current translator to avoid recreating it for every translation
	cachedTranslator Translator
	cachedProvider   string
	cachedAPIKey     string
	cachedAppID      string
	cachedSecretKey  string
	cachedEndpoint   string
	cachedModel      string
}

// NewDynamicTranslator creates a new dynamic translator that uses the given settings provider.
func NewDynamicTranslator(settings SettingsProvider) *DynamicTranslator {
	return &DynamicTranslator{
		settings: settings,
	}
}

// Translate translates text using the currently configured translation provider.
func (t *DynamicTranslator) Translate(text, targetLang string) (string, error) {
	if text == "" {
		return "", nil
	}

	translator, err := t.getTranslator()
	if err != nil {
		return "", err
	}

	return translator.Translate(text, targetLang)
}

// getTranslator returns the appropriate translator based on current settings.
// It caches the translator and only recreates it if settings have changed.
func (t *DynamicTranslator) getTranslator() (Translator, error) {
	provider, _ := t.settings.GetSetting("translation_provider")
	if provider == "" {
		provider = "google" // Default to Google Free
	}

	// Get provider-specific settings
	var apiKey, appID, secretKey, endpoint, model string
	switch provider {
	case "deepl":
		apiKey, _ = t.settings.GetSetting("deepl_api_key")
	case "baidu":
		appID, _ = t.settings.GetSetting("baidu_app_id")
		secretKey, _ = t.settings.GetSetting("baidu_secret_key")
	case "ai":
		apiKey, _ = t.settings.GetSetting("ai_api_key")
		endpoint, _ = t.settings.GetSetting("ai_endpoint")
		model, _ = t.settings.GetSetting("ai_model")
	}

	// Check if we can reuse the cached translator
	t.mu.RLock()
	if t.cachedTranslator != nil &&
		t.cachedProvider == provider &&
		t.cachedAPIKey == apiKey &&
		t.cachedAppID == appID &&
		t.cachedSecretKey == secretKey &&
		t.cachedEndpoint == endpoint &&
		t.cachedModel == model {
		translator := t.cachedTranslator
		t.mu.RUnlock()
		return translator, nil
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
		if apiKey == "" {
			return nil, fmt.Errorf("DeepL API key is required")
		}
		translator = NewDeepLTranslator(apiKey)
	case "baidu":
		if appID == "" || secretKey == "" {
			return nil, fmt.Errorf("Baidu App ID and Secret Key are required")
		}
		translator = NewBaiduTranslator(appID, secretKey)
	case "ai":
		if apiKey == "" {
			return nil, fmt.Errorf("AI API key is required")
		}
		translator = NewAITranslator(apiKey, endpoint, model)
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

	return translator, nil
}
