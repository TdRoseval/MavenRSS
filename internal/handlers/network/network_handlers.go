package network

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	"MavenRSS/internal/handlers/core"
	"MavenRSS/internal/handlers/response"
	"MavenRSS/internal/network"
	"MavenRSS/internal/utils/httputil"
)

type StoredTestResult struct {
	URL        string `json:"url"`
	Domain     string `json:"domain"`
	Success    bool   `json:"success"`
	LatencyMs  int64  `json:"latency_ms,omitempty"`
	Error      string `json:"error,omitempty"`
	IsDomestic bool   `json:"is_domestic"`
	TestType   string `json:"test_type"`
}

type StoredDetectionResult struct {
	SpeedLevel       network.SpeedLevel `json:"speed_level"`
	BandwidthMbps    float64            `json:"bandwidth_mbps"`
	LatencyMs        int64              `json:"latency_ms"`
	MaxConcurrency   int                `json:"max_concurrency"`
	DetectionTime    time.Time          `json:"detection_time"`
	SuccessCount     int                `json:"success_count"`
	TotalCount       int                `json:"total_count"`
	URLTestResults   []StoredTestResult `json:"url_test_results"`
	DetectionSuccess bool               `json:"detection_success"`
	ErrorMessage     string             `json:"error_message,omitempty"`
}

func convertToStoredResult(r network.URLTestResult, testType string) StoredTestResult {
	return StoredTestResult{
		URL:        r.URL,
		Domain:     r.Domain,
		Success:    r.Success,
		LatencyMs:  r.LatencyMs,
		Error:      r.Error,
		IsDomestic: r.IsDomestic,
		TestType:   testType,
	}
}

func convertToStoredResults(results []network.URLTestResult) []StoredTestResult {
	stored := make([]StoredTestResult, 0, len(results))
	for _, r := range results {
		testType := "latency"
		if r.BandwidthMbps > 0 || r.BytesRead > 0 {
			testType = "bandwidth"
		}
		stored = append(stored, convertToStoredResult(r, testType))
	}
	return stored
}

func convertFromStoredResults(stored []StoredTestResult) []network.URLTestResult {
	results := make([]network.URLTestResult, 0, len(stored))
	for _, s := range stored {
		results = append(results, network.URLTestResult{
			URL:        s.URL,
			Domain:     s.Domain,
			Success:    s.Success,
			LatencyMs:  s.LatencyMs,
			Error:      s.Error,
			IsDomestic: s.IsDomestic,
		})
	}
	return results
}

func HandleDetectNetwork(h *core.Handler, w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		response.Error(w, nil, http.StatusMethodNotAllowed)
		return
	}

	userID, ok := core.GetUserIDFromRequest(r)

	var proxyEnabled, proxyType, proxyHost, proxyPort string
	var proxyUsername, proxyPassword string

	if ok {
		proxyEnabled, _ = h.DB.GetSettingForUser(userID, "proxy_enabled")
		proxyType, _ = h.DB.GetSettingForUser(userID, "proxy_type")
		proxyHost, _ = h.DB.GetSettingForUser(userID, "proxy_host")
		proxyPort, _ = h.DB.GetSettingForUser(userID, "proxy_port")
		proxyUsername, _ = h.DB.GetEncryptedSettingForUser(userID, "proxy_username")
		proxyPassword, _ = h.DB.GetEncryptedSettingForUser(userID, "proxy_password")

		// Fall back to global settings if user settings are empty
		if proxyEnabled == "" {
			proxyEnabled, _ = h.DB.GetSetting("proxy_enabled")
		}
		if proxyType == "" {
			proxyType, _ = h.DB.GetSetting("proxy_type")
		}
		if proxyHost == "" {
			proxyHost, _ = h.DB.GetSetting("proxy_host")
		}
		if proxyPort == "" {
			proxyPort, _ = h.DB.GetSetting("proxy_port")
		}
		if proxyUsername == "" {
			proxyUsername, _ = h.DB.GetEncryptedSetting("proxy_username")
		}
		if proxyPassword == "" {
			proxyPassword, _ = h.DB.GetEncryptedSetting("proxy_password")
		}
	} else {
		proxyEnabled, _ = h.DB.GetSetting("proxy_enabled")
		proxyType, _ = h.DB.GetSetting("proxy_type")
		proxyHost, _ = h.DB.GetSetting("proxy_host")
		proxyPort, _ = h.DB.GetSetting("proxy_port")
		proxyUsername, _ = h.DB.GetEncryptedSetting("proxy_username")
		proxyPassword, _ = h.DB.GetEncryptedSetting("proxy_password")
	}

	var httpClient *http.Client
	var proxyURL string
	if proxyEnabled == "true" {
		proxyURL = httputil.BuildProxyURL(proxyType, proxyHost, proxyPort, proxyUsername, proxyPassword)
		log.Printf("[NetworkDetect] Proxy enabled, userID=%v, using proxy URL: %s", userID, proxyURL)
		httpClient = httputil.GetPooledHTTPClient(proxyURL, 10*time.Second)
	} else {
		log.Printf("[NetworkDetect] Proxy disabled, userID=%v, using direct connection", userID)
		httpClient = httputil.GetPooledHTTPClient("", 10*time.Second)
	}

	detector := network.NewDetector(httpClient)
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	result := detector.DetectSpeed(ctx)

	if result.DetectionSuccess {
		h.DB.SetSetting("network_speed", string(result.SpeedLevel))
		h.DB.SetSetting("network_bandwidth_mbps", fmt.Sprintf("%.2f", result.BandwidthMbps))
		h.DB.SetSetting("network_latency_ms", strconv.FormatInt(result.LatencyMs, 10))
		h.DB.SetSetting("max_concurrent_refreshes", strconv.Itoa(result.MaxConcurrency))
		h.DB.SetSetting("last_network_test", result.DetectionTime.Format(time.RFC3339))
		h.DB.SetSetting("network_success_count", strconv.Itoa(result.SuccessCount))
		h.DB.SetSetting("network_total_count", strconv.Itoa(result.TotalCount))

		storedResults := convertToStoredResults(result.URLTestResults)
		if jsonData, err := json.Marshal(storedResults); err == nil {
			h.DB.SetSetting("network_test_results", string(jsonData))
		}
	}

	response.JSON(w, result)
}

