// Package httputil provides HTTP client utilities including proxy support,
// custom transports, and Cloudflare bypass functionality.
package httputil

import (
	"bytes"
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// BuildProxyURL constructs a proxy URL from settings.
// This is the canonical implementation used across all AI-related handlers.
func BuildProxyURL(proxyType, proxyHost, proxyPort, username, password string) string {
	if proxyHost == "" || proxyPort == "" {
		return ""
	}

	scheme := strings.ToLower(proxyType)
	switch scheme {
	case "socks5", "socks5h":
		scheme = "socks5"
	case "https", "http":
		scheme = "http"
	}

	if username != "" && password != "" {
		return fmt.Sprintf("%s://%s:%s@%s:%s",
			scheme,
			url.QueryEscape(username),
			url.QueryEscape(password),
			proxyHost,
			proxyPort)
	} else if username != "" {
		return fmt.Sprintf("%s://%s@%s:%s",
			scheme,
			url.QueryEscape(username),
			proxyHost,
			proxyPort)
	}

	return fmt.Sprintf("%s://%s:%s", scheme, proxyHost, proxyPort)
}

// ValidateProxyURL validates a proxy URL string and returns an error if invalid.
// This should be called before using the proxy URL to ensure it's properly formatted.
func ValidateProxyURL(proxyURL string) error {
	if proxyURL == "" {
		return nil
	}

	parsed, err := url.Parse(proxyURL)
	if err != nil {
		return fmt.Errorf("invalid proxy URL format: %w", err)
	}

	if parsed.Scheme != "http" && parsed.Scheme != "https" && 
	   parsed.Scheme != "socks5" && parsed.Scheme != "socks5h" {
		return fmt.Errorf("unsupported proxy scheme '%s': must be http, https, socks5, or socks5h", parsed.Scheme)
	}

	if parsed.Host == "" {
		return fmt.Errorf("proxy host is required")
	}

	return nil
}

// ValidateAndParseProxyURL validates and parses a proxy URL, returning the parsed URL or an error.
// Use this when you need both validation and the parsed URL.
func ValidateAndParseProxyURL(proxyURL string) (*url.URL, error) {
	if proxyURL == "" {
		return nil, nil
	}

	if err := ValidateProxyURL(proxyURL); err != nil {
		return nil, err
	}

	return url.Parse(proxyURL)
}

// CreateHTTPClient creates an HTTP client with optional proxy support.
// Deprecated: Use GetPooledHTTPClient for better connection reuse.
func CreateHTTPClient(proxyURL string, timeout time.Duration) (*http.Client, error) {
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{
			MinVersion:         tls.VersionTLS12,
			InsecureSkipVerify: false,
		},
		MaxIdleConns:          100,
		MaxIdleConnsPerHost:   20,
		MaxConnsPerHost:       50,
		IdleConnTimeout:       90 * time.Second,
		ForceAttemptHTTP2:     true,
		WriteBufferSize:       64 * 1024,
		ReadBufferSize:        64 * 1024,
		ResponseHeaderTimeout: 30 * time.Second,
		TLSHandshakeTimeout:   15 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}

	if proxyURL != "" {
		parsedProxy, err := ValidateAndParseProxyURL(proxyURL)
		if err != nil {
			return nil, fmt.Errorf("invalid proxy configuration: %w", err)
		}
		if parsedProxy != nil {
			transport.Proxy = http.ProxyURL(parsedProxy)
		}
	}

	return &http.Client{
		Transport: transport,
		Timeout:   timeout,
	}, nil
}

// CreateAIHTTPClient creates an HTTP client optimized for AI API requests.
// It has longer timeouts and better connection handling for AI services.
// Deprecated: Use GetPooledAIHTTPClient for better connection reuse.
func CreateAIHTTPClient(proxyURL string, timeout time.Duration) (*http.Client, error) {
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{
			MinVersion:         tls.VersionTLS12,
			InsecureSkipVerify: false,
		},
		MaxIdleConns:          50,
		MaxIdleConnsPerHost:   20,
		MaxConnsPerHost:       30,
		IdleConnTimeout:       90 * time.Second,
		ForceAttemptHTTP2:     false,
		ResponseHeaderTimeout: 60 * time.Second,
		TLSHandshakeTimeout:   20 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}

	if proxyURL != "" {
		parsedProxy, err := ValidateAndParseProxyURL(proxyURL)
		if err != nil {
			return nil, fmt.Errorf("invalid proxy configuration: %w", err)
		}
		if parsedProxy != nil {
			transport.Proxy = http.ProxyURL(parsedProxy)
		}
	}

	return &http.Client{
		Transport: transport,
		Timeout:   timeout,
	}, nil
}

// CreateHTTPClientWithUserAgent creates an HTTP client with custom User-Agent.
func CreateHTTPClientWithUserAgent(proxyURL string, timeout time.Duration, userAgent string) (*http.Client, error) {
	baseClient, err := CreateHTTPClient(proxyURL, timeout)
	if err != nil {
		return nil, err
	}

	baseClient.Transport = &UserAgentTransport{
		Original:  baseClient.Transport,
		userAgent: userAgent,
	}

	return baseClient, nil
}

// RoundTripFunc is an adapter for ordinary functions as http.RoundTripper.
type RoundTripFunc func(req *http.Request) (*http.Response, error)

// RoundTrip implements http.RoundTripper.
func (rt RoundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return rt(req)
}

// UserAgentTransport wraps http.RoundTripper to add User-Agent headers.
type UserAgentTransport struct {
	Original  http.RoundTripper
	userAgent string
}

