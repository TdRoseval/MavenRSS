package translation

import (
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"MrRSS/internal/ai"
	"MrRSS/internal/config"
)

// AITranslator implements translation using OpenAI-compatible APIs (GPT, Claude, etc.).
type AITranslator struct {
	APIKey        string
	Endpoint      string
	Model         string
	SystemPrompt  string
	CustomHeaders string
	client        *ai.Client
	httpClient    *http.Client // Store HTTP client to preserve proxy settings
}

// NewAITranslator creates a new AI translator with the given credentials.
// endpoint should be the full API URL (e.g., "https://api.openai.com/v1/chat/completions" for OpenAI, "http://localhost:11434/api/generate" for Ollama)
// model should be the model name (e.g., "gpt-4o-mini", "claude-3-haiku-20240307")
// Supports proxy via HTTP_PROXY, HTTPS_PROXY, ALL_PROXY environment variables.
func NewAITranslator(apiKey, endpoint, model string) *AITranslator {
	defaults := config.Get()
	if endpoint == "" {
		endpoint = defaults.AIEndpoint
	}
	if model == "" {
		model = defaults.AIModel
	}

	proxyURL := getProxyFromEnv()

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
		client:        ai.NewClient(clientConfig),
	}
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
		// Fallback to default client if proxy creation fails
		httpClient = &http.Client{Timeout: 60 * time.Second}
	}

	clientConfig := ai.ClientConfig{
		APIKey:   apiKey,
		Endpoint: strings.TrimSuffix(endpoint, "/"),
		Model:    model,
		Timeout:  60 * time.Second,
	}

	return &AITranslator{
		APIKey:        apiKey,
		Endpoint:      strings.TrimSuffix(endpoint, "/"),
		Model:         model,
		SystemPrompt:  "",
		CustomHeaders: "", // Will be set from settings when used
		httpClient:    httpClient,
		client:        ai.NewClientWithHTTPClient(clientConfig, httpClient),
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
