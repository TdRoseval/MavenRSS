package sqlite

import (
	"strings"
	"testing"
)

func TestArticleContentCache(t *testing.T) {
	// Create a test database
	db, err := NewDB(":memory:")
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}
	defer db.DB.Close()

	// Initialize schema
	if err := db.Init(); err != nil {
		t.Fatalf("Failed to initialize database: %v", err)
	}

	t.Run("GetArticleContent - not found", func(t *testing.T) {
		content, found, err := db.GetArticleContent(999)
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
		if found {
			t.Error("Expected content to not be found")
		}
		if content != "" {
			t.Error("Expected empty content")
		}
	})

	t.Run("Set and Get ArticleContent", func(t *testing.T) {
		articleID := int64(1)
		// Insert dummy article first
		_, err = db.Exec(`INSERT INTO articles (id, user_id, title, url, published_at) VALUES (?, 1, 'Test', 'url1', CURRENT_TIMESTAMP)`, articleID)
		if err != nil {
			t.Fatalf("Failed to insert dummy article %d: %v", articleID, err)
		}

		testContent := "<p>This is test article content</p>"

		// Set content
		if err := db.SetArticleContent(articleID, testContent); err != nil {
			t.Errorf("Failed to set article content: %v", err)
		}

		// Get content
		content, found, err := db.GetArticleContent(articleID)
		if err != nil {
			t.Errorf("Failed to get article content: %v", err)
		}
		if !found {
			t.Error("Expected content to be found")
		}
		if content != testContent {
			t.Errorf("Content mismatch: got %q, want %q", content, testContent)
		}
	})

	t.Run("Update existing ArticleContent", func(t *testing.T) {
		articleID := int64(2)
		// Insert dummy article first
		_, err = db.Exec(`INSERT INTO articles (id, user_id, title, url, published_at) VALUES (?, 1, 'Test2', 'url2', CURRENT_TIMESTAMP)`, articleID)
		if err != nil {
			t.Fatalf("Failed to insert dummy article %d: %v", articleID, err)
		}

		initialContent := "<p>Initial content</p>"
		updatedContent := "<p>Updated content</p>"

		// Set initial content
		if err := db.SetArticleContent(articleID, initialContent); err != nil {
			t.Errorf("Failed to set initial content: %v", err)
		}

		// Verify initial content
		content, found, err := db.GetArticleContent(articleID)
		if err != nil || !found || content != initialContent {
			t.Error("Failed to retrieve initial content")
		}

		// Update content (should overwrite)
		if err := db.SetArticleContent(articleID, updatedContent); err != nil {
			t.Errorf("Failed to update content: %v", err)
		}

		// Verify updated content
		content, found, err = db.GetArticleContent(articleID)
		if err != nil {
			t.Errorf("Failed to get updated content: %v", err)
		}
		if !found {
			t.Error("Expected updated content to be found")
		}
		if content != updatedContent {
			t.Errorf("Updated content mismatch: got %q, want %q", content, updatedContent)
		}
	})

	t.Run("Get ArticleContent normalizes legacy markdown cache", func(t *testing.T) {
		articleID := int64(22)
		_, err = db.Exec(`INSERT INTO articles (id, user_id, title, url, published_at) VALUES (?, 1, 'Legacy', 'url22', CURRENT_TIMESTAMP)`, articleID)
		if err != nil {
			t.Fatalf("Failed to insert dummy article %d: %v", articleID, err)
		}

		legacyMarkdown := "Intro paragraph\n\nTable of Contents [Toggle](https://example.com/toggle)\n- [One](https://example.com/one)\n- [Two](https://example.com/two)\n\n## Heading"
		_, err = db.Exec(
			`INSERT INTO article_contents (article_id, content, fetched_at) VALUES (?, ?, CURRENT_TIMESTAMP)`,
			articleID, legacyMarkdown,
		)
		if err != nil {
			t.Fatalf("Failed to insert legacy article content: %v", err)
		}

		content, found, err := db.GetArticleContent(articleID)
		if err != nil {
			t.Fatalf("Failed to get normalized legacy content: %v", err)
		}
		if !found {
			t.Fatal("Expected normalized legacy content to be found")
		}
		if content == legacyMarkdown {
			t.Fatalf("Expected legacy markdown to be normalized, got original content: %q", content)
		}
		if content == "" {
			t.Fatal("Expected normalized content to be non-empty")
		}
		if contains := strings.Contains(content, "Table of Contents"); contains {
			t.Fatalf("Expected normalized content to remove TOC boilerplate: %q", content)
		}
	})

	t.Run("SetArticleContent rewrites in place without churning ids", func(t *testing.T) {
		articleID := int64(44)
		_, err = db.Exec(`INSERT INTO articles (id, user_id, title, url, published_at) VALUES (?, 1, 'Churn', 'url44', CURRENT_TIMESTAMP)`, articleID)
		if err != nil {
			t.Fatalf("Failed to insert dummy article %d: %v", articleID, err)
		}

		content := "<p>Churn check content</p>"
		if err := db.SetArticleContent(articleID, content); err != nil {
			t.Fatalf("Failed to set content: %v", err)
		}

		readRow := func() (int64, string, int64) {
			t.Helper()
			var rowID int64
			var fetchedAt string
			if err := db.QueryRow(`SELECT id, fetched_at FROM article_contents WHERE article_id = ?`, articleID).Scan(&rowID, &fetchedAt); err != nil {
				t.Fatalf("Failed to read cached row: %v", err)
			}
			var seq int64
			if err := db.QueryRow(`SELECT COALESCE((SELECT seq FROM sqlite_sequence WHERE name = 'article_contents'), 0)`).Scan(&seq); err != nil {
				t.Fatalf("Failed to read sqlite_sequence: %v", err)
			}
			return rowID, fetchedAt, seq
		}

		firstID, firstFetchedAt, firstSeq := readRow()

		// Re-caching identical content must be a no-op: the previous
		// "INSERT OR REPLACE" implementation deleted and reinserted the row,
		// allocating a new AUTOINCREMENT id on every feed refresh.
		if err := db.SetArticleContent(articleID, content); err != nil {
			t.Fatalf("Failed to re-set identical content: %v", err)
		}
		sameID, sameFetchedAt, sameSeq := readRow()
		if sameID != firstID {
			t.Errorf("Row id changed on identical rewrite: got %d, want %d", sameID, firstID)
		}
		if sameFetchedAt != firstFetchedAt {
			t.Errorf("fetched_at changed on identical rewrite: got %q, want %q", sameFetchedAt, firstFetchedAt)
		}
		if sameSeq != firstSeq {
			t.Errorf("AUTOINCREMENT sequence grew on identical rewrite: got %d, want %d", sameSeq, firstSeq)
		}

		// A genuine content change must update the existing row, not replace it.
		updated := "<p>Churn check content v2</p>"
		if err := db.SetArticleContent(articleID, updated); err != nil {
			t.Fatalf("Failed to update content: %v", err)
		}
		updatedID, _, updatedSeq := readRow()
		if updatedID != firstID {
			t.Errorf("Row id changed on content update: got %d, want %d", updatedID, firstID)
		}
		if updatedSeq != firstSeq {
			t.Errorf("AUTOINCREMENT sequence grew on content update: got %d, want %d", updatedSeq, firstSeq)
		}
		got, found, err := db.GetArticleContent(articleID)
		if err != nil || !found {
			t.Fatalf("Failed to read updated content: found=%v err=%v", found, err)
		}
		if got != "<p>Churn check content v2</p>" {
			t.Errorf("Unexpected stored content: %q", got)
		}
	})

	t.Run("Delete ArticleContent", func(t *testing.T) {
		articleID := int64(3)
		// Insert dummy article first
		_, err = db.Exec(`INSERT INTO articles (id, user_id, title, url, published_at) VALUES (?, 1, 'Test3', 'url3', CURRENT_TIMESTAMP)`, articleID)
		if err != nil {
			t.Fatalf("Failed to insert dummy article %d: %v", articleID, err)
		}

		testContent := "<p>Content to delete</p>"

		// Set content
		if err := db.SetArticleContent(articleID, testContent); err != nil {
			t.Errorf("Failed to set content: %v", err)
		}

		// Verify it exists
		_, found, err := db.GetArticleContent(articleID)
		if err != nil || !found {
			t.Error("Content should exist before deletion")
		}

		// Delete content
		if err := db.DeleteArticleContent(articleID); err != nil {
			t.Errorf("Failed to delete content: %v", err)
		}

		// Verify it's deleted
		_, found, err = db.GetArticleContent(articleID)
		if err != nil {
			t.Errorf("Error after deletion: %v", err)
		}
		if found {
			t.Error("Content should not exist after deletion")
		}
	})

	t.Run("CleanupOldArticleContents", func(t *testing.T) {
		// This test verifies the cleanup function works
		// Note: In an in-memory database, we can't test actual time-based cleanup
		// but we can verify the function executes without error
		affected, err := db.CleanupOldArticleContents(30, 0)
		if err != nil {
			t.Errorf("CleanupOldArticleContents failed: %v", err)
		}
		// In a fresh database, should affect 0 rows
		if affected != 0 {
			t.Errorf("Expected 0 rows affected, got %d", affected)
		}
	})
}
