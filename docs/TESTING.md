# Testing Guide

This document covers testing strategies and patterns for MrRSS.

## Backend Testing (Go)

### Unit Test Pattern

```go
func TestDatabaseOperations(t *testing.T) {
    // Setup test database
    db, cleanup := setupTestDB(t)
    defer cleanup()

    // Test data
    feed := models.Feed{
        Title: "Test Feed",
        URL:   "https://example.com/feed.xml",
    }

    // Execute
    id, err := db.AddFeed(feed)

    // Assert
    if err != nil {
        t.Fatalf("AddFeed failed: %v", err)
    }
    if id == 0 {
        t.Error("Expected non-zero ID")
    }

    // Verify
    retrieved, err := db.GetFeed(id)
    if err != nil {
        t.Fatalf("GetFeed failed: %v", err)
    }
    if retrieved.Title != feed.Title {
        t.Errorf("Expected title %q, got %q", feed.Title, retrieved.Title)
    }
}
```

### Test Helper Functions

```go
func setupTestDB(t *testing.T) (*database.DB, func()) {
    t.Helper()

    // Create temporary database
    tmpFile, err := os.CreateTemp("", "test-*.db")
    if err != nil {
        t.Fatal(err)
    }
    tmpFile.Close()

    // Initialize database
    db, err := database.New(tmpFile.Name())
    if err != nil {
        os.Remove(tmpFile.Name())
        t.Fatal(err)
    }

    // Return cleanup function
    cleanup := func() {
        db.Close()
        os.Remove(tmpFile.Name())
    }

    return db, cleanup
}
```

### Table-Driven Tests

```go
func TestValidateURL(t *testing.T) {
    tests := []struct {
        name    string
        url     string
        wantErr bool
    }{
        {"valid http", "http://example.com", false},
        {"valid https", "https://example.com", false},
        {"missing scheme", "example.com", true},
        {"invalid scheme", "ftp://example.com", true},
        {"empty url", "", true},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            err := validateURL(tt.url)
            if (err != nil) != tt.wantErr {
                t.Errorf("validateURL() error = %v, wantErr %v", err, tt.wantErr)
            }
        })
    }
}
```

## Frontend Testing (Vitest)

### Component Test Pattern

```javascript
import { describe, it, expect, vi } from 'vitest';
import { mount } from '@vue/test-utils';
import ArticleItem from './ArticleItem.vue';

describe('ArticleItem', () => {
  it('renders article title', () => {
    const article = {
      id: 1,
      title: 'Test Article',
      isRead: false
    };

    const wrapper = mount(ArticleItem, {
      props: { article }
    });

    expect(wrapper.text()).toContain('Test Article');
  });

  it('emits mark-read event when clicked', async () => {
    const article = {
      id: 1,
      title: 'Test Article',
      isRead: false
    };

    const wrapper = mount(ArticleItem, {
      props: { article }
    });

    await wrapper.trigger('click');

    expect(wrapper.emitted('mark-read')).toBeTruthy();
    expect(wrapper.emitted('mark-read')[0]).toEqual([article.id]);
  });

  it('shows unread indicator for unread articles', () => {
    const article = {
      id: 1,
      title: 'Test',
      isRead: false
    };

    const wrapper = mount(ArticleItem, {
      props: { article }
    });

    expect(wrapper.find('.unread-indicator').exists()).toBe(true);
  });
});
```

### Composable Testing

```javascript
import { describe, it, expect } from 'vitest';
import { useArticleDetail } from './useArticleDetail';

describe('useArticleDetail', () => {
  it('loads article correctly', async () => {
    const { article, loadArticle, isLoading } = useArticleDetail();

    // Initially null
    expect(article.value).toBeNull();

    // Load article
    await loadArticle(123);

    // Check result
    expect(isLoading.value).toBe(false);
    expect(article.value).not.toBeNull();
    expect(article.value.id).toBe(123);
  });
});
```

## Running Tests

### Backend Tests

```bash
# Run all tests
go test ./...

# Run tests with coverage
go test -cover ./...

# Run specific test
go test -run TestDatabaseOperations ./internal/database

# Verbose output
go test -v ./...
```

### Frontend Tests

```bash
cd frontend

# Run all tests
npm test

# Run with coverage
npm run test:coverage

# Watch mode
npm run test:watch

# Run specific test file
npm test ArticleItem.test.ts
```

## Test Coverage

### Backend Coverage Goals

- Database operations: 80%+
- Handler functions: 70%+
- Business logic: 80%+
- Utility functions: 90%+

### Frontend Coverage Goals

- Components: 70%+
- Composables: 80%+
- Utilities: 90%+

## Continue Reading

- [Architecture Overview](ARCHITECTURE.md)
- [Code Patterns](CODE_PATTERNS.md)
