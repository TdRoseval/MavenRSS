package network

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"
)

// SpeedLevel represents the detected network speed category
type SpeedLevel string

const (
	SpeedSlow   SpeedLevel = "slow"
	SpeedMedium SpeedLevel = "medium"
	SpeedFast   SpeedLevel = "fast"
)

// DetectionResult contains the results of network speed detection
type DetectionResult struct {
	SpeedLevel       SpeedLevel `json:"speed_level"`
	BandwidthMbps    float64    `json:"bandwidth_mbps"`
	LatencyMs        int64      `json:"latency_ms"`
	MaxConcurrency   int        `json:"max_concurrency"`
	DetectionTime    time.Time  `json:"detection_time"`
	DetectionSuccess bool       `json:"detection_success"`
	ErrorMessage     string     `json:"error_message,omitempty"`
}

// Detector handles network speed detection
type Detector struct {
	testURLs []string
	timeout  time.Duration
}

// NewDetector creates a new network speed detector
func NewDetector() *Detector {
	return &Detector{
		testURLs: []string{
			"https://www.google.com/favicon.ico",
			"https://www.cloudflare.com/favicon.ico",
			"https://www.github.com/favicon.ico",
			// Chinese domestic sites for better connectivity in China
			"https://www.baidu.com/favicon.ico",
			"https://www.qq.com/favicon.ico",
			"https://www.aliyun.com/favicon.ico",
		},
		timeout: 10 * time.Second,
	}
}

// DetectSpeed performs network speed detection
func (d *Detector) DetectSpeed(ctx context.Context) DetectionResult {
	result := DetectionResult{
		DetectionTime:  time.Now(),
		SpeedLevel:     SpeedMedium, // Default fallback
		MaxConcurrency: 5,           // Default fallback
	}

	// Test latency first
	latency, err := d.testLatency(ctx)
	if err != nil {
		result.DetectionSuccess = false
		result.ErrorMessage = fmt.Sprintf("Latency test failed: %v", err)
		log.Printf("Network detection failed: %v", err)
		return result
	}
	result.LatencyMs = latency

	// Test bandwidth
	bandwidth, err := d.testBandwidth(ctx)
	if err != nil {
		result.DetectionSuccess = false
		result.ErrorMessage = fmt.Sprintf("Bandwidth test failed: %v", err)
		log.Printf("Network bandwidth test failed: %v", err)
		return result
	}
	result.BandwidthMbps = bandwidth

	// Determine speed level and concurrency based on results
	result.SpeedLevel, result.MaxConcurrency = d.calculateSpeedLevel(latency, bandwidth)
	result.DetectionSuccess = true

	log.Printf("Network detection complete: %s (%.2f Mbps, %d ms latency, max concurrency: %d)",
		result.SpeedLevel, result.BandwidthMbps, result.LatencyMs, result.MaxConcurrency)

	return result
}

// testLatency measures network latency by pinging a reliable server
func (d *Detector) testLatency(ctx context.Context) (int64, error) {
	client := &http.Client{
		Timeout: d.timeout,
	}

	var totalLatency int64
	successCount := 0

	for _, url := range d.testURLs {
		start := time.Now()
		req, err := http.NewRequestWithContext(ctx, "HEAD", url, nil)
		if err != nil {
			continue
		}

		resp, err := client.Do(req)
		if err != nil {
			continue
		}
		defer resp.Body.Close()

		// Check if response is successful
		if resp.StatusCode >= 400 {
			continue
		}

		latency := time.Since(start).Milliseconds()
		totalLatency += latency
		successCount++

		// One successful test is enough
		break
	}

	if successCount == 0 {
		return 0, fmt.Errorf("all latency tests failed")
	}

	return totalLatency / int64(successCount), nil
}

// testBandwidth measures download bandwidth
func (d *Detector) testBandwidth(ctx context.Context) (float64, error) {
	// Use a small test file to measure bandwidth
	// We'll use a ~100KB test download
	testURL := "https://www.google.com/images/branding/googlelogo/2x/googlelogo_color_272x92dp.png"

	client := &http.Client{
		Timeout: d.timeout,
	}

	req, err := http.NewRequestWithContext(ctx, "GET", testURL, nil)
	if err != nil {
		return 0, err
	}

	start := time.Now()
	resp, err := client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	// Check if response is successful
	if resp.StatusCode >= 400 {
		return 0, fmt.Errorf("HTTP %d error", resp.StatusCode)
	}

	// Read the response body
	bytesRead, err := io.Copy(io.Discard, resp.Body)
	if err != nil {
		return 0, err
	}

	duration := time.Since(start).Seconds()
	if duration == 0 {
		return 0, fmt.Errorf("download completed too quickly to measure")
	}

	// Calculate bandwidth in Mbps
	bytesPerSecond := float64(bytesRead) / duration
	mbps := (bytesPerSecond * 8) / (1024 * 1024)

	return mbps, nil
}

// calculateSpeedLevel determines the speed level and appropriate concurrency
func (d *Detector) calculateSpeedLevel(latencyMs int64, bandwidthMbps float64) (SpeedLevel, int) {
	// Define thresholds
	// Slow: latency > 200ms OR bandwidth < 1 Mbps
	// Medium: latency 100-200ms OR bandwidth 1-10 Mbps
	// Fast: latency < 100ms AND bandwidth > 10 Mbps

	var speedLevel SpeedLevel
	var maxConcurrency int

	// Determine speed level based on both latency and bandwidth
	if latencyMs > 200 || bandwidthMbps < 1.0 {
		speedLevel = SpeedSlow
		maxConcurrency = 5
	} else if latencyMs > 100 || bandwidthMbps < 10.0 {
		speedLevel = SpeedMedium
		maxConcurrency = 8
	} else {
		speedLevel = SpeedFast
		maxConcurrency = 15
	}

	return speedLevel, maxConcurrency
}
