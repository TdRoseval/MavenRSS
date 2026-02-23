package translation

import (
	"crypto/md5"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"math/big"
	"net/http"
	"net/url"
	"time"

	"MavenRSS/internal/utils/httputil"
)

type BaiduTranslator struct {
	AppID     string
	SecretKey string
	client    *http.Client
	db        DBInterface
}

func NewBaiduTranslator(appID, secretKey string) *BaiduTranslator {
	return &BaiduTranslator{
		AppID:     appID,
		SecretKey: secretKey,
		client:    httputil.GetPooledHTTPClient("", 30*time.Second),
		db:        nil,
	}
}

func NewBaiduTranslatorWithDB(appID, secretKey string, db DBInterface) *BaiduTranslator {
	client, err := CreateHTTPClientWithProxy(db, 30*time.Second)
	if err != nil {
		client = httputil.GetPooledHTTPClient("", 30*time.Second)
	}
	return &BaiduTranslator{
		AppID:     appID,
		SecretKey: secretKey,
		client:    client,
		db:        db,
	}
}

func (t *BaiduTranslator) RefreshProxy() {
	if t.db == nil {
		return
	}

	client, err := CreateHTTPClientWithProxy(t.db, 30*time.Second)
	if err != nil {
		client = httputil.GetPooledHTTPClient("", 30*time.Second)
	}
	t.client = client
}

func (t *BaiduTranslator) Translate(text, targetLang string) (string, error) {
	if text == "" {
		return "", nil
	}

	baiduLang := mapToBaiduLang(targetLang)

	n, err := rand.Int(rand.Reader, big.NewInt(1000000000))
	if err != nil {
		return "", fmt.Errorf("failed to generate salt: %w", err)
	}
	salt := n.String()

	signStr := t.AppID + text + salt + t.SecretKey
	hash := md5.Sum([]byte(signStr))
	sign := hex.EncodeToString(hash[:])

	apiURL := "https://fanyi-api.baidu.com/api/trans/vip/translate"
	data := url.Values{}
	data.Set("q", text)
	data.Set("from", "auto")
	data.Set("to", baiduLang)
	data.Set("appid", t.AppID)
	data.Set("salt", salt)
	data.Set("sign", sign)

	var result struct {
		ErrorCode   string `json:"error_code"`
		ErrorMsg    string `json:"error_msg"`
		TransResult []struct {
			Src string `json:"src"`
			Dst string `json:"dst"`
		} `json:"trans_result"`
	}

	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		resp, err := t.client.PostForm(apiURL, data)
		if err != nil {
			lastErr = err
			if httputil.IsNetworkError(err.Error()) && attempt < 2 {
				log.Printf("[Baidu] Network error on attempt %d/3, retrying: %v", attempt+1, err)
				time.Sleep(time.Duration(attempt+1) * time.Second)
				continue
			}
			return "", fmt.Errorf("baidu api request failed: %w", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			lastErr = fmt.Errorf("baidu api returned status: %d", resp.StatusCode)
			if resp.StatusCode >= 500 && attempt < 2 {
				log.Printf("[Baidu] Server error on attempt %d/3, retrying", attempt+1)
				time.Sleep(time.Duration(attempt+1) * time.Second)
				continue
			}
			return "", lastErr
		}

		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			return "", fmt.Errorf("failed to decode baidu response: %w", err)
		}

		break
	}

	if result.ErrorCode != "" && result.ErrorCode != "52000" {
		return "", fmt.Errorf("baidu api error: %s - %s", result.ErrorCode, result.ErrorMsg)
	}

	if len(result.TransResult) > 0 {
		return result.TransResult[0].Dst, nil
	}

	return "", fmt.Errorf("no translation found in baidu response")
}

func mapToBaiduLang(lang string) string {
	langMap := map[string]string{
		"en":    "en",
		"zh":    "zh",
		"zh-TW": "cht",
		"es":    "spa",
		"fr":    "fra",
		"de":    "de",
		"ja":    "jp",
		"ko":    "kor",
		"pt":    "pt",
		"ru":    "ru",
		"it":    "it",
		"ar":    "ara",
	}
	if baiduLang, ok := langMap[lang]; ok {
		return baiduLang
	}
	return lang
}
