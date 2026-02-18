// Package cache provides media caching functionality for anti-hotlinking support.
package cache

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"MrRSS/internal/utils/httputil"
)

const (
	maxRetries              = 2
	largeFileSizeThreshold  = httputil.LargeFileSizeThreshold
	defaultDownloadTimeout  = httputil.DefaultMediaCacheTimeout
	maxDownloadTimeout      = httputil.MaxMediaDownloadTimeout
	totalMaxTimeout         = 60 * time.Second
	headRequestTimeout      = 3 * time.Second
)

// MediaCache handles caching of images and videos to work around anti-hotlinking
type MediaCache struct {
	cacheDir string
	proxyURL string
}

// NewMediaCache creates a new media cache instance
func NewMediaCache(cacheDir string) (*MediaCache, error) {
	return NewMediaCacheWithProxy(cacheDir, "")
}

// NewMediaCacheWithProxy creates a new media cache instance with proxy support
func NewMediaCacheWithProxy(cacheDir, proxyURL string) (*MediaCache, error) {
	// Create cache directory if it doesn't exist
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create cache directory: %w", err)
	}

	return &MediaCache{
		cacheDir: cacheDir,
		proxyURL: proxyURL,
	}, nil
}

// GetCachedPath returns the cached file path for a given URL (using extension from URL)
func (mc *MediaCache) GetCachedPath(url string) string {
	hash := hashURL(url)
	ext := getExtensionFromURL(url)
	return filepath.Join(mc.cacheDir, hash+ext)
}

// SetProxy updates the proxy URL for the media cache
func (mc *MediaCache) SetProxy(proxyURL string) {
	mc.proxyURL = proxyURL
}

// findCachedFile returns the path to a cached file for the given URL, regardless of extension.
func (mc *MediaCache) findCachedFile(url string) (string, bool) {
	hash := hashURL(url)
	pattern := filepath.Join(mc.cacheDir, hash+".*")
	matches, err := filepath.Glob(pattern)
	if err != nil || len(matches) == 0 {
		// Try also the case where there is no extension (rare, but possible)
		noExtPath := filepath.Join(mc.cacheDir, hash)
		if _, err := os.Stat(noExtPath); err == nil {
			return noExtPath, true
		}
		return "", false
	}
	// If multiple matches found, log warning and clean up duplicates
	if len(matches) > 1 {
		fmt.Printf("Warning: Found %d cached files for URL hash %s, cleaning up duplicates\n", len(matches), hash)
		// Keep the most recent file, remove others
		for i := 1; i < len(matches); i++ {
			if err := os.Remove(matches[i]); err != nil {
				fmt.Printf("Failed to remove duplicate cache file %s: %v\n", matches[i], err)
			}
		}
	}
	return matches[0], true
}

// Exists checks if a media file is already cached (regardless of extension)
func (mc *MediaCache) Exists(url string) bool {
	_, found := mc.findCachedFile(url)
	return found
}

// Get retrieves cached media or downloads it if not cached
func (mc *MediaCache) Get(url, referer string) ([]byte, string, error) {
	return mc.GetWithContext(context.Background(), url, referer)
}

// GetWithContext retrieves cached media or downloads it with context support
func (mc *MediaCache) GetWithContext(ctx context.Context, url, referer string) ([]byte, string, error) {
	// Check if already cached
	cachedPath, found := mc.findCachedFile(url)
	if found {
		data, err := os.ReadFile(cachedPath)
		if err != nil {
			return nil, "", fmt.Errorf("failed to read cached file: %w", err)
		}
		contentType := getContentTypeFromPath(cachedPath)
		return data, contentType, nil
	}

	// Download and cache
	data, contentType, err := mc.downloadWithContext(ctx, url, referer)
	if err != nil {
		return nil, "", fmt.Errorf("failed to download media: %w", err)
	}

	// Determine better file extension from Content-Type if available
	if contentType != "" {
		betterExt := getExtensionFromContentType(contentType)
		if betterExt != "" {
			// Update cached path with correct extension
			cachedPath = filepath.Join(mc.cacheDir, hashURL(url)+betterExt)
		}
	}

	// Save to cache
	if err := os.WriteFile(cachedPath, data, 0644); err != nil {
		return nil, "", fmt.Errorf("failed to cache media: %w", err)
	}

	return data, contentType, nil
}