func HandleGetNetworkInfo(h *core.Handler, w http.ResponseWriter, r *http.Request) {
	speedLevel, _ := h.DB.GetSetting("network_speed")
	bandwidthStr, _ := h.DB.GetSetting("network_bandwidth_mbps")
	latencyStr, _ := h.DB.GetSetting("network_latency_ms")
	concurrencyStr, _ := h.DB.GetSetting("max_concurrent_refreshes")
	lastTestStr, _ := h.DB.GetSetting("last_network_test")
	successCountStr, _ := h.DB.GetSetting("network_success_count")
	totalCountStr, _ := h.DB.GetSetting("network_total_count")
	testResultsJSON, _ := h.DB.GetSetting("network_test_results")

	bandwidth, _ := strconv.ParseFloat(bandwidthStr, 64)
	latency, _ := strconv.ParseInt(latencyStr, 10, 64)
	concurrency, _ := strconv.Atoi(concurrencyStr)
	if concurrency < 1 {
		concurrency = 10
	}
	successCount, _ := strconv.Atoi(successCountStr)
	totalCount, _ := strconv.Atoi(totalCountStr)

	var lastTest time.Time
	if lastTestStr != "" {
		lastTest, _ = time.Parse(time.RFC3339, lastTestStr)
	}

	var storedResults []StoredTestResult
	if testResultsJSON != "" {
		json.Unmarshal([]byte(testResultsJSON), &storedResults)
	}

	result := StoredDetectionResult{
		SpeedLevel:       network.SpeedLevel(speedLevel),
		BandwidthMbps:    bandwidth,
		LatencyMs:        latency,
		MaxConcurrency:   concurrency,
		DetectionTime:    lastTest,
		SuccessCount:     successCount,
		TotalCount:       totalCount,
		URLTestResults:   storedResults,
		DetectionSuccess: speedLevel != "" && successCount > 0,
	}

	response.JSON(w, result)
}

