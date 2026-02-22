package translation

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"MavenRSS/internal/utils/httputil"
)

type DeepLTranslator struct {
	APIKey   string
	Endpoint string
	client   *http.Client
	db       DBInterface
}

func NewDeepLTranslator(apiKey string) *DeepLTranslator {
	return NewDeepLTranslatorWithDB(apiKey, nil)
}

func NewDeepLTranslatorWithEndpoint(apiKey, endpoint string) *DeepLTranslator {
	return NewDeepLTranslatorWithEndpointAndDB(apiKey, endpoint, nil)
}

func NewDeepLTranslatorWithDB(apiKey string, db DBInterface) *DeepLTranslator {
	client, err := CreateHTTPClientWithProxy(db, httputil.DefaultTranslationTimeout)
	if err != nil {
		client = httputil.GetPooledAIHTTPClient("", httputil.DefaultTranslationTimeout)
	}
	return &DeepLTranslator{
		APIKey:   apiKey,
		Endpoint: "",
		client:   client,
		db:       db,
	}
}

func NewDeepLTranslatorWithEndpointAndDB(apiKey, endpoint string, db DBInterface) *DeepLTranslator {
	client, err := CreateHTTPClientWithProxy(db, httputil.DefaultTranslationTimeout)
	if err != nil {
		client = httputil.GetPooledAIHTTPClient("", httputil.DefaultTranslationTimeout)
	}
	return &DeepLTranslator{
		APIKey:   apiKey,
		Endpoint: strings.TrimSuffix(endpoint, "/"),
		client:   client,
		db:       db,
	}
}

func (t *DeepLTranslator) RefreshProxy() {
	if t.db == nil {
		return
	}

	client, err := CreateHTTPClientWithProxy(t.db, httputil.DefaultTranslationTimeout)
	if err != nil {
		client = httputil.GetPooledAIHTTPClient("", httputil.DefaultTranslationTimeout)
	}
	t.client = client
}

func (t *DeepLTranslator) Translate(text, targetLang string) (string, error) {
	if text == "" {
		return "", nil
	}

	if t.Endpoint != "" {
		return t.translateWithDeeplx(text, targetLang)
	}

	apiURL := "https://api.deepl.com/v2/translate"
	if strings.HasSuffix(t.APIKey, ":fx") {
		apiURL = "https://api-free.deepl.com/v2/translate"
	}

	data := url.Values{}
	data.Set("auth_key", t.APIKey)
	data.Set("text", text)
	data.Set("target_lang", strings.ToUpper(targetLang))

	var result struct {
		Translations []struct {
			Text string `json:"text"`
		} `json:"translations"`
	}

	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		resp, err := t.client.PostForm(apiURL, data)
		if err != nil {
			lastErr = err
			if httputil.IsNetworkError(err.Error()) && attempt < 2 {
				log.Printf("[DeepL] Network error on attempt %d/3, retrying: %v", attempt+1, err)
				time.Sleep(time.Duration(attempt+1) * time.Second)
				continue
			}
			return "", err
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			lastErr = fmt.Errorf("deepl api returned status: %d", resp.StatusCode)
			if resp.StatusCode >= 500 && attempt < 2 {
				log.Printf("[DeepL] Server error on attempt %d/3, retrying", attempt+1)
				time.Sleep(time.Duration(attempt+1) * time.Second)
				continue
			}
			return "", lastErr
		}

		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			return "", err
		}

		break
	}

	if len(result.Translations) > 0 {
		return result.Translations[0].Text, nil
	}

	return "", fmt.Errorf("no translation found")
}

func (t *DeepLTranslator) translateWithDeeplx(text, targetLang string) (string, error) {
	apiURL := t.Endpoint + "/translate"

	requestBody := map[string]string{
		"text":        text,
		"source_lang": "auto",
		"target_lang": strings.ToUpper(targetLang),
	}

	jsonBody, err := json.Marshal(requestBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal deeplx request: %w", err)
	}

	var result struct {
		Code    int      `json:"code"`
		Message string   `json:"message"`
		Data    string   `json:"data"`
		Alt     []string `json:"alternatives"`
	}

	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		req, err := http.NewRequest("POST", apiURL, bytes.NewBuffer(jsonBody))
		if err != nil {
			return "", fmt.Errorf("failed to create deeplx request: %w", err)
		}

		req.Header.Set("Content-Type", "application/json")
		if t.APIKey != "" {
			req.Header.Set("Authorization", "Bearer "+t.APIKey)
		}

		resp, err := t.client.Do(req)
		if err != nil {
			lastErr = err
			if httputil.IsNetworkError(err.Error()) && attempt < 2 {
				log.Printf("[DeepLX] Network error on attempt %d/3, retrying: %v", attempt+1, err)
				time.Sleep(time.Duration(attempt+1) * time.Second)
				continue
			}
			return "", fmt.Errorf("deeplx request failed: %w", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			lastErr = fmt.Errorf("deeplx returned status: %d", resp.StatusCode)
			if resp.StatusCode >= 500 && attempt < 2 {
				log.Printf("[DeepLX] Server error on attempt %d/3, retrying", attempt+1)
				time.Sleep(time.Duration(attempt+1) * time.Second)
				continue
			}
			return "", lastErr
		}

		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			return "", fmt.Errorf("failed to decode deeplx response: %w", err)
		}

		break
	}

	if result.Code != 200 {
		return "", fmt.Errorf("deeplx error: %s", result.Message)
	}

	if result.Data != "" {
		return result.Data, nil
	}

	return "", fmt.Errorf("no translation found from deeplx")
}
