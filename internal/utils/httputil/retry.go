package httputil

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net/http"
	"strings"
	"time"
)

const (
	DefaultMaxRetries     = 3
	DefaultInitialBackoff = 1 * time.Second
	DefaultMaxBackoff     = 10 * time.Second
	DefaultBackoffFactor  = 2.0
)

type RetryConfig struct {
	MaxAttempts     int
	InitialBackoff  time.Duration
	MaxBackoff      time.Duration
	BackoffFactor   float64
	RetryableErrors []string
}

var DefaultRetryConfig = RetryConfig{
	MaxAttempts:     DefaultMaxRetries,
	InitialBackoff:  DefaultInitialBackoff,
	MaxBackoff:      DefaultMaxBackoff,
	BackoffFactor:   DefaultBackoffFactor,
	RetryableErrors: []string{},
}

var NetworkRetryConfig = RetryConfig{
	MaxAttempts:    5,
	InitialBackoff: DefaultInitialBackoff,
	MaxBackoff:     30 * time.Second,
	BackoffFactor:  DefaultBackoffFactor,
	RetryableErrors: []string{
		"connection refused",
		"connection reset",
		"connection timed out",
		"timeout",
		"dial tcp",
		"i/o timeout",
		"tls:",
		"certificate",
		"x509",
		"proxy",
		"no such host",
		"handshake timeout",
		"connection closed",
		"broken pipe",
		"context deadline exceeded",
		"eof",
		"unexpected eof",
		"temporary failure",
		"network is unreachable",
		"no route to host",
	},
}

// CalculateBackoff calculates exponential backoff duration with optional jitter
// This is the canonical implementation for all network retry operations
func CalculateBackoff(attempt int, initialBackoff, maxBackoff time.Duration) time.Duration {
	backoff := float64(initialBackoff)
	for i := 0; i < attempt; i++ {
		backoff *= DefaultBackoffFactor
	}
	if backoff > float64(maxBackoff) {
		backoff = float64(maxBackoff)
	}
	// Add jitter (0-10% of backoff time)
	jitter := rand.Float64() * 0.1 * backoff
	return time.Duration(backoff + jitter)
}

// CalculateBackoffSimple calculates exponential backoff with default settings
func CalculateBackoffSimple(attempt int) time.Duration {
	return CalculateBackoff(attempt, DefaultInitialBackoff, DefaultMaxBackoff)
}

func DoWithRetry(ctx context.Context, client *http.Client, req *http.Request, config RetryConfig) (*http.Response, error) {
	var lastErr error
	var resp *http.Response

	for attempt := 0; attempt < config.MaxAttempts; attempt++ {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		reqCopy := copyRequest(req)

		resp, lastErr = client.Do(reqCopy)
		if lastErr == nil {
			if resp.StatusCode >= 500 || resp.StatusCode == 429 {
				resp.Body.Close()
				lastErr = fmt.Errorf("server error: %d", resp.StatusCode)
			} else {
				return resp, nil
			}
		}

		if !isRetryableError(lastErr.Error(), config.RetryableErrors) {
			return nil, lastErr
		}

		if attempt < config.MaxAttempts-1 {
			backoff := calculateBackoff(attempt, config)
			log.Printf("[Retry] Attempt %d/%d failed: %v, retrying in %v", attempt+1, config.MaxAttempts, lastErr, backoff)

			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoff):
			}
		}
	}

	return nil, fmt.Errorf("all %d attempts failed, last error: %w", config.MaxAttempts, lastErr)
}

func DoWithRetrySimple(client *http.Client, req *http.Request, config RetryConfig) (*http.Response, error) {
	return DoWithRetry(context.Background(), client, req, config)
}

func copyRequest(req *http.Request) *http.Request {
	reqCopy := req.Clone(req.Context())
	if req.Body != nil {
		bodyBytes, err := io.ReadAll(req.Body)
		if err == nil {
			req.Body = io.NopCloser(bytes.NewReader(bodyBytes))
			reqCopy.Body = io.NopCloser(bytes.NewReader(bodyBytes))
		}
	}
	return reqCopy
}

func calculateBackoff(attempt int, config RetryConfig) time.Duration {
	backoff := float64(config.InitialBackoff)
	for i := 0; i < attempt; i++ {
		backoff *= config.BackoffFactor
	}
	if backoff > float64(config.MaxBackoff) {
		backoff = float64(config.MaxBackoff)
	}
	jitter := rand.Float64() * 0.1 * backoff
	return time.Duration(backoff + jitter)
}

func isRetryableError(errMsg string, retryablePatterns []string) bool {
	if len(retryablePatterns) == 0 {
		retryablePatterns = NetworkRetryConfig.RetryableErrors
	}

	errLower := strings.ToLower(errMsg)
	for _, pattern := range retryablePatterns {
		if strings.Contains(errLower, strings.ToLower(pattern)) {
			return true
		}
	}
	return false
}

func IsNetworkError(errMsg string) bool {
	return isRetryableError(errMsg, NetworkRetryConfig.RetryableErrors)
}

type RetryableTransport struct {
	Transport http.RoundTripper
	Config    RetryConfig
}

func (t *RetryableTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	var lastErr error
	var resp *http.Response

	for attempt := 0; attempt < t.Config.MaxAttempts; attempt++ {
		resp, lastErr = t.Transport.RoundTrip(req)
		if lastErr == nil {
			if resp.StatusCode >= 500 || resp.StatusCode == 429 {
				resp.Body.Close()
				lastErr = fmt.Errorf("server error: %d", resp.StatusCode)
			} else {
				return resp, nil
			}
		}

		if !isRetryableError(lastErr.Error(), t.Config.RetryableErrors) {
			return nil, lastErr
		}

		if attempt < t.Config.MaxAttempts-1 {
			backoff := calculateBackoff(attempt, t.Config)
			select {
			case <-req.Context().Done():
				return nil, req.Context().Err()
			case <-time.After(backoff):
			}
		}
	}

	return nil, fmt.Errorf("all %d attempts failed: %w", t.Config.MaxAttempts, lastErr)
}

func NewRetryableTransport(transport http.RoundTripper, config RetryConfig) *RetryableTransport {
	return &RetryableTransport{
		Transport: transport,
		Config:    config,
	}
}
