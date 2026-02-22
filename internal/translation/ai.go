package translation

import (
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"MrRSS/internal/ai"
	"MrRSS/internal/config"
	"MrRSS/internal/utils/httputil"
)

// AITranslator implements translation using OpenAI-compatible APIs (GPT, Claude, etc.).
type AITranslator struct {
	APIKey         string
	Endpoint       string
	Model          string
	SystemPrompt   string
	CustomHeaders  string
	db             DBInterface // Store DB reference for proxy updates
	client         *ai.Client
	httpClient     *http.Client // Store HTTP client to preserve proxy settings
	useGlobalProxy bool         // Store whether to use global proxy
}

// NewAITranslator creates a new AI translator with the given credentials.
// endpoint should be the full API URL (e.g., "https://api.openai.com/v1/chat/completions" for OpenAI, "http://localhost:11434/api/generate" for Ollama)
// model should be the model name (e.g., "gpt-4o-mini", "claude-3-haiku-20240307")
// Supports proxy via HTTP_PROXY, HTTPS_PROXY, ALL_PROXY environment variables.
// If db is provided, it will also check for database proxy settings (higher priority than env vars).
func NewAITranslator(apiKey, endpoint, model string, db ...DBInterface) *AITranslator {
	defaults := config.Get()
	if endpoint == "" {
		endpoint = defaults.AIEndpoint
	}
	if model == "" {
		model = defaults.AIModel
	}

	proxyURL := getProxyFromSettings(db...)

	clientConfig := ai.ClientConfig{
		APIKey:   apiKey,
		Endpoint: strings.TrimSuffix(endpoint, "/"),
		Model:    model,
		Timeout:  60 * time.Second,
		ProxyURL: proxyURL,
	}

	return &AITranslator{
		APIKey:        apiKey,
		Endpoint:      strings.TrimSuffix(endpoint, "/"),
		Model:         model,
		SystemPrompt:  "",
		CustomHeaders: "",
		db:            getDBFromSlice(db),
		client:        ai.NewClient(clientConfig),
	}
}

// getDBFromSlice extracts DBInterface from variadic arguments
func getDBFromSlice(dbArgs []DBInterface) DBInterface {
	if len(dbArgs) > 0 {
		return dbArgs[0]
	}
	return nil
}

// getProxyFromSettings retrieves proxy URL from database (higher priority) or environment variables
func getProxyFromSettings(dbArgs ...DBInterface) string {
	// First try database settings if available
	if len(dbArgs) > 0 && dbArgs[0] != nil {
		db := dbArgs[0]
		proxyEnabled, _ := db.GetSetting("proxy_enabled")
		if proxyEnabled == "true" {
			proxyType, _ := db.GetSetting("proxy_type")
			proxyHost, _ := db.GetSetting("proxy_host")
			proxyPort, _ := db.GetSetting("proxy_port")
			proxyUsername, _ := db.GetEncryptedSetting("proxy_username")
			proxyPassword, _ := db.GetEncryptedSetting("proxy_password")
			proxyURL := httputil.BuildProxyURL(proxyType, proxyHost, proxyPort, proxyUsername, proxyPassword)
			if proxyURL != "" {
				return proxyURL
			}
		}
	}

	// Fallback to environment variables
	return getProxyFromEnv()
}

func getProxyFromEnv() string {
	if proxyURL := os.Getenv("HTTP_PROXY"); proxyURL != "" {
		return proxyURL
	}
	if proxyURL := os.Getenv("HTTPS_PROXY"); proxyURL != "" {
		return proxyURL
	}
	if proxyURL := os.Getenv("ALL_PROXY"); proxyURL != "" {
		return proxyURL
	}
	return ""
}

