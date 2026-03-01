package cache

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestMediaCache_BasicOperations(t *testing.T) {
	dir := t.TempDir()

	mc, err := NewMediaCache(dir)
	if err != nil {
		t.Fatalf("NewMediaCache failed: %v", err)
	}

	url := "https://example.com/image.jpg?query=1"
	path := mc.GetCachedPath(url)
	if filepath.Dir(path) != dir {
		t.Fatalf("cached path in wrong dir: %s", path)
	}

	// Create a cached file to simulate existing cache
	if err := os.WriteFile(path, []byte("data"), 0644); err != nil {
		t.Fatalf("write cached file: %v", err)
	}

	if !mc.Exists(url) {
		t.Fatalf("expected Exists to be true for cached file")
	}

	data, ctype, err := mc.Get(url, "")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if string(data) != "data" {
		t.Fatalf("unexpected data")
	}
	if ctype != "image/jpeg" {
		t.Fatalf("unexpected content type: %s", ctype)
	}

	// Test CleanupOldFiles (create an old file)
	oldPath := filepath.Join(dir, "old.bin")
	if err := os.WriteFile(oldPath, []byte("x"), 0644); err != nil {
		t.Fatalf("write old file: %v", err)
	}
	// backdate modification time
	oldTime := time.Now().AddDate(0, 0, -10)
	_ = os.Chtimes(oldPath, oldTime, oldTime)

	removed, err := mc.CleanupOldFiles(1)
	if err != nil {
		t.Fatalf("CleanupOldFiles failed: %v", err)
	}
	if removed == 0 {
		t.Fatalf("expected CleanupOldFiles to remove old file")
	}
}

func TestGetExtensionAndContentTypeHelpers(t *testing.T) {
	if ext := getExtensionFromURL("https://x/y.png?v=1"); ext != ".png" {
		t.Fatalf("expected .png got %s", ext)
	}
	if ct := getContentTypeFromPath("file.jpg"); ct != "image/jpeg" {
		t.Fatalf("unexpected content type: %s", ct)
	}
	if ext := getExtensionFromContentType("image/png; charset=utf8"); ext != ".png" {
		t.Fatalf("unexpected ext: %s", ext)
	}
}

func TestGetSmartReferer_ITHome(t *testing.T) {
	testCases := []struct {
		name     string
		imageURL string
		referer  string
		expected string
	}{
		{
			name:     "img.ithome.com with www.ithome.com referer",
			imageURL: "https://img.ithome.com/newsuploadfiles/2026/2/image.jpg",
			referer:  "https://www.ithome.com/0/839/839773.htm",
			expected: "https://www.ithome.com",
		},
		{
			name:     "img.ithome.com with ithome.com referer",
			imageURL: "https://img.ithome.com/newsuploadfiles/2026/2/image.jpg",
			referer:  "https://ithome.com/0/839/839773.htm",
			expected: "https://www.ithome.com",
		},
		{
			name:     "img.500px.me with different domain referer",
			imageURL: "https://img.500px.me/image.jpg",
			referer:  "https://rsshub.pseudoyu.com/feed",
			expected: "https://img.500px.me",
		},
		{
			name:     "same domain image",
			imageURL: "https://www.ithome.com/images/logo.png",
			referer:  "https://www.ithome.com/0/839/839773.htm",
			expected: "https://www.ithome.com/0/839/839773.htm",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := getSmartReferer(tc.imageURL, tc.referer)
			if result != tc.expected {
				t.Errorf("getSmartReferer(%q, %q) = %q, want %q", tc.imageURL, tc.referer, result, tc.expected)
			}
		})
	}
}
