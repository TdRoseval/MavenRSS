//go:build server

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

// HandleExportToObsidian exports an article to Obsidian in server mode.
// Instead of writing directly to file system, it provides a download of the Markdown file.
// @Summary      Export article to Obsidian (server mode)
// @Description  Export an article as a Markdown file for download (server mode)
// @Tags         articles
// @Accept       json
// @Produce      text/markdown
// @Param        request  body      ExportToObsidianRequest  true  "Article export request"
// @Success      200  {string}  string  "Markdown file download"
// @Failure      400  {object}  map[string]string  "Bad request"
// @Failure      404  {object}  map[string]string  "Article not found"
// @Router       /articles/export/obsidian [post]
func HandleExportToObsidian(h *core.Handler, w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		response.Error(w, nil, http.StatusMethodNotAllowed)
		return
	}

	var req ExportToObsidianRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, err, http.StatusBadRequest)
		return
	}

	if req.ArticleID <= 0 {
		response.Error(w, nil, http.StatusBadRequest)
		return
	}

	// Get article from database
	article, err := h.DB.GetArticleByID(int64(req.ArticleID))
	if err != nil {
		response.Error(w, err, http.StatusNotFound)
		return
	}

	// Check if Obsidian integration is enabled (optional in server mode, but still check)
	obsidianEnabled, _ := h.DB.GetSetting("obsidian_enabled")

	// Get article content
	content, _, err := h.GetArticleContent(int64(req.ArticleID))
	if err != nil {
		content = ""
	}

	// Generate Markdown content
	markdownContent := generateObsidianMarkdown(*article, content)

	// Generate filename (sanitize title)
	filename := sanitizeFilename(article.Title)
	if filename == "" {
		filename = fmt.Sprintf("Article_%d", article.ID)
	}
	filename += ".md"

	// In server mode, provide download instead of writing to vault
	w.Header().Set("Content-Type", "text/markdown; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(markdownContent))

	// Log that we're providing download instead of direct vault write
	if obsidianEnabled != "true" {
		logMsg := "Obsidian integration not enabled, providing Markdown download instead"
		fmt.Println(logMsg)
	} else {
		logMsg := "Server mode detected, providing Markdown download instead of direct vault write"
		fmt.Println(logMsg)
	}
}

// generateObsidianMarkdown converts an article to Markdown format for Obsidian
// (duplicate of the function in article_export.go to avoid build tag issues)
func generateObsidianMarkdown(article models.Article, content string) string {
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
	sb.WriteString(fmt.Sprintf("**Added to Obsidian:** %s\n", time.Now().Format("2006-01-02 15:04:05")))
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