// RoundTrip implements http.RoundTripper with automatic Cloudflare bypass.
func (t *UserAgentTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	return t.roundTripWithRetry(req, true)
}

func (t *UserAgentTransport) roundTripWithRetry(req *http.Request, useBrowserUA bool) (*http.Response, error) {
	if useBrowserUA {
		req.Header.Set("User-Agent", t.userAgent)
		req.Header.Set("Accept", "application/rss+xml, application/xml, text/xml, application/atom+xml;q=0.9,*/*;q=0.8")
		req.Header.Set("Accept-Language", "en-US,en;q=0.9,zh-CN;q=0.8,zh;q=0.7")
		req.Header.Set("DNT", "1")

		if req.Header.Get("Sec-Fetch-Dest") == "" {
			req.Header.Set("Sec-Fetch-Dest", "document")
		}
		if req.Header.Get("Sec-Fetch-Mode") == "" {
			req.Header.Set("Sec-Fetch-Mode", "navigate")
		}
		if req.Header.Get("Sec-Fetch-Site") == "" {
			req.Header.Set("Sec-Fetch-Site", "none")
		}
		if req.Header.Get("Sec-Fetch-User") == "" {
			req.Header.Set("Sec-Fetch-User", "?1")
		}
		if req.Header.Get("Cache-Control") == "" {
			req.Header.Set("Cache-Control", "max-age=0")
		}
	} else {
		req.Header.Set("User-Agent", "curl/8.11.1")
		req.Header.Set("Accept", "*/*")
		req.Header.Del("Sec-Fetch-Dest")
		req.Header.Del("Sec-Fetch-Mode")
		req.Header.Del("Sec-Fetch-Site")
		req.Header.Del("Sec-Fetch-User")
		req.Header.Del("Cache-Control")
		req.Header.Del("DNT")
		req.Header.Del("Accept-Language")
	}

	resp, err := t.Original.RoundTrip(req)
	if err != nil {
		return resp, err
	}

	if useBrowserUA && resp.StatusCode == 403 {
		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()

		if err != nil {
			return resp, fmt.Errorf("failed to read 403 response body: %w", err)
		}

		bodyStr := string(body)

		isCloudflare := strings.Contains(bodyStr, "Checking your browser") ||
			strings.Contains(bodyStr, "Cloudflare") ||
			strings.Contains(bodyStr, "cf_chl_opt") ||
			strings.Contains(bodyStr, "challenge-platform") ||
			strings.Contains(bodyStr, "jschl-answer") ||
			strings.Contains(bodyStr, "cf-browser-verification")

		if isCloudflare {
			retryResp, retryErr := t.roundTripWithRetry(req, false)
			if retryErr != nil {
				resp.Body = io.NopCloser(bytes.NewReader(body))
				return resp, retryErr
			}
			return retryResp, nil
		}

		resp.Body = io.NopCloser(bytes.NewReader(body))
	}

	return resp, nil
}

// ProxyTestResult contains the result of a proxy connection test
type ProxyTestResult struct {
	Success      bool          `json:"success"`
	Error        string        `json:"error,omitempty"`
	LatencyMs    int64         `json:"latency_ms,omitempty"`
	StatusCode   int           `json:"status_code,omitempty"`
	TestURL      string        `json:"test_url"`
	ProxyWorking bool          `json:"proxy_working"`
}

// TestProxyConnection tests if a proxy is working by making a request through it
// testURL should be a reliable, fast-responding URL (e.g., http://httpbin.org/ip or https://api.ipify.org)
func TestProxyConnection(proxyURL, testURL string, timeout time.Duration) ProxyTestResult {
	result := ProxyTestResult{
		TestURL: testURL,
	}

	if proxyURL == "" {
		result.Error = "no proxy URL provided"
		result.ProxyWorking = false
		return result
	}

	if err := ValidateProxyURL(proxyURL); err != nil {
		result.Error = fmt.Sprintf("invalid proxy URL: %v", err)
		result.ProxyWorking = false
		return result
	}

	client := GetPooledHTTPClient(proxyURL, timeout)

	start := time.Now()
	req, err := http.NewRequest("GET", testURL, nil)
	if err != nil {
		result.Error = fmt.Sprintf("failed to create request: %v", err)
		result.ProxyWorking = false
		return result
	}

	req.Header.Set("User-Agent", "MrRSS-ProxyTest/1.0")
	req.Header.Set("Accept", "*/*")

	resp, err := client.Do(req)
	if err != nil {
		result.Error = fmt.Sprintf("proxy connection failed: %v", err)
		result.ProxyWorking = false
		return result
	}
	defer resp.Body.Close()

	result.LatencyMs = time.Since(start).Milliseconds()
	result.StatusCode = resp.StatusCode
	result.Success = resp.StatusCode < 400
	result.ProxyWorking = true

	if resp.StatusCode >= 400 {
		result.Error = fmt.Sprintf("proxy returned status %d", resp.StatusCode)
	}

	return result
}

// TestProxyWithDefaultURL tests a proxy using a default test URL
func TestProxyWithDefaultURL(proxyURL string, timeout time.Duration) ProxyTestResult {
	testURLs := []string{
		"https://httpbin.org/ip",
		"https://api.ipify.org?format=json",
		"https://www.google.com/favicon.ico",
	}

	for _, testURL := range testURLs {
		result := TestProxyConnection(proxyURL, testURL, timeout)
		if result.Success {
			return result
		}
	}

	return ProxyTestResult{
		Success:      false,
		Error:        "all test URLs failed",
		ProxyWorking: false,
	}
}
