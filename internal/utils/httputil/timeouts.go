package httputil

import "time"

const (
	DefaultRSSFetchTimeout     = 60 * time.Second
	DefaultRSSHubTimeout       = 60 * time.Second
	DefaultFreshRSSTimeout     = 60 * time.Second
	DefaultDiscoveryTimeout    = 60 * time.Second
	DefaultTranslationTimeout  = 60 * time.Second
	DefaultAITimeout           = 120 * time.Second
	DefaultMediaProxyTimeout   = 90 * time.Second
	DefaultMediaCacheTimeout   = 90 * time.Second
	DefaultNetworkDetectTimeout = 10 * time.Second
	DefaultWebpageProxyTimeout = 60 * time.Second
	
	MaxMediaDownloadTimeout = 180 * time.Second
	LargeFileSizeThreshold  = 10 * 1024 * 1024 // 10MB
)

type TimeoutConfig struct {
	RSSFetch     time.Duration
	RSSHub       time.Duration
	FreshRSS     time.Duration
	Discovery    time.Duration
	Translation  time.Duration
	AI           time.Duration
	MediaProxy   time.Duration
	MediaCache   time.Duration
	NetworkDetect time.Duration
	WebpageProxy time.Duration
}

var DefaultTimeoutConfig = TimeoutConfig{
	RSSFetch:      DefaultRSSFetchTimeout,
	RSSHub:        DefaultRSSHubTimeout,
	FreshRSS:      DefaultFreshRSSTimeout,
	Discovery:     DefaultDiscoveryTimeout,
	Translation:   DefaultTranslationTimeout,
	AI:            DefaultAITimeout,
	MediaProxy:    DefaultMediaProxyTimeout,
	MediaCache:    DefaultMediaCacheTimeout,
	NetworkDetect: DefaultNetworkDetectTimeout,
	WebpageProxy:  DefaultWebpageProxyTimeout,
}

func CalculateDynamicMediaTimeout(contentLength int64) time.Duration {
	if contentLength <= 0 {
		return DefaultMediaCacheTimeout
	}
	
	if contentLength > LargeFileSizeThreshold {
		extraMB := float64(contentLength-LargeFileSizeThreshold) / (1024 * 1024)
		extraTime := time.Duration(extraMB*3) * time.Second
		timeout := DefaultMediaCacheTimeout + extraTime
		if timeout > MaxMediaDownloadTimeout {
			return MaxMediaDownloadTimeout
		}
		return timeout
	}
	
	return DefaultMediaCacheTimeout
}