// download fetches media from the given URL with proper headers, retry support, and dynamic timeout
func (mc *MediaCache) download(urlStr, referer string) ([]byte, string, error) {
	return mc.downloadWithContext(context.Background(), urlStr, referer)
}

// downloadWithContext fetches media with context support for cancellation
func (mc *MediaCache) downloadWithContext(ctx context.Context, urlStr, referer string) ([]byte, string, error) {
	var lastErr error
	var lastResp *http.Response
	var contentLength int64 = -1
	
	totalCtx, cancel := context.WithTimeout(ctx, totalMaxTimeout)
	defer cancel()
	
	// Try a HEAD request with a very short timeout - if it fails quickly, just skip it
	// This avoids adding unnecessary latency for requests that don't support HEAD
	headCtx, headCancel := context.WithTimeout(totalCtx, headRequestTimeout)
	defer headCancel()
	
	headClient := httputil.GetPooledHTTPClient(mc.proxyURL, headRequestTimeout)
	headReq, err := http.NewRequestWithContext(headCtx, "HEAD", urlStr, nil)
	if err == nil {
		userAgent := "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"
		headReq.Header.Set("User-Agent", userAgent)
		smartReferer := getSmartReferer(urlStr, referer)
		if smartReferer != "" {
			headReq.Header.Set("Referer", smartReferer)
		}
		
		headResp, headErr := headClient.Do(headReq)
		if headErr == nil {
			defer headResp.Body.Close()
			if headResp.StatusCode == http.StatusOK {
				if cl := headResp.Header.Get("Content-Length"); cl != "" {
					if size, err := parseContentLength(cl); err == nil {
						contentLength = size
						log.Printf("[MediaCache] HEAD request successful, Content-Length: %d bytes", contentLength)
					}
				}
			}
		}
	}
	
	for attempt := 0; attempt < maxRetries; attempt++ {
		select {
		case <-totalCtx.Done():
			return nil, "", fmt.Errorf("total timeout exceeded after %v: %w", totalMaxTimeout, totalCtx.Err())
		default:
		}
		
		var timeout time.Duration
		if contentLength > 0 {
			timeout = httputil.CalculateDynamicMediaTimeout(contentLength)
		} else if lastResp != nil {
			timeout = mc.calculateTimeout(urlStr, lastResp)
		} else {
			timeout = defaultDownloadTimeout
		}
		
		client := httputil.GetPooledHTTPClient(mc.proxyURL, timeout)

		req, err := http.NewRequestWithContext(totalCtx, "GET", urlStr, nil)
		if err != nil {
			return nil, "", fmt.Errorf("failed to create request: %w", err)
		}

		// Set headers to bypass anti-hotlinking - try multiple user agents
		userAgents := []string{
			"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
			"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
			"Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
		}

		req.Header.Set("User-Agent", userAgents[0]) // Start with Windows Chrome

		// CRITICAL FIX: Use smart referer logic to handle cases where the original referer would be blocked
		smartReferer := getSmartReferer(urlStr, referer)
		if smartReferer != "" {
			req.Header.Set("Referer", smartReferer)
		}

		// Add additional headers to bypass restrictions
		// Note: Don't set Accept-Encoding - let Go's http.Transport handle it automatically
		req.Header.Set("Accept", "image/webp,image/apng,image/*,*/*;q=0.8")
		req.Header.Set("Accept-Language", "en-US,en;q=0.9")
		req.Header.Set("DNT", "1")
		req.Header.Set("Connection", "keep-alive")
		req.Header.Set("Upgrade-Insecure-Requests", "1")

		resp, err := client.Do(req)
		if err != nil {
			lastErr = err
			if httputil.IsNetworkError(err.Error()) && attempt < maxRetries-1 {
				backoff := httputil.CalculateBackoffSimple(attempt)
				log.Printf("[MediaCache] Network error on attempt %d/%d for %s, retrying in %v: %v", 
					attempt+1, maxRetries, urlStr, backoff, err)
				select {
				case <-totalCtx.Done():
					return nil, "", totalCtx.Err()
				case <-time.After(backoff):
				}
				continue
			}
			return nil, "", fmt.Errorf("failed to fetch media after %d attempts: %w", attempt+1, err)
		}
		lastResp = resp

		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			lastErr = fmt.Errorf("unexpected status code: %d", resp.StatusCode)
			
			// Retry on server errors
			if resp.StatusCode >= 500 && attempt < maxRetries-1 {
				backoff := httputil.CalculateBackoffSimple(attempt)
				log.Printf("[MediaCache] Server error %d on attempt %d/%d for %s, retrying in %v", 
					resp.StatusCode, attempt+1, maxRetries, urlStr, backoff)
				select {
				case <-totalCtx.Done():
					return nil, "", totalCtx.Err()
				case <-time.After(backoff):
				}
				continue
			}
			return nil, "", lastErr
		}

		data, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			lastErr = err
			if httputil.IsNetworkError(err.Error()) && attempt < maxRetries-1 {
				backoff := httputil.CalculateBackoffSimple(attempt)
				log.Printf("[MediaCache] Read error on attempt %d/%d for %s, retrying in %v: %v", 
					attempt+1, maxRetries, urlStr, backoff, err)
				select {
				case <-totalCtx.Done():
					return nil, "", totalCtx.Err()
				case <-time.After(backoff):
				}
				continue
			}
			return nil, "", fmt.Errorf("failed to read response body: %w", err)
		}

		contentType := resp.Header.Get("Content-Type")
		if contentType == "" {
			contentType = getContentTypeFromPath(urlStr)
		}

		return data, contentType, nil
	}

	return nil, "", fmt.Errorf("all %d attempts failed, last error: %w", maxRetries, lastErr)
}

