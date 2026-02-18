package article

import (
	"strings"

	md "github.com/JohannesKaufmann/html-to-markdown"
)

// sanitizeFilename creates a safe filename from a title
func sanitizeFilename(title string) string {
	invalidChars := []string{"<", ">", ":", "\"", "|", "?", "*", "\\", "/"}
	result := title

	for _, char := range invalidChars {
		result = strings.ReplaceAll(result, char, "_")
	}

	result = strings.TrimSpace(result)
	if len(result) > 100 {
		result = result[:100]
	}

	return result
}

// sanitizeTag creates a safe tag from feed name
func sanitizeTag(feedName string) string {
	tag := strings.ToLower(strings.ReplaceAll(feedName, " ", "_"))
	tag = strings.ReplaceAll(tag, "-", "_")
	tag = strings.ReplaceAll(tag, ".", "_")
	return tag
}

// htmlEncodeURL encodes URL characters
func htmlEncodeURL(url string) string {
	result := strings.ReplaceAll(url, "&", "&amp;")
	result = strings.ReplaceAll(result, "?", "&#63;")
	result = strings.ReplaceAll(result, "=", "&#61;")
	result = strings.ReplaceAll(result, "%", "&#37;")
	return result
}

// escapeYamlString escapes special characters for YAML
func escapeYamlString(s string) string {
	return strings.ReplaceAll(s, "\"", "\\\"")
}

// htmlToMarkdown converts HTML to Markdown
func htmlToMarkdown(html string) string {
	converter := md.NewConverter("", true, nil)
	markdown, err := converter.ConvertString(html)
	if err != nil {
		return cleanWhitespace(removeHTMLTags(html))
	}
	return cleanWhitespace(markdown)
}

// removeHTMLTags removes HTML tags
func removeHTMLTags(html string) string {
	var result strings.Builder
	inTag := false

	for _, char := range html {
		if char == '<' {
			inTag = true
		} else if char == '>' {
			inTag = false
		} else if !inTag {
			result.WriteRune(char)
		}
	}

	return result.String()
}

// cleanWhitespace removes excessive whitespace
func cleanWhitespace(text string) string {
	lines := strings.Split(text, "\n")
	var cleaned []string

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" || (len(cleaned) > 0 && cleaned[len(cleaned)-1] != "") {
			cleaned = append(cleaned, trimmed)
		}
	}

	result := strings.Join(cleaned, "\n")

	for strings.Contains(result, "\n\n\n") {
		result = strings.ReplaceAll(result, "\n\n\n", "\n\n")
	}

	return result
}
