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

	needsProcessing := false
	hasMalformedTags := false
	hasStyleAttrs := false
	hasClassAttrs := false
	hasStyleTags := false
	hasScriptTags := false

	// Single pass to detect all needed operations
	for i := 0; i < len(htmlContent); i++ {
		// Check for malformed tags pattern "-->"
		if i+2 < len(htmlContent) && htmlContent[i] == '-' && htmlContent[i+1] == '-' && htmlContent[i+2] == '>' {
			hasMalformedTags = true
			needsProcessing = true
		}
		// Check for " style="
		if i+6 < len(htmlContent) && htmlContent[i] == ' ' && 
		   htmlContent[i+1] == 's' && htmlContent[i+2] == 't' && 
		   htmlContent[i+3] == 'y' && htmlContent[i+4] == 'l' && 
		   htmlContent[i+5] == 'e' && htmlContent[i+6] == '=' {
			hasStyleAttrs = true
			needsProcessing = true
		}
		// Check for " class="
		if i+6 < len(htmlContent) && htmlContent[i] == ' ' && 
		   htmlContent[i+1] == 'c' && htmlContent[i+2] == 'l' && 
		   htmlContent[i+3] == 'a' && htmlContent[i+4] == 's' && 
		   htmlContent[i+5] == 's' && htmlContent[i+6] == '=' {
			hasClassAttrs = true
			needsProcessing = true
		}
		// Check for "<style"
		if i+5 < len(htmlContent) && htmlContent[i] == '<' && 
		   htmlContent[i+1] == 's' && htmlContent[i+2] == 't' && 
		   htmlContent[i+3] == 'y' && htmlContent[i+4] == 'l' && 
		   htmlContent[i+5] == 'e' {
			hasStyleTags = true
			needsProcessing = true
		}
		// Check for "<script"
		if i+6 < len(htmlContent) && htmlContent[i] == '<' && 
		   htmlContent[i+1] == 's' && htmlContent[i+2] == 'c' && 
		   htmlContent[i+3] == 'r' && htmlContent[i+4] == 'i' && 
		   htmlContent[i+5] == 'p' && htmlContent[i+6] == 't' {
			hasScriptTags = true
			needsProcessing = true
		}
		
		// Early exit if we found everything already
		if hasMalformedTags && hasStyleAttrs && hasClassAttrs && hasStyleTags && hasScriptTags {
			break
		}
	}

	if !needsProcessing {
		return strings.TrimSpace(htmlContent)
	}

	result := htmlContent

	// Fix malformed opening tags like <p--> to <p>
	if hasMalformedTags {
		result = malformedTagRegex.ReplaceAllString(result, "<$1>")
		result = malformedSelfClosingWithAttrs.ReplaceAllString(result, "<$1 $2>")
		result = malformedSelfClosingNoAttrs.ReplaceAllString(result, "<$1>")
	}

	// Remove inline style attributes
	if hasStyleAttrs {
		result = styleAttrRegex.ReplaceAllString(result, "")
		result = styleAttrSingleQuoteRegex.ReplaceAllString(result, "")
	}

	// Remove class attributes
	if hasClassAttrs {
		result = classAttrRegex.ReplaceAllString(result, "")
		result = classAttrSingleQuoteRegex.ReplaceAllString(result, "")
	}

	// Remove <style> tags and their content
	if hasStyleTags {
		result = styleTagRegex.ReplaceAllString(result, "")
	}

	// Remove <script> tags and their content
	if hasScriptTags {
		result = scriptTagRegex.ReplaceAllString(result, "")
	}

	return strings.TrimSpace(result)
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
