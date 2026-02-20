package network

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"MrRSS/internal/utils/httputil"
)

type SpeedLevel string

const (
	SpeedSlow   SpeedLevel = "slow"
	SpeedMedium SpeedLevel = "medium"
	SpeedFast   SpeedLevel = "fast"
)

const maxTestRetries = 2

type URLTestResult struct {
	URL         string `json:"url"`
	Domain      string `json:"domain"`
	Success     bool   `json:"success"`
	LatencyMs   int64  `json:"latency_ms,omitempty"`
	Error       string `json:"error,omitempty"`
	StatusCode  int    `json:"status_code,omitempty"`
	BytesRead   int64  `json:"bytes_read,omitempty"`
	BandwidthMbps float64 `json:"bandwidth_mbps,omitempty"`
	IsDomestic  bool   `json:"is_domestic"`
}

type DetectionResult struct {
	SpeedLevel       SpeedLevel      `json:"speed_level"`
	BandwidthMbps    float64         `json:"bandwidth_mbps"`
	LatencyMs        int64           `json:"latency_ms"`
	MaxConcurrency   int             `json:"max_concurrency"`
	DetectionTime    time.Time       `json:"detection_time"`
	SuccessCount     int             `json:"success_count"`
	TotalCount       int             `json:"total_count"`
	URLTestResults   []URLTestResult `json:"url_test_results"`
	DetectionSuccess bool            `json:"detection_success"`
	ErrorMessage     string          `json:"error_message,omitempty"`
}

type Detector struct {
	testURLs          []testURLInfo
	bandwidthTestURLs []testURLInfo
	httpClient        *http.Client
}

type testURLInfo struct {
	URL        string
	IsDomestic bool
}

func NewDetector(httpClient *http.Client) *Detector {
	return &Detector{
		testURLs: []testURLInfo{
			{URL: "https://www.baidu.com/favicon.ico", IsDomestic: true},
			{URL: "https://www.qq.com/favicon.ico", IsDomestic: true},
			{URL: "https://www.aliyun.com/favicon.ico", IsDomestic: true},
			{URL: "https://www.bing.com/favicon.ico", IsDomestic: false},
			{URL: "https://www.google.com/favicon.ico", IsDomestic: false},
			{URL: "https://www.cloudflare.com/favicon.ico", IsDomestic: false},
			{URL: "https://www.github.com/favicon.ico", IsDomestic: false},
		},
		bandwidthTestURLs: []testURLInfo{
			{URL: "https://www.baidu.com/img/flexible/logo/pc/result.png", IsDomestic: true},
			{URL: "https://www.aliyun.com/favicon.ico", IsDomestic: true},
			{URL: "https://www.bing.com/s/a/bing_p.ico", IsDomestic: false},
			{URL: "https://www.google.com/images/branding/googlelogo/2x/googlelogo_color_272x92dp.png", IsDomestic: false},
		},
		httpClient: httpClient,
	}
}

func extractDomain(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}
	return parsed.Hostname()
}

func (d *Detector) DetectSpeed(ctx context.Context) DetectionResult {
	result := DetectionResult{
		SpeedLevel:     SpeedMedium,
		MaxConcurrency: 10,
		URLTestResults: make([]URLTestResult, 0),
	}

	latencyResults := d.testAllLatency(ctx)
	result.URLTestResults = append(result.URLTestResults, latencyResults...)

	var totalLatency int64
	var latencySuccessCount int
	for _, r := range latencyResults {
		if r.Success {
			totalLatency += r.LatencyMs
			latencySuccessCount++
		}
	}

	bandwidthResults := d.testAllBandwidth(ctx)
	result.URLTestResults = append(result.URLTestResults, bandwidthResults...)

	var bandwidth float64
	var bandwidthSuccessCount int
	for _, r := range bandwidthResults {
		if r.Success && r.BandwidthMbps > 0 {
			bandwidth = r.BandwidthMbps
			bandwidthSuccessCount++
			break
		}
	}

	result.TotalCount = len(result.URLTestResults)
	result.SuccessCount = 0
	for _, r := range result.URLTestResults {
		if r.Success {
			result.SuccessCount++
		}
	}

	if latencySuccessCount == 0 {
		result.DetectionSuccess = false
		result.ErrorMessage = fmt.Sprintf("All latency tests failed (%d/%d URLs succeeded)", result.SuccessCount, result.TotalCount)
		log.Printf("Network detection failed: all latency tests failed")
		return result
	}

	result.LatencyMs = totalLatency / int64(latencySuccessCount)
	result.BandwidthMbps = bandwidth
	result.SpeedLevel, result.MaxConcurrency = d.calculateSpeedLevel(result.LatencyMs, result.BandwidthMbps)
	result.DetectionSuccess = true
	result.DetectionTime = time.Now()

	log.Printf("Network detection complete: %s (%.2f Mbps, %d ms latency, %d/%d URLs succeeded)",
		result.SpeedLevel, result.BandwidthMbps, result.LatencyMs, result.SuccessCount, result.TotalCount)

	return result
}

