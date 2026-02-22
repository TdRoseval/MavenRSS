package feed

import (
	"MavenRSS/internal/models"
	"strings"
	"testing"
	"time"

	"github.com/antchfx/htmlquery"
	"github.com/antchfx/xmlquery"
)

func TestParseFeedWithXPath_HTML(t *testing.T) {
	// Create a mock fetcher
	fetcher := &Fetcher{}

	// Create a test feed with HTML+XPath configuration
	feed := &models.Feed{
		Title:               "Test HTML Feed",
		URL:                 "http://example.com",
		Description:         "Test feed",
		Type:                "HTML+XPath",
		XPathItem:           "//div[@class='article']",
		XPathItemTitle:      ".//h2[@class='title']",
		XPathItemContent:    ".//div[@class='content']",
		XPathItemUri:        ".//a[@class='link']/@href",
		XPathItemAuthor:     ".//span[@class='author']",
		XPathItemTimestamp:  ".//time[@class='date']",
		XPathItemTimeFormat: "2006-01-02",
		XPathItemThumbnail:  ".//img[@class='thumb']/@src",
		XPathItemCategories: ".//span[@class='category']",
		XPathItemUid:        ".//div[@class='id']",
	}

	// Test the extractItemFromHTMLNode function with a mock HTML node
	// This is a simplified test - in real usage, the HTML would be parsed from the web
	htmlContent := `
		<div class="article">
			<h2 class="title">Test Article</h2>
			<div class="content">This is test content</div>
			<a class="link" href="http://example.com/article1">Link</a>
			<span class="author">Test Author</span>
			<time class="date">2023-12-01</time>
			<img class="thumb" src="http://example.com/image.jpg" />
			<span class="category">Tech</span>
			<div class="id">12345</div>
		</div>
	`

	// Parse the HTML
	doc, err := htmlquery.Parse(strings.NewReader(htmlContent))
	if err != nil {
		t.Fatalf("Failed to parse HTML: %v", err)
	}

	// Find the article node
	articleNode := htmlquery.FindOne(doc, "//div[@class='article']")
	if articleNode == nil {
		t.Fatal("Article node not found")
	}

	// Extract item
	item := fetcher.extractItemFromHTMLNode(articleNode, feed)

	// Verify extraction
	if item.Title != "Test Article" {
		t.Errorf("Expected title 'Test Article', got '%s'", item.Title)
	}

	if item.Content != "<div class=\"content\">This is test content</div>" {
		t.Errorf("Expected content '<div class=\"content\">This is test content</div>', got '%s'", item.Content)
	}

	if item.Link != "http://example.com/article1" {
		t.Errorf("Expected link 'http://example.com/article1', got '%s'", item.Link)
	}

	if item.Author == nil || item.Author.Name != "Test Author" {
		t.Errorf("Expected author 'Test Author', got '%v'", item.Author)
	}

	if item.PublishedParsed == nil {
		t.Error("Expected published date to be parsed")
	} else {
		expectedTime, _ := time.Parse("2006-01-02", "2023-12-01")
		if !item.PublishedParsed.Equal(expectedTime) {
			t.Errorf("Expected date %v, got %v", expectedTime, item.PublishedParsed)
		}
	}

	if item.Image == nil || item.Image.URL != "http://example.com/image.jpg" {
		t.Errorf("Expected image URL 'http://example.com/image.jpg', got '%v'", item.Image)
	}

	if len(item.Categories) != 1 || item.Categories[0] != "Tech" {
		t.Errorf("Expected categories ['Tech'], got %v", item.Categories)
	}

	if item.GUID != "12345" {
		t.Errorf("Expected GUID '12345', got '%s'", item.GUID)
	}
}

func TestParseFeedWithXPath_XML(t *testing.T) {
	// Create a mock fetcher
	fetcher := &Fetcher{}

	// Create a test feed with XML+XPath configuration
	feed := &models.Feed{
		Title:               "Test XML Feed",
		URL:                 "http://example.com",
		Description:         "Test feed",
		Type:                "XML+XPath",
		XPathItem:           "//item",
		XPathItemTitle:      "title",
		XPathItemContent:    "content",
		XPathItemUri:        "link",
		XPathItemAuthor:     "author",
		XPathItemTimestamp:  "pubDate",
		XPathItemTimeFormat: time.RFC1123,
		XPathItemThumbnail:  "thumbnail/@url",
		XPathItemCategories: "category",
		XPathItemUid:        "guid",
	}

	// Test the extractItemFromXMLNode function with a mock XML node
	xmlContent := `
		<item>
			<title>Test Article</title>
			<content>This is test content</content>
			<link>http://example.com/article1</link>
			<author>Test Author</author>
			<pubDate>Mon, 01 Dec 2023 12:00:00 GMT</pubDate>
			<thumbnail url="http://example.com/image.jpg" />
			<category>Tech</category>
			<guid>12345</guid>
		</item>
	`

	// Parse the XML
	doc, err := xmlquery.Parse(strings.NewReader(xmlContent))
	if err != nil {
		t.Fatalf("Failed to parse XML: %v", err)
	}

	// Find the item node
	itemNode := xmlquery.FindOne(doc, "//item")
	if itemNode == nil {
		t.Fatal("Item node not found")
	}

	// Extract item
	item := fetcher.extractItemFromXMLNode(itemNode, feed)

	// Verify extraction
	if item.Title != "Test Article" {
		t.Errorf("Expected title 'Test Article', got '%s'", item.Title)
	}

	if item.Content != "<content>This is test content</content>" {
		t.Errorf("Expected content '<content>This is test content</content>', got '%s'", item.Content)
	}

	if item.Link != "http://example.com/article1" {
		t.Errorf("Expected link 'http://example.com/article1', got '%s'", item.Link)
	}

	if item.Author == nil || item.Author.Name != "Test Author" {
		t.Errorf("Expected author 'Test Author', got '%v'", item.Author)
	}

	if item.PublishedParsed == nil {
		t.Error("Expected published date to be parsed")
	} else {
		expectedTime, _ := time.Parse(time.RFC1123, "Mon, 01 Dec 2023 12:00:00 GMT")
		if !item.PublishedParsed.Equal(expectedTime) {
			t.Errorf("Expected date %v, got %v", expectedTime, item.PublishedParsed)
		}
	}

	if item.Image == nil || item.Image.URL != "http://example.com/image.jpg" {
		t.Errorf("Expected image URL 'http://example.com/image.jpg', got '%v'", item.Image)
	}

	if len(item.Categories) != 1 || item.Categories[0] != "Tech" {
		t.Errorf("Expected categories ['Tech'], got %v", item.Categories)
	}

	if item.GUID != "12345" {
		t.Errorf("Expected GUID '12345', got '%s'", item.GUID)
	}
}
