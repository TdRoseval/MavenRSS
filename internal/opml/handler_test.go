package opml

import (
	"MrRSS/internal/models"
	"strings"
	"testing"
)

func TestParse(t *testing.T) {
	xmlData := `
	<opml version="1.0">
		<head>
			<title>Test Subscriptions</title>
		</head>
		<body>
			<outline text="Tech" title="Tech">
				<outline type="rss" text="Hacker News" title="Hacker News" xmlUrl="https://news.ycombinator.com/rss" htmlUrl="https://news.ycombinator.com/"/>
			</outline>
			<outline type="rss" text="Go Blog" title="Go Blog" xmlUrl="https://blog.golang.org/feed.atom"/>
		</body>
	</opml>`

	r := strings.NewReader(xmlData)
	feeds, err := Parse(r)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	if len(feeds) != 2 {
		t.Errorf("Expected 2 feeds, got %d", len(feeds))
	}

	if feeds[0].Title != "Hacker News" {
		t.Errorf("Expected first feed title 'Hacker News', got '%s'", feeds[0].Title)
	}
	if feeds[0].Category != "Tech" {
		t.Errorf("Expected first feed category 'Tech', got '%s'", feeds[0].Category)
	}

	if feeds[1].Title != "Go Blog" {
		t.Errorf("Expected second feed title 'Go Blog', got '%s'", feeds[1].Title)
	}
	if feeds[1].Category != "" {
		t.Errorf("Expected second feed category '', got '%s'", feeds[1].Category)
	}
}

func TestGenerate(t *testing.T) {
	feeds := []models.Feed{
		{Title: "Feed 1", URL: "http://feed1.com/rss", Category: "Cat1"},
		{Title: "Feed 2", URL: "http://feed2.com/rss", Category: ""},
	}

	data, err := Generate(feeds)
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	xmlStr := string(data)
	if !strings.Contains(xmlStr, `xmlUrl="http://feed1.com/rss"`) {
		t.Error("Generated XML missing Feed 1 URL")
	}
	if !strings.Contains(xmlStr, `xmlUrl="http://feed2.com/rss"`) {
		t.Error("Generated XML missing Feed 2 URL")
	}
}
