package textutil

import (
	"regexp"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

var (
	htmlTagRegex              = regexp.MustCompile(`(?i)</?[a-z][^>]*>`)
	markdownTOCBlockRegex     = regexp.MustCompile(`(?is)\b(?:table of contents|contents|toc|目录)\b\s*(?:\[\s*toggle\s*\]\([^)]+\)\s*)?(?:(?:[-*+]\s*)?\[[^\]]+\]\([^)]+\)\s*)+`)
	markdownToggleLinkRegex   = regexp.MustCompile(`(?is)\[\s*toggle\s*\]\([^)]+\)`)
	emptyMarkdownHeadingRegex = regexp.MustCompile(`(?m)^\s*#{1,6}\s*$`)
	multipleBlankLinesRegex   = regexp.MustCompile(`\n{3,}`)
	tocIdentifierTokenRegex   = regexp.MustCompile(`(?i)(?:^|[\s_-])(toc|table[\s_-]*of[\s_-]*contents?|contents?)(?:$|[\s_-])`)
)

// NormalizeArticleContent standardizes cached article content for rendering.
// HTML content is cleaned and common TOC boilerplate is removed.
// Non-HTML content is treated as markdown/plain text and rendered to HTML.
func NormalizeArticleContent(content string) string {
	content = strings.TrimSpace(content)
	if content == "" {
		return ""
	}

	if looksLikeHTML(content) {
		return strings.TrimSpace(stripHTMLBoilerplate(CleanHTML(content)))
	}

	content = stripMarkdownBoilerplate(content)
	if content == "" {
		return ""
	}

	return strings.TrimSpace(RenderMarkdown(content))
}

func looksLikeHTML(content string) bool {
	return htmlTagRegex.MatchString(content)
}

func stripMarkdownBoilerplate(content string) string {
	content = markdownTOCBlockRegex.ReplaceAllString(content, "")
	content = markdownToggleLinkRegex.ReplaceAllString(content, "")
	content = emptyMarkdownHeadingRegex.ReplaceAllString(content, "")
	content = multipleBlankLinesRegex.ReplaceAllString(content, "\n\n")
	return strings.TrimSpace(content)
}

func stripHTMLBoilerplate(content string) string {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(content))
	if err != nil {
		return content
	}

	doc.Find("*").Each(func(_ int, selection *goquery.Selection) {
		if shouldRemoveTOCNode(selection) {
			selection.Remove()
		}
	})

	if bodyHTML, err := doc.Find("body").Html(); err == nil && strings.TrimSpace(bodyHTML) != "" {
		return bodyHTML
	}
	if docHTML, err := doc.Html(); err == nil {
		return docHTML
	}
	return content
}

func shouldRemoveTOCNode(selection *goquery.Selection) bool {
	tagName := strings.ToLower(goquery.NodeName(selection))
	if tagName == "html" || tagName == "head" || tagName == "body" {
		return false
	}

	idAttr := selection.AttrOr("id", "")
	classAttr := selection.AttrOr("class", "")
	if hasTOCIdentifier(idAttr) || hasTOCIdentifier(classAttr) {
		return true
	}

	text := normalizeBoilerplateText(selection.Text())
	if text == "" {
		return false
	}

	if text == "table of contents" || text == "contents" || text == "目录" {
		return true
	}

	if strings.Contains(text, "table of contents") || strings.Contains(text, "目录") {
		switch tagName {
		case "nav", "aside", "section", "div", "details", "ul", "ol":
			if selection.Find("a").Length() >= 3 {
				return true
			}
		}
	}

	if tagName == "summary" && strings.Contains(text, "toggle") {
		return true
	}

	return false
}

func hasTOCIdentifier(value string) bool {
	value = strings.TrimSpace(strings.ToLower(value))
	if value == "" {
		return false
	}
	return tocIdentifierTokenRegex.MatchString(value)
}

func normalizeBoilerplateText(value string) string {
	value = strings.TrimSpace(strings.ToLower(value))
	if value == "" {
		return ""
	}
	return strings.Join(strings.Fields(value), " ")
}
