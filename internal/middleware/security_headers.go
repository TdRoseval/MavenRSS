package middleware

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

// baseFrameSources defines the base frame sources that are always allowed
var baseFrameSources = []string{
	"'self'",
	"https://www.youtube.com",
	"https://youtube.com",
	"https://www.youtube-nocookie.com",
	"https://player.vimeo.com",
	"https://vimeo.com",
	"https://www.bilibili.com",
	"https://player.bilibili.com",
}

// SecurityHeaders adds common security headers to responses.
func SecurityHeaders() Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("X-Content-Type-Options", "nosniff")
			w.Header().Set("X-Frame-Options", "DENY")
			w.Header().Set("X-XSS-Protection", "1; mode=block")
			w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
			w.Header().Set("Content-Security-Policy", buildCSP(r))
			w.Header().Set("Permissions-Policy", "geolocation=(), microphone=(), camera=()")
			
			next.ServeHTTP(w, r)
		})
	}
}

// buildCSP builds the Content-Security-Policy header value
// It dynamically adds frame sources based on the current request context
func buildCSP(r *http.Request) string {
	// Start with base policies
	cspParts := []string{
		"default-src 'self'",
		"script-src 'self' 'unsafe-inline' 'unsafe-eval'",
		"style-src 'self' 'unsafe-inline' https://fonts.googleapis.com",
		"img-src 'self' data: https:",
		"font-src 'self' data: https://fonts.gstatic.com",
		"connect-src 'self' https:",
		"media-src 'self' https: blob:",
	}

	// Build frame-src dynamically
	frameSources := make([]string, len(baseFrameSources))
	copy(frameSources, baseFrameSources)

	// Add dynamic frame sources based on request context
	dynamicFrameSources := getDynamicFrameSources(r)
	if len(dynamicFrameSources) > 0 {
		frameSources = append(frameSources, dynamicFrameSources...)
	}

	frameSrcValue := strings.Join(frameSources, " ")
	cspParts = append(cspParts, "frame-src "+frameSrcValue)
	cspParts = append(cspParts, "object-src 'none'")

	return strings.Join(cspParts, "; ") + ";"
}

// getDynamicFrameSources returns additional frame sources based on request context
// For example, if viewing an article from ithome.com, allow img.ithome.com
func getDynamicFrameSources(r *http.Request) []string {
	sources := []string{}

	// Check if this is a media proxy request with a referer
	referer := r.Referer()
	if referer != "" {
		if parsed, err := url.Parse(referer); err == nil {
			host := parsed.Hostname()
			if host != "" {
				// Allow the main domain and its subdomains
				// e.g., www.ithome.com -> allow *.ithome.com and img.ithome.com
				parts := strings.Split(host, ".")
				if len(parts) >= 2 {
					// Get the main domain (e.g., ithome.com)
					mainDomain := strings.Join(parts[len(parts)-2:], ".")
					sources = append(sources, fmt.Sprintf("https://*.%s", mainDomain))
					sources = append(sources, fmt.Sprintf("https://img.%s", mainDomain))
				}
			}
		}
	}

	// Remove duplicates
	seen := make(map[string]bool)
	unique := []string{}
	for _, src := range sources {
		if !seen[src] {
			seen[src] = true
			unique = append(unique, src)
		}
	}

	return unique
}
