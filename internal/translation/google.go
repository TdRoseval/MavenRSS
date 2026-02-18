package translation

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"time"

	"MrRSS/internal/utils/httputil"
)

type GoogleFreeTranslator struct {
	client *http.Client
	db     DBInterface
}

func NewGoogleFreeTranslator() *GoogleFreeTranslator {
	return &GoogleFreeTranslator{
		client: httputil.GetPooledHTTPClient("", httputil.DefaultTranslationTimeout),
		db:     nil,
	}
}

func NewGoogleFreeTranslatorWithDB(db DBInterface) *GoogleFreeTranslator {
	client, err := CreateHTTPClientWithProxy(db, httputil.DefaultTranslationTimeout)
	if err != nil {
		client = httputil.GetPooledHTTPClient("", httputil.DefaultTranslationTimeout)
	}
	return &GoogleFreeTranslator{
		client: client,
		db:     db,
	}
}

func (t *GoogleFreeTranslator) RefreshProxy() {
	if t.db == nil {
		return
	}

	client, err := CreateHTTPClientWithProxy(t.db, httputil.DefaultTranslationTimeout)
	if err != nil {
		client = httputil.GetPooledHTTPClient("", httputil.DefaultTranslationTimeout)
	}
	t.client = client
}

func (t *GoogleFreeTranslator) Translate(text, targetLang string) (string, error) {
	if text == "" {
		return "", nil
	}

	endpoint := "translate.googleapis.com"
	if t.db != nil {
		if configuredEndpoint, err := t.db.GetSetting("google_translate_endpoint"); err == nil && configuredEndpoint != "" {
			endpoint = configuredEndpoint
		}
	}

	var baseURL string
	var clientParam string

	if endpoint == "clients5.google.com" {
		baseURL = "https://clients5.google.com/translate_a/t"
		clientParam = "dict-chrome-ex"
	} else {
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
	googleLang := targetLang
	if targetLang == "zh" {
		googleLang = "zh-CN"
	}
	q.Set("tl", googleLang)
	q.Set("dt", "t")
	q.Set("q", text)
	u.RawQuery = q.Encode()

	var result []interface{}
	var lastErr error
	const maxRetries = 3

	for attempt := 0; attempt < maxRetries; attempt++ {
		resp, err := t.client.Get(u.String())
		if err != nil {
			lastErr = err
			if httputil.IsNetworkError(err.Error()) && attempt < maxRetries-1 {
				backoff := httputil.CalculateBackoffSimple(attempt)
				log.Printf("[Google Translate] Network error on attempt %d/%d, retrying in %v: %v", attempt+1, maxRetries, backoff, err)
				time.Sleep(backoff)
				continue
			}
			return "", err
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			lastErr = fmt.Errorf("translation api returned status: %d", resp.StatusCode)
			if resp.StatusCode >= 500 && attempt < maxRetries-1 {
				backoff := httputil.CalculateBackoffSimple(attempt)
				log.Printf("[Google Translate] Server error on attempt %d/%d, retrying in %v", attempt+1, maxRetries, backoff)
				time.Sleep(backoff)
				continue
			}
			return "", lastErr
		}

		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			return "", err
		}

		break
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
