// Package textutil provides text processing utilities including HTML cleaning,
// markdown rendering, and text sanitization.
package textutil

import (
	"regexp"
	"strings"

	"github.com/gomarkdown/markdown"
	"github.com/gomarkdown/markdown/html"
	"github.com/gomarkdown/markdown/parser"
)

// selfClosingTags is the list of HTML self-closing tags to handle
const selfClosingTags = "img|br|hr|input|meta|link"

// Compile regex patterns once at package initialization for better performance
var (
	// Matches malformed opening tags like <p-->, <div-->
	malformedTagRegex = regexp.MustCompile(`<([a-zA-Z][a-zA-Z0-9]*)\s*--+>`)

	// Matches malformed self-closing tags with attributes like <img src="..." -->
	malformedSelfClosingWithAttrs = regexp.MustCompile(`<(` + selfClosingTags + `)\s+([^<>]+?)--+>`)

	// Matches malformed self-closing tags without attributes like <br-->
	malformedSelfClosingNoAttrs = regexp.MustCompile(`<(` + selfClosingTags + `)\s*--+>`)

	// Matches style attributes in HTML tags
	styleAttrRegex = regexp.MustCompile(`\s+style\s*=\s*"[^"]*"`)

	// Alternative style attribute with single quotes
	styleAttrSingleQuoteRegex = regexp.MustCompile(`\s+style\s*=\s*'[^']*'`)

	// Matches class attributes in HTML tags
	classAttrRegex = regexp.MustCompile(`\s+class\s*=\s*"[^"]*"`)

	// Alternative class attribute with single quotes
	classAttrSingleQuoteRegex = regexp.MustCompile(`\s+class\s*=\s*'[^']*'`)

	// Matches <style> tags and their content
	styleTagRegex = regexp.MustCompile(`(?i)<style[^>]*>.*?</style>`)

	// Matches <script> tags and their content
	scriptTagRegex = regexp.MustCompile(`(?i)<script[^>]*>.*?</script>`)
)

// CleanHTML sanitizes HTML content by fixing common malformed patterns
// and removing unwanted inline styles, classes, and scripts.
// Optimized version to reduce CPU load on 2-core systems.
func CleanHTML(htmlContent string) string {
	if htmlContent == "" {
		return htmlContent
	}

	// Quick check: if no tags to fix or remove, return early
	if !strings.Contains(htmlContent, "-->") && 
	   !strings.Contains(htmlContent, " style=") && 
	   !strings.Contains(htmlContent, " class=") &&
	   !strings.Contains(htmlContent, "<style") &&
	   !strings.Contains(htmlContent, "<script") {
		return strings.TrimSpace(htmlContent)
	}

	// Fix malformed opening tags like <p--> to <p>
	if strings.Contains(htmlContent, "-->") {
		htmlContent = malformedTagRegex.ReplaceAllString(htmlContent, "<$1>")
		htmlContent = malformedSelfClosingWithAttrs.ReplaceAllString(htmlContent, "<$1 $2>")
		htmlContent = malformedSelfClosingNoAttrs.ReplaceAllString(htmlContent, "<$1>")
	}

	// Remove inline style attributes
	if strings.Contains(htmlContent, " style=") {
		htmlContent = styleAttrRegex.ReplaceAllString(htmlContent, "")
		htmlContent = styleAttrSingleQuoteRegex.ReplaceAllString(htmlContent, "")
	}

	// Remove class attributes
	if strings.Contains(htmlContent, " class=") {
		htmlContent = classAttrRegex.ReplaceAllString(htmlContent, "")
		htmlContent = classAttrSingleQuoteRegex.ReplaceAllString(htmlContent, "")
	}

	// Remove <style> tags and their content
	if strings.Contains(htmlContent, "<style") {
		htmlContent = styleTagRegex.ReplaceAllString(htmlContent, "")
	}

	// Remove <script> tags and their content
	if strings.Contains(htmlContent, "<script") {
		htmlContent = scriptTagRegex.ReplaceAllString(htmlContent, "")
	}

	return strings.TrimSpace(htmlContent)
}

// RenderMarkdown converts markdown text to safe HTML.
func RenderMarkdown(markdownText string) string {
	if markdownText == "" {
		return ""
	}

	extensions := parser.CommonExtensions | parser.AutoHeadingIDs
	p := parser.NewWithExtensions(extensions)

	htmlFlags := html.CommonFlags | html.HrefTargetBlank
	opts := html.RendererOptions{Flags: htmlFlags}
	renderer := html.NewRenderer(opts)

	htmlBytes := markdown.ToHTML([]byte(markdownText), p, renderer)
	return string(htmlBytes)
}

// RenderMarkdownInline converts markdown to HTML without wrapping <p> tags.
func RenderMarkdownInline(markdownText string) string {
	if markdownText == "" {
		return ""
	}

	extensions := parser.CommonExtensions | parser.AutoHeadingIDs
	p := parser.NewWithExtensions(extensions)

	htmlFlags := html.CommonFlags | html.HrefTargetBlank
	opts := html.RendererOptions{Flags: htmlFlags}
	renderer := html.NewRenderer(opts)

	htmlBytes := markdown.ToHTML([]byte(markdownText), p, renderer)

	result := string(htmlBytes)
	result = strings.TrimPrefix(result, "<p>")
	result = strings.TrimSuffix(result, "</p>")
	result = strings.TrimSuffix(result, "<p />")

	return result
}

// SanitizeHTML removes potentially dangerous HTML tags and attributes.
func SanitizeHTML(htmlContent string) string {
	if htmlContent == "" {
		return ""
	}

	// Remove script tags
	scriptRegex := regexp.MustCompile(`(?i)<script[^>]*>.*?</script>`)
	htmlContent = scriptRegex.ReplaceAllString(htmlContent, "")

	// Remove style tags
	styleRegex := regexp.MustCompile(`(?i)<style[^>]*>.*?</style>`)
	htmlContent = styleRegex.ReplaceAllString(htmlContent, "")

	// Remove iframe tags
	iframeRegex := regexp.MustCompile(`(?i)<iframe[^>]*>.*?</iframe>`)
	htmlContent = iframeRegex.ReplaceAllString(htmlContent, "")

	// Remove on* event handlers
	eventRegex := regexp.MustCompile(`(?i)\s+on\w+\s*=\s*["'][^"']*["']`)
	htmlContent = eventRegex.ReplaceAllString(htmlContent, "")

	// Remove javascript: protocol
	jsRegex := regexp.MustCompile(`(?i)javascript:`)
	htmlContent = jsRegex.ReplaceAllString(htmlContent, "")

	return htmlContent
}

// ConvertMarkdownToHTML converts markdown to safe HTML with sanitization.
func ConvertMarkdownToHTML(markdownText string) string {
	if markdownText == "" {
		return ""
	}

	htmlContent := RenderMarkdown(markdownText)
	return SanitizeHTML(htmlContent)
}