// calculateTimeout determines the appropriate timeout based on Content-Length if available
func (mc *MediaCache) calculateTimeout(urlStr string, lastResp *http.Response) time.Duration {
	// Check if we have a Content-Length from a previous response
	if lastResp != nil {
		if contentLength := lastResp.Header.Get("Content-Length"); contentLength != "" {
			if size, err := parseContentLength(contentLength); err == nil {
				return httputil.CalculateDynamicMediaTimeout(size)
			}
		}
	}
	return defaultDownloadTimeout
}

// parseContentLength parses the Content-Length header value
func parseContentLength(s string) (int64, error) {
	var size int64
	_, err := fmt.Sscanf(s, "%d", &size)
	return size, err
}

// getSmartReferer determines the appropriate referer to use for a given image URL
// For third-party images (different domain than the referer), we use the image's own domain
// as the referer to avoid anti-hotlinking issues
func getSmartReferer(imageURL, originalReferer string) string {
	// Parse the image URL to get its hostname
	imgURL, err := url.Parse(imageURL)
	if err != nil {
		// If we can't parse the image URL, use the original referer
		return originalReferer
	}

	// Parse the original referer to get its hostname
	refURL, err := url.Parse(originalReferer)
	if err != nil {
		// If we can't parse the referer, use no referer
		return ""
	}

	imgHost := imgURL.Hostname()
	refHost := refURL.Hostname()

	// If the image host and referer host are the same domain, use the original referer
	// This handles same-origin images (e.g., images hosted on the same site as the article)
	if imgHost == refHost || strings.HasSuffix(imgHost, "."+refHost) || strings.HasSuffix(refHost, "."+imgHost) {
		return originalReferer
	}

	// For third-party images (different domain), use the image's own domain as referer
	// This avoids anti-hotlinking issues when the article's referer is blocked
	// For example: img.500px.me/image.jpg with referer from rsshub.pseudoyu.com
	// will use https://img.500px.me as the referer
	return fmt.Sprintf("%s://%s", imgURL.Scheme, imgURL.Host)
}

