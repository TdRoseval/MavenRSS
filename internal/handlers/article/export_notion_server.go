//go:build server

// Package article provides HTTP handlers for article-related operations (server mode).
package article

import (
	"encoding/json"
	"fmt"
	"html"
	"net/http"
	"strings"
	"time"

	"MrRSS/internal/handlers/core"
	"MrRSS/internal/handlers/response"
	"MrRSS/internal/models"

	md "github.com/JohannesKaufmann/html-to-markdown"
)

// HandleExportToNotion exports an article to Notion in server mode.
// Instead of using Notion API, it provides a download of the Markdown file.
// @Summary      Export article to Notion (server mode)
// @Description  Export an article as a Markdown file for download (server mode)
// @Tags         articles
// @Accept       json
// @Produce      text/markdown
// @Param        request  body      ExportToNotionRequest  true  "Article export request"
// @Success      200  {file}  file  "Markdown file download"
// @Failure      400  {object}  map[string]string  "Bad request"
// @Failure      404  {object}  map[string]string  "Article not found"
// @Router       /articles/export/notion [post]
func HandleExportToNotion(h *core.Handler, w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		response.Error(w, nil, http.StatusMethodNotAllowed)
		return
	}

	var req ExportToNotionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, err, http.StatusBadRequest)
		return
	}

	if req.ArticleID <= 0 {
		response.Error(w, fmt.Errorf("invalid article ID"), http.StatusBadRequest)
		return
	}

	// Get article from database
	article, err := h.DB.GetArticleByID(int64(req.ArticleID))
	if err != nil {
		response.Error(w, err, http.StatusNotFound)
		return
	}

	// Get article content
	content, _, err := h.GetArticleContent(int64(req.ArticleID))
	if err != nil {
		content = ""
	}

	// Generate Markdown content
	markdownContent := generateNotionMarkdown(*article, content)

	// Generate filename (sanitize title)
	filename := sanitizeFilename(article.Title)
	if filename == "" {
		filename = fmt.Sprintf("Article_%d", article.ID)
	}
	filename += ".md"

	// In server mode, provide download instead of Notion API integration
	w.Header().Set("Content-Type", "text/markdown; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(markdownContent))

	logMsg := "Server mode detected, providing Markdown download for Notion export"
	fmt.Println(logMsg)
}

// generateNotionMarkdown converts an article to Markdown format for Notion
// (duplicate of the function in article_export_server.go to avoid build tag issues)
func generateNotionMarkdown(article models.Article, content string) string {
	var sb strings.Builder

	sb.WriteString("---\n")
	sb.WriteString(fmt.Sprintf("title: \"%s\"\n", escapeYamlString(article.Title)))
	sb.WriteString(fmt.Sprintf("feed: \"%s\"\n", escapeYamlString(article.FeedTitle)))
	sb.WriteString(fmt.Sprintf("published: \"%s\"\n", article.PublishedAt.Format(time.RFC3339)))
	sb.WriteString(fmt.Sprintf("tags: [rss, %s]\n", sanitizeTag(article.FeedTitle)))
	sb.WriteString("---\n\n")

	sb.WriteString(fmt.Sprintf("# %s\n\n", article.Title))
	sb.WriteString(fmt.Sprintf("**Source:** %s\n\n", htmlEncodeURL(article.URL)))

	if content != "" {
		decodedContent := html.UnescapeString(content)
		markdownContent := htmlToMarkdown(decodedContent)
		sb.WriteString(markdownContent)
		sb.WriteString("\n\n")
	}

	sb.WriteString("---\n")
	sb.WriteString(fmt.Sprintf("**Exported:** %s\n", time.Now().Format("2006-01-02 15:04:05")))
	sb.WriteString(fmt.Sprintf("**Article ID:** %d\n", article.ID))

	return sb.String()
}

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