// NewAITranslatorWithDB creates a new AI translator with database for proxy support
func NewAITranslatorWithDB(apiKey, endpoint, model string, db DBInterface, useGlobalProxy ...bool) *AITranslator {
	defaults := config.Get()
	if endpoint == "" {
		endpoint = defaults.AIEndpoint
	}
	if model == "" {
		model = defaults.AIModel
	}

	useProxy := true
	if len(useGlobalProxy) > 0 {
		useProxy = useGlobalProxy[0]
	}

	httpClient, err := CreateHTTPClientWithProxyOption(db, 60*time.Second, useProxy)
	if err != nil {
		httpClient = httputil.GetPooledAIHTTPClient("", 60*time.Second)
	}

	clientConfig := ai.ClientConfig{
		APIKey:   apiKey,
		Endpoint: strings.TrimSuffix(endpoint, "/"),
		Model:    model,
		Timeout:  60 * time.Second,
	}

	return &AITranslator{
		APIKey:         apiKey,
		Endpoint:       strings.TrimSuffix(endpoint, "/"),
		Model:          model,
		SystemPrompt:   "",
		CustomHeaders:  "", // Will be set from settings when used
		db:             db,
		httpClient:     httpClient,
		client:         ai.NewClientWithHTTPClient(clientConfig, httpClient),
		useGlobalProxy: useProxy,
	}
}

// SetSystemPrompt sets a custom system prompt for the translator.
func (t *AITranslator) SetSystemPrompt(prompt string) {
	t.SystemPrompt = prompt
	// Re-create client with updated system prompt, preserving HTTP client
	t.recreateClient()
}

// SetCustomHeaders sets custom headers for AI requests.
func (t *AITranslator) SetCustomHeaders(headers string) {
	t.CustomHeaders = headers
	// Re-create client with updated custom headers, preserving HTTP client
	t.recreateClient()
}

// recreateClient re-creates the AI client with current configuration
// Preserves the HTTP client (and its proxy settings) if available
func (t *AITranslator) recreateClient() {
	clientConfig := ai.ClientConfig{
		APIKey:        t.APIKey,
		Endpoint:      t.Endpoint,
		Model:         t.Model,
		SystemPrompt:  t.SystemPrompt,
		CustomHeaders: t.CustomHeaders,
		Timeout:       60 * time.Second,
	}
	if t.httpClient != nil {
		t.client = ai.NewClientWithHTTPClient(clientConfig, t.httpClient)
	} else {
		t.client = ai.NewClient(clientConfig)
	}
}

// RefreshProxy refreshes the HTTP client with current proxy settings from database
// This allows proxy changes to take effect without restarting the application
func (t *AITranslator) RefreshProxy() {
	if t.db == nil {
		return
	}

	httpClient, err := CreateHTTPClientWithProxyOption(t.db, 60*time.Second, t.useGlobalProxy)
	if err != nil {
		httpClient = httputil.GetPooledAIHTTPClient("", 60*time.Second)
	}
	t.httpClient = httpClient

	clientConfig := ai.ClientConfig{
		APIKey:        t.APIKey,
		Endpoint:      t.Endpoint,
		Model:         t.Model,
		SystemPrompt:  t.SystemPrompt,
		CustomHeaders: t.CustomHeaders,
		Timeout:       60 * time.Second,
	}
	t.client = ai.NewClientWithHTTPClient(clientConfig, t.httpClient)
}

// Translate translates text to the target language using an OpenAI-compatible API.
// Automatically detects and adapts to different API formats (Gemini, OpenAI, Ollama).
func (t *AITranslator) Translate(text, targetLang string) (string, error) {
	if text == "" {
		return "", nil
	}

	langName := getLanguageName(targetLang)

	// Use custom system prompt if provided, otherwise use default
	systemPrompt := t.SystemPrompt
	if systemPrompt == "" {
		systemPrompt = "You are a translator. Translate the given text accurately. Output ONLY the translated text, nothing else."
	}
	userPrompt := fmt.Sprintf("Translate to %s:\n%s", langName, text)

	// Use the universal client which handles format detection automatically
	result, err := t.client.RequestWithThinking(systemPrompt, userPrompt)
	if err != nil {
		return "", err
	}

	// Clean up the response - remove any quotes or extra whitespace
	translated := strings.TrimSpace(result.Content)
	translated = strings.Trim(translated, "\"'")
	return translated, nil
}

// getLanguageName converts a language code to a human-readable name.
func getLanguageName(code string) string {
	langNames := map[string]string{
		"en":    "English",
		"zh":    "Simplified Chinese",
		"zh-TW": "Traditional Chinese",
		"es":    "Spanish",
		"fr":    "French",
		"de":    "German",
		"ja":    "Japanese",
		"ko":    "Korean",
		"pt":    "Portuguese",
		"ru":    "Russian",
		"it":    "Italian",
		"ar":    "Arabic",
	}
	if name, ok := langNames[code]; ok {
		return name
	}
	return code
}