// CleanupOldFiles removes cached files older than the specified age
func (mc *MediaCache) CleanupOldFiles(maxAgeDays int) (int, error) {
	var cutoffTime time.Time
	count := 0

	if maxAgeDays <= 0 {
		// Special case: remove all files regardless of age
		cutoffTime = time.Now().Add(time.Hour) // Future time to match all files
	} else {
		cutoffTime = time.Now().AddDate(0, 0, -maxAgeDays)
	}

	entries, err := os.ReadDir(mc.cacheDir)
	if err != nil {
		return 0, fmt.Errorf("failed to read cache directory: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		filePath := filepath.Join(mc.cacheDir, entry.Name())
		info, err := entry.Info()
		if err != nil {
			continue
		}

		if info.ModTime().Before(cutoffTime) {
			if err := os.Remove(filePath); err == nil {
				count++
			}
		}
	}

	return count, nil
}

// GetCacheSize returns the total size of cached files in bytes
func (mc *MediaCache) GetCacheSize() (int64, error) {
	var totalSize int64

	entries, err := os.ReadDir(mc.cacheDir)
	if err != nil {
		return 0, fmt.Errorf("failed to read cache directory: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			continue
		}

		totalSize += info.Size()
	}

	return totalSize, nil
}

// CleanupBySize removes oldest files until cache is under the size limit
func (mc *MediaCache) CleanupBySize(maxSizeMB int) (int, error) {
	maxSize := int64(maxSizeMB) * 1024 * 1024
	currentSize, err := mc.GetCacheSize()
	if err != nil {
		return 0, err
	}

	if currentSize <= maxSize {
		return 0, nil
	}

	// Get all files with their modification times
	type fileInfo struct {
		path    string
		modTime time.Time
		size    int64
	}

	var files []fileInfo
	entries, err := os.ReadDir(mc.cacheDir)
	if err != nil {
		return 0, fmt.Errorf("failed to read cache directory: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			continue
		}

		files = append(files, fileInfo{
			path:    filepath.Join(mc.cacheDir, entry.Name()),
			modTime: info.ModTime(),
			size:    info.Size(),
		})
	}

	// Sort by modification time (oldest first) using built-in sort for better performance
	sort.Slice(files, func(i, j int) bool {
		return files[i].modTime.Before(files[j].modTime)
	})

	// Remove oldest files until under limit
	count := 0
	for _, f := range files {
		if currentSize <= maxSize {
			break
		}

		if err := os.Remove(f.path); err == nil {
			currentSize -= f.size
			count++
		}
	}

	return count, nil
}

// hashURL creates a SHA256 hash of the URL for use as filename
func hashURL(url string) string {
	h := sha256.New()
	h.Write([]byte(url))
	return hex.EncodeToString(h.Sum(nil))
}

// getExtensionFromURL extracts the file extension from URL
func getExtensionFromURL(url string) string {
	// Remove query parameters
	if idx := strings.Index(url, "?"); idx != -1 {
		url = url[:idx]
	}

	ext := filepath.Ext(url)
	if ext == "" {
		// Try to guess from URL patterns
		if strings.Contains(url, "image") || strings.Contains(url, "img") {
			return ".jpg"
		}
		if strings.Contains(url, "video") || strings.Contains(url, "vid") {
			return ".mp4"
		}
		return ".bin"
	}

	return ext
}

// getContentTypeFromPath determines content type from file extension
func getContentTypeFromPath(path string) string {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".png":
		return "image/png"
	case ".gif":
		return "image/gif"
	case ".webp":
		return "image/webp"
	case ".svg":
		return "image/svg+xml"
	case ".mp4":
		return "video/mp4"
	case ".webm":
		return "video/webm"
	case ".ogg":
		return "video/ogg"
	case ".mp3":
		return "audio/mpeg"
	case ".wav":
		return "audio/wav"
	case ".m4a":
		return "audio/mp4"
	default:
		return "application/octet-stream"
	}
}

// getExtensionFromContentType determines file extension from Content-Type header
func getExtensionFromContentType(contentType string) string {
	// Remove any parameters from content type
	if idx := strings.Index(contentType, ";"); idx != -1 {
		contentType = contentType[:idx]
	}
	contentType = strings.TrimSpace(strings.ToLower(contentType))

	switch contentType {
	case "image/jpeg":
		return ".jpg"
	case "image/png":
		return ".png"
	case "image/gif":
		return ".gif"
	case "image/webp":
		return ".webp"
	case "image/svg+xml":
		return ".svg"
	case "video/mp4":
		return ".mp4"
	case "video/webm":
		return ".webm"
	case "video/ogg":
		return ".ogg"
	case "audio/mpeg":
		return ".mp3"
	case "audio/wav":
		return ".wav"
	case "audio/mp4":
		return ".m4a"
	default:
		return ""
	}
}
