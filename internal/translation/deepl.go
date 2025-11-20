package translation

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type DeepLTranslator struct {
	APIKey string
	client *http.Client
}

func NewDeepLTranslator(apiKey string) *DeepLTranslator {
	return &DeepLTranslator{
		APIKey: apiKey,
		client: &http.Client{Timeout: 10 * time.Second},
	}
}

func (t *DeepLTranslator) Translate(text, targetLang string) (string, error) {
	if text == "" {
		return "", nil
	}

	apiURL := "https://api.deepl.com/v2/translate"
	if strings.HasSuffix(t.APIKey, ":fx") {
		apiURL = "https://api-free.deepl.com/v2/translate"
	}

	data := url.Values{}
	data.Set("auth_key", t.APIKey)
	data.Set("text", text)
	data.Set("target_lang", strings.ToUpper(targetLang))

	resp, err := t.client.PostForm(apiURL, data)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("deepl api returned status: %d", resp.StatusCode)
	}

	var result struct {
		Translations []struct {
			Text string `json:"text"`
		} `json:"translations"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}

	if len(result.Translations) > 0 {
		return result.Translations[0].Text, nil
	}

	return "", fmt.Errorf("no translation found")
}
