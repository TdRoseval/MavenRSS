package translation

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"
)

type GoogleFreeTranslator struct {
	client *http.Client
	db     DBInterface
}

// NewGoogleFreeTranslator creates a new Google Free Translator
// db is optional - if nil, no proxy will be used
func NewGoogleFreeTranslator() *GoogleFreeTranslator {
	return &GoogleFreeTranslator{
		client: &http.Client{Timeout: 10 * time.Second},
		db:     nil,
	}
}

// NewGoogleFreeTranslatorWithDB creates a new Google Free Translator with database for proxy support
func NewGoogleFreeTranslatorWithDB(db DBInterface) *GoogleFreeTranslator {
	client, err := CreateHTTPClientWithProxy(db, 10*time.Second)
	if err != nil {
		// Fallback to default client if proxy creation fails
		client = &http.Client{Timeout: 10 * time.Second}
	}
	return &GoogleFreeTranslator{
		client: client,
		db:     db,
	}
}

func (t *GoogleFreeTranslator) Translate(text, targetLang string) (string, error) {
	if text == "" {
		return "", nil
	}

	// Get the configured endpoint, default to translate.googleapis.com
	endpoint := "translate.googleapis.com"
	if t.db != nil {
		if configuredEndpoint, err := t.db.GetSetting("google_translate_endpoint"); err == nil && configuredEndpoint != "" {
			endpoint = configuredEndpoint
		}
	}

	// Determine which client parameter and path to use based on endpoint
	var baseURL string
	var clientParam string

	if endpoint == "clients5.google.com" {
		baseURL = "https://clients5.google.com/translate_a/t"
		clientParam = "dict-chrome-ex"
	} else {
		// Default to translate.googleapis.com or any other endpoint
		baseURL = "https://" + endpoint + "/translate_a/single"
		clientParam = "gtx"
	}

	u, err := url.Parse(baseURL)
	if err != nil {
		return "", err
	}

	q := u.Query()
	q.Set("client", clientParam)
	q.Set("sl", "auto")
	// Map zh-TW to zh-TW for Google Translate
	// Google Translate uses "zh-TW" for Traditional Chinese and "zh-CN" for Simplified
	googleLang := targetLang
	if targetLang == "zh" {
		googleLang = "zh-CN"
	}
	q.Set("tl", googleLang)
	q.Set("dt", "t")
	q.Set("q", text)
	u.RawQuery = q.Encode()

	resp, err := t.client.Get(u.String())
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("translation api returned status: %d", resp.StatusCode)
	}

	// The response is a complex nested array structure
	// [[[ "translated", "original", ... ]], ...]
	var result []interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}

	if len(result) > 0 {
		if inner, ok := result[0].([]interface{}); ok {
			var translatedText string
			for _, slice := range inner {
				if s, ok := slice.([]interface{}); ok && len(s) > 0 {
					if str, ok := s[0].(string); ok {
						translatedText += str
					}
				}
			}
			if translatedText != "" {
				return translatedText, nil
			}
		}
	}

	return "", fmt.Errorf("invalid response format")
}
