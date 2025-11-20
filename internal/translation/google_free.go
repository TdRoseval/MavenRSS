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
}

func NewGoogleFreeTranslator() *GoogleFreeTranslator {
	return &GoogleFreeTranslator{
		client: &http.Client{Timeout: 10 * time.Second},
	}
}

func (t *GoogleFreeTranslator) Translate(text, targetLang string) (string, error) {
	if text == "" {
		return "", nil
	}

	baseURL := "https://translate.googleapis.com/translate_a/single"
	u, err := url.Parse(baseURL)
	if err != nil {
		return "", err
	}

	q := u.Query()
	q.Set("client", "gtx")
	q.Set("sl", "auto")
	q.Set("tl", targetLang)
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