// HandleTestProxy tests the current proxy configuration
// @Summary      Test proxy connection
// @Description  Test if the configured proxy is working correctly
// @Tags         network
// @Accept       json
// @Produce      json
// @Success      200  {object}  httputil.ProxyTestResult  "Proxy test result"
// @Failure      400  {object}  map[string]string  "Bad request"
// @Router       /network/test-proxy [post]
func HandleTestProxy(h *core.Handler, w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		response.Error(w, nil, http.StatusMethodNotAllowed)
		return
	}

	userID, ok := core.GetUserIDFromRequest(r)

	var proxyEnabled, proxyType, proxyHost, proxyPort string
	var proxyUsername, proxyPassword string

	if ok {
		proxyEnabled, _ = h.DB.GetSettingForUser(userID, "proxy_enabled")
		proxyType, _ = h.DB.GetSettingForUser(userID, "proxy_type")
		proxyHost, _ = h.DB.GetSettingForUser(userID, "proxy_host")
		proxyPort, _ = h.DB.GetSettingForUser(userID, "proxy_port")
		proxyUsername, _ = h.DB.GetEncryptedSettingForUser(userID, "proxy_username")
		proxyPassword, _ = h.DB.GetEncryptedSettingForUser(userID, "proxy_password")

		// Fall back to global settings if user settings are empty
		if proxyEnabled == "" {
			proxyEnabled, _ = h.DB.GetSetting("proxy_enabled")
		}
		if proxyType == "" {
			proxyType, _ = h.DB.GetSetting("proxy_type")
		}
		if proxyHost == "" {
			proxyHost, _ = h.DB.GetSetting("proxy_host")
		}
		if proxyPort == "" {
			proxyPort, _ = h.DB.GetSetting("proxy_port")
		}
		if proxyUsername == "" {
			proxyUsername, _ = h.DB.GetEncryptedSetting("proxy_username")
		}
		if proxyPassword == "" {
			proxyPassword, _ = h.DB.GetEncryptedSetting("proxy_password")
		}
	} else {
		proxyEnabled, _ = h.DB.GetSetting("proxy_enabled")
		proxyType, _ = h.DB.GetSetting("proxy_type")
		proxyHost, _ = h.DB.GetSetting("proxy_host")
		proxyPort, _ = h.DB.GetSetting("proxy_port")
		proxyUsername, _ = h.DB.GetEncryptedSetting("proxy_username")
		proxyPassword, _ = h.DB.GetEncryptedSetting("proxy_password")
	}

	if proxyEnabled != "true" {
		response.JSON(w, httputil.ProxyTestResult{
			Success:      false,
			Error:        "proxy is not enabled",
			ProxyWorking: false,
		})
		return
	}

	proxyURL := httputil.BuildProxyURL(proxyType, proxyHost, proxyPort, proxyUsername, proxyPassword)

	if proxyURL == "" {
		response.JSON(w, httputil.ProxyTestResult{
			Success:      false,
			Error:        "proxy URL is empty (check host and port settings)",
			ProxyWorking: false,
		})
		return
	}

	if err := httputil.ValidateProxyURL(proxyURL); err != nil {
		response.JSON(w, httputil.ProxyTestResult{
			Success:      false,
			Error:        fmt.Sprintf("invalid proxy configuration: %v", err),
			ProxyWorking: false,
		})
		return
	}

	result := httputil.TestProxyWithDefaultURL(proxyURL, 15*time.Second)
	response.JSON(w, result)
}

// HandleTestCustomProxy tests a custom proxy configuration without saving it
// @Summary      Test custom proxy
// @Description  Test a custom proxy configuration without saving it
// @Tags         network
// @Accept       json
// @Produce      json
// @Param        request  body      object  true  "Proxy configuration"
// @Success      200  {object}  httputil.ProxyTestResult  "Proxy test result"
// @Failure      400  {object}  map[string]string  "Bad request"
// @Router       /network/test-custom-proxy [post]
func HandleTestCustomProxy(h *core.Handler, w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		response.Error(w, nil, http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		ProxyType string `json:"proxy_type"`
		ProxyHost string `json:"proxy_host"`
		ProxyPort string `json:"proxy_port"`
		Username  string `json:"username"`
		Password  string `json:"password"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, fmt.Errorf("invalid request body: %w", err), http.StatusBadRequest)
		return
	}

	proxyURL := httputil.BuildProxyURL(req.ProxyType, req.ProxyHost, req.ProxyPort, req.Username, req.Password)

	if proxyURL == "" {
		response.JSON(w, httputil.ProxyTestResult{
			Success:      false,
			Error:        "proxy URL is empty (check host and port)",
			ProxyWorking: false,
		})
		return
	}

	if err := httputil.ValidateProxyURL(proxyURL); err != nil {
		response.JSON(w, httputil.ProxyTestResult{
			Success:      false,
			Error:        fmt.Sprintf("invalid proxy configuration: %v", err),
			ProxyWorking: false,
		})
		return
	}

	result := httputil.TestProxyWithDefaultURL(proxyURL, 15*time.Second)
	response.JSON(w, result)
}
