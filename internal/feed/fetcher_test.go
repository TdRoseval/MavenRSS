package feed

import (
	"MavenRSS/internal/models"
	"MavenRSS/internal/store/sqlite"
	"testing"
)

func TestNewFetcherSanity(t *testing.T) {
	db, err := sqlite.NewDB(":memory:")
	if err != nil {
		t.Fatalf("NewDB error: %v", err)
	}
	if err := db.Init(); err != nil {
		t.Fatalf("db Init error: %v", err)
	}

	f := NewFetcher(db)
	if f == nil {
		t.Fatal("NewFetcher returned nil")
	}
}

func TestCacheArticleContentsSkipsMissingArticleID(t *testing.T) {
	db, err := sqlite.NewDB(":memory:")
	if err != nil {
		t.Fatalf("NewDB error: %v", err)
	}
	if err := db.Init(); err != nil {
		t.Fatalf("db Init error: %v", err)
	}

	f := &Fetcher{db: db}
	f.cacheArticleContents([]*ArticleWithContent{
		{
			Article: &models.Article{
				UserID:   1,
				UniqueID: "missing-article",
				Title:    "Missing Article",
				URL:      "https://example.com/missing",
			},
			Content: "cached content",
		},
	})

	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM article_contents`).Scan(&count); err != nil {
		t.Fatalf("count article contents: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected no article content rows for missing article ID, got %d", count)
	}
}
