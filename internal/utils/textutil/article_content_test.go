package textutil

import (
	"strings"
	"testing"
)

func TestNormalizeArticleContentConvertsMarkdownAndStripsTOC(t *testing.T) {
	input := `股神巴菲特（Warren Buffett）於2026年初正式卸任波克夏海瑟威（Berkshire Hathaway）執行長後，首度接受[CNBC專訪](https://www.cnbc.com/video/2026/03/31/watch-cnbcs-full-interview-with-berkshire-hathaway-chairman-warren-buffett.html)。這場長達一小時的對話中，巴菲特面對當前市場修正直言「現在找不到值得買的東西」。

Table of Contents [Toggle](https://example.com/toggle)
- [股市買點還沒出現](https://example.com/1)
- [蘋果賣早了](https://example.com/2)

## 巴菲特：現在還不是買點

現金部位仍然很高。`

	got := NormalizeArticleContent(input)
	if !strings.Contains(got, "<a href=\"https://www.cnbc.com/video/2026/03/31/watch-cnbcs-full-interview-with-berkshire-hathaway-chairman-warren-buffett.html\"") {
		t.Fatalf("normalized content did not render markdown links: %q", got)
	}
	if strings.Contains(got, "Table of Contents") {
		t.Fatalf("normalized content still contains TOC boilerplate: %q", got)
	}
	if strings.Contains(got, "Toggle") {
		t.Fatalf("normalized content still contains toggle boilerplate: %q", got)
	}
	if strings.Contains(got, "[股市買點還沒出現]") {
		t.Fatalf("normalized content still contains raw markdown TOC links: %q", got)
	}
	if !strings.Contains(got, "<h2") || !strings.Contains(got, "巴菲特：現在還不是買點</h2>") {
		t.Fatalf("normalized content did not render markdown heading: %q", got)
	}
}

func TestNormalizeArticleContentRemovesHTMLTOCContainer(t *testing.T) {
	input := `<div id="table-of-contents"><p>Table of Contents</p><a href="#one">One</a><a href="#two">Two</a><a href="#three">Three</a></div><p>Main body</p>`
	got := NormalizeArticleContent(input)

	if strings.Contains(strings.ToLower(got), "table of contents") {
		t.Fatalf("normalized html still contains toc text: %q", got)
	}
	if !strings.Contains(got, "<p>Main body</p>") {
		t.Fatalf("normalized html removed main content unexpectedly: %q", got)
	}
}
