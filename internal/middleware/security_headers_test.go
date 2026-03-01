package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestBuildCSP_BasicStructure(t *testing.T) {
	req := httptest.NewRequest("GET", "/api/media/proxy", nil)
	csp := buildCSP(req)

	// Check that CSP contains all required directives
	requiredDirectives := []string{
		"default-src 'self'",
		"script-src",
		"style-src",
		"img-src",
		"font-src",
		"connect-src",
		"media-src",
		"frame-src",
		"object-src 'none'",
	}

	for _, directive := range requiredDirectives {
		if !strings.Contains(csp, directive) {
			t.Errorf("CSP missing directive: %s, got: %s", directive, csp)
		}
	}
}

func TestBuildCSP_BaseFrameSources(t *testing.T) {
	req := httptest.NewRequest("GET", "/api/media/proxy", nil)
	csp := buildCSP(req)

	// Check that base frame sources are always present
	baseSources := []string{
		"'self'",
		"https://www.youtube.com",
		"https://youtube.com",
		"https://www.youtube-nocookie.com",
		"https://player.vimeo.com",
		"https://vimeo.com",
		"https://www.bilibili.com",
		"https://player.bilibili.com",
	}

	for _, source := range baseSources {
		if !strings.Contains(csp, source) {
			t.Errorf("CSP missing base frame source: %s, got: %s", source, csp)
		}
	}
}

func TestBuildCSP_DynamicFrameSources_ITHome(t *testing.T) {
	req := httptest.NewRequest("GET", "/api/media/proxy", nil)
	req.Header.Set("Referer", "https://www.ithome.com/0/839/839773.htm")
	csp := buildCSP(req)

	// Check that dynamic frame sources for ithome.com are added
	expectedSources := []string{
		"https://*.ithome.com",
		"https://img.ithome.com",
	}

	for _, source := range expectedSources {
		if !strings.Contains(csp, source) {
			t.Errorf("CSP missing dynamic frame source for ithome: %s, got: %s", source, csp)
		}
	}
}

func TestBuildCSP_DynamicFrameSources_OtherDomain(t *testing.T) {
	req := httptest.NewRequest("GET", "/api/media/proxy", nil)
	req.Header.Set("Referer", "https://www.example.com/article/123")
	csp := buildCSP(req)

	// Check that dynamic frame sources for example.com are added
	expectedSources := []string{
		"https://*.example.com",
		"https://img.example.com",
	}

	for _, source := range expectedSources {
		if !strings.Contains(csp, source) {
			t.Errorf("CSP missing dynamic frame source for example: %s, got: %s", source, csp)
		}
	}
}

func TestBuildCSP_NoReferer(t *testing.T) {
	req := httptest.NewRequest("GET", "/api/media/proxy", nil)
	csp := buildCSP(req)

	// Should not contain dynamic sources when no referer
	if strings.Contains(csp, "*.com") {
		t.Errorf("CSP should not contain dynamic sources without referer, got: %s", csp)
	}
}

func TestBuildCSP_InvalidReferer(t *testing.T) {
	req := httptest.NewRequest("GET", "/api/media/proxy", nil)
	req.Header.Set("Referer", "not-a-valid-url")
	csp := buildCSP(req)

	// Should not crash or add invalid sources
	if strings.Contains(csp, "not-a-valid-url") {
		t.Errorf("CSP should not contain invalid referer, got: %s", csp)
	}
}

func TestSecurityHeaders_Middleware(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	wrapped := SecurityHeaders()(handler)
	req := httptest.NewRequest("GET", "/test", nil)
	rr := httptest.NewRecorder()

	wrapped.ServeHTTP(rr, req)

	// Check that security headers are set
	expectedHeaders := map[string]string{
		"X-Content-Type-Options": "nosniff",
		"X-Frame-Options":        "DENY",
		"X-XSS-Protection":       "1; mode=block",
		"Content-Security-Policy": "", // Just check it's set
	}

	for header, expectedValue := range expectedHeaders {
		value := rr.Header().Get(header)
		if value == "" {
			t.Errorf("Missing security header: %s", header)
		}
		if expectedValue != "" && value != expectedValue {
			t.Errorf("Header %s has wrong value: got %s, want %s", header, value, expectedValue)
		}
	}
}

func TestGetDynamicFrameSources_DomainExtraction(t *testing.T) {
	testCases := []struct {
		name     string
		referer  string
		expected []string
	}{
		{
			name:     "www.ithome.com",
			referer:  "https://www.ithome.com/article",
			expected: []string{"https://*.ithome.com", "https://img.ithome.com"},
		},
		{
			name:     "ithome.com (no subdomain)",
			referer:  "https://ithome.com/article",
			expected: []string{"https://*.ithome.com", "https://img.ithome.com"},
		},
		{
			name:     "subdomain.example.org",
			referer:  "https://blog.example.org/post",
			expected: []string{"https://*.example.org", "https://img.example.org"},
		},
		{
			name:     "deep.subdomain.example.com",
			referer:  "https://deep.subdomain.example.com/page",
			expected: []string{"https://*.example.com", "https://img.example.com"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/api/media/proxy", nil)
			req.Header.Set("Referer", tc.referer)
			sources := getDynamicFrameSources(req)

			for _, exp := range tc.expected {
				found := false
				for _, src := range sources {
					if src == exp {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Expected source %s not found in %v", exp, sources)
				}
			}
		})
	}
}