func (d *Detector) testAllLatency(ctx context.Context) []URLTestResult {
	results := make([]URLTestResult, 0, len(d.testURLs))

	log.Printf("[NetworkDetector] Starting latency tests with %d URLs", len(d.testURLs))

	for _, testInfo := range d.testURLs {
		result := URLTestResult{
			URL:        testInfo.URL,
			Domain:     extractDomain(testInfo.URL),
			IsDomestic: testInfo.IsDomestic,
		}

		var lastErr error
		for attempt := 0; attempt < maxTestRetries; attempt++ {
			select {
			case <-ctx.Done():
				result.Success = false
				result.Error = "Context cancelled"
				results = append(results, result)
				goto nextURL
			default:
			}

			log.Printf("[NetworkDetector] Testing latency for %s (attempt %d)", testInfo.URL, attempt+1)
			start := time.Now()
			req, err := http.NewRequestWithContext(ctx, "HEAD", testInfo.URL, nil)
			if err != nil {
				lastErr = err
				continue
			}

			resp, err := d.httpClient.Do(req)
			if err != nil {
				lastErr = err
				if httputil.IsNetworkError(err.Error()) && attempt < maxTestRetries-1 {
					backoff := httputil.CalculateBackoffSimple(attempt)
					time.Sleep(backoff)
					continue
				}
				result.Success = false
				result.Error = fmt.Sprintf("Request failed: %v", err)
				results = append(results, result)
				goto nextURL
			}
			resp.Body.Close()

			result.StatusCode = resp.StatusCode
			if resp.StatusCode >= 400 {
				lastErr = fmt.Errorf("HTTP %d", resp.StatusCode)
				if attempt < maxTestRetries-1 {
					backoff := httputil.CalculateBackoffSimple(attempt)
					time.Sleep(backoff)
					continue
				}
				result.Success = false
				result.Error = fmt.Sprintf("HTTP %d", resp.StatusCode)
			} else {
				result.Success = true
				result.LatencyMs = time.Since(start).Milliseconds()
				results = append(results, result)
				goto nextURL
			}
		}

		if lastErr != nil {
			result.Success = false
			result.Error = fmt.Sprintf("All attempts failed: %v", lastErr)
		}
		results = append(results, result)
	nextURL:
	}

	return results
}

func (d *Detector) testAllBandwidth(ctx context.Context) []URLTestResult {
	results := make([]URLTestResult, 0, len(d.bandwidthTestURLs))

	for _, testInfo := range d.bandwidthTestURLs {
		result := URLTestResult{
			URL:        testInfo.URL,
			Domain:     extractDomain(testInfo.URL),
			IsDomestic: testInfo.IsDomestic,
		}

		var lastErr error
		for attempt := 0; attempt < maxTestRetries; attempt++ {
			select {
			case <-ctx.Done():
				result.Success = false
				result.Error = "Context cancelled"
				results = append(results, result)
				goto nextBandwidthURL
			default:
			}

			req, err := http.NewRequestWithContext(ctx, "GET", testInfo.URL, nil)
			if err != nil {
				lastErr = err
				continue
			}

			start := time.Now()
			resp, err := d.httpClient.Do(req)
			if err != nil {
				lastErr = err
				if httputil.IsNetworkError(err.Error()) && attempt < maxTestRetries-1 {
					backoff := httputil.CalculateBackoffSimple(attempt)
					time.Sleep(backoff)
					continue
				}
				result.Success = false
				result.Error = fmt.Sprintf("Request failed: %v", err)
				results = append(results, result)
				goto nextBandwidthURL
			}

			result.StatusCode = resp.StatusCode
			if resp.StatusCode >= 400 {
				resp.Body.Close()
				lastErr = fmt.Errorf("HTTP %d", resp.StatusCode)
				if attempt < maxTestRetries-1 {
					backoff := httputil.CalculateBackoffSimple(attempt)
					time.Sleep(backoff)
					continue
				}
				result.Success = false
				result.Error = fmt.Sprintf("HTTP %d", resp.StatusCode)
				results = append(results, result)
				goto nextBandwidthURL
			}

			bytesRead, err := io.Copy(io.Discard, resp.Body)
			resp.Body.Close()
			if err != nil {
				lastErr = err
				if httputil.IsNetworkError(err.Error()) && attempt < maxTestRetries-1 {
					backoff := httputil.CalculateBackoffSimple(attempt)
					time.Sleep(backoff)
					continue
				}
				result.Success = false
				result.Error = fmt.Sprintf("Read failed: %v", err)
				results = append(results, result)
				goto nextBandwidthURL
			}

			duration := time.Since(start).Seconds()
			result.BytesRead = bytesRead
			result.Success = true

			if duration > 0 {
				bytesPerSecond := float64(bytesRead) / duration
				result.BandwidthMbps = (bytesPerSecond * 8) / (1024 * 1024)
			}

			results = append(results, result)
			goto nextBandwidthURL
		}

		if lastErr != nil {
			result.Success = false
			result.Error = fmt.Sprintf("All attempts failed: %v", lastErr)
		}
		results = append(results, result)
	nextBandwidthURL:
	}

	return results
}

func (d *Detector) calculateSpeedLevel(latencyMs int64, bandwidthMbps float64) (SpeedLevel, int) {
	var speedLevel SpeedLevel
	var maxConcurrency int

	if latencyMs > 300 || bandwidthMbps < 2.0 {
		speedLevel = SpeedSlow
		maxConcurrency = 8
	} else if latencyMs > 100 || bandwidthMbps < 20.0 {
		speedLevel = SpeedMedium
		maxConcurrency = 12
	} else {
		speedLevel = SpeedFast
		maxConcurrency = 20
	}

	return speedLevel, maxConcurrency
}

func IsNetworkError(errMsg string) bool {
	errLower := strings.ToLower(errMsg)
	networkErrorPatterns := []string{
		"connection refused",
		"connection reset",
		"connection timed out",
		"timeout",
		"i/o timeout",
		"proxy",
		"handshake timeout",
		"no such host",
		"network is unreachable",
		"dns",
	}

	for _, pattern := range networkErrorPatterns {
		if strings.Contains(errLower, pattern) {
			return true
		}
	}
	return false
}
