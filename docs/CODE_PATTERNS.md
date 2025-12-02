# Code Patterns for MrRSS

This document provides common coding patterns and best practices for the MrRSS project.

## Table of Contents

- [Backend Patterns (Go)](#backend-patterns-go)
- [Frontend Patterns (Vue)](#frontend-patterns-vue)
- [Styling Patterns](#styling-patterns)
- [API Communication](#api-communication)

## Backend Patterns (Go)

### Handler Method Pattern

Always use `context.Context` for exported methods and proper error wrapping:

```go
func (h *Handler) GetArticles(ctx context.Context, feedID int) ([]models.Article, error) {
    if feedID < 0 {
        return nil, errors.New("invalid feed ID")
    }

    articles, err := h.DB.GetArticles(ctx, feedID)
    if err != nil {
        return nil, fmt.Errorf("failed to get articles: %w", err)
    }

    return articles, nil
}
```

**Key Points**:

- Use `context.Context` as first parameter
- Validate inputs early
- Wrap errors with `fmt.Errorf` and `%w`
- Return zero values and errors, not panics

### Database Query Pattern

Always use prepared statements with proper cleanup:

```go
func (db *DB) GetArticlesByFeed(feedID int, isRead bool) ([]models.Article, error) {
    // Prepare statement
    stmt, err := db.conn.Prepare(`
        SELECT id, title, url, content, published_at, is_read, is_favorite
        FROM articles
        WHERE feed_id = ? AND is_read = ?
        ORDER BY published_at DESC
    `)
    if err != nil {
        return nil, fmt.Errorf("prepare statement: %w", err)
    }
    defer stmt.Close()

    // Execute query
    rows, err := stmt.Query(feedID, isRead)
    if err != nil {
        return nil, fmt.Errorf("execute query: %w", err)
    }
    defer rows.Close()

    // Scan results
    var articles []models.Article
    for rows.Next() {
        var article models.Article
        err := rows.Scan(
            &article.ID,
            &article.Title,
            &article.URL,
            &article.Content,
            &article.PublishedAt,
            &article.IsRead,
            &article.IsFavorite,
        )
        if err != nil {
            return nil, fmt.Errorf("scan row: %w", err)
        }
        articles = append(articles, article)
    }

    // Check for iteration errors
    if err := rows.Err(); err != nil {
        return nil, fmt.Errorf("iterate rows: %w", err)
    }

    return articles, nil
}
```

**Key Points**:

- Use prepared statements for all queries
- Always `defer Close()` on statements and rows
- Scan into proper types
- Check `rows.Err()` after iteration
- Use proper error wrapping

### Settings Management Pattern

Settings are stored as key-value strings in the database:

```go
// Get setting with default value
func (db *DB) GetSetting(key string, defaultValue string) string {
    var value string
    err := db.conn.QueryRow("SELECT value FROM settings WHERE key = ?", key).Scan(&value)
    if err == sql.ErrNoRows {
        return defaultValue
    }
    if err != nil {
        log.Printf("Error getting setting %s: %v", key, err)
        return defaultValue
    }
    return value
}

// Set setting
func (db *DB) SetSetting(key, value string) error {
    _, err := db.conn.Exec(`
        INSERT INTO settings (key, value)
        VALUES (?, ?)
        ON CONFLICT(key) DO UPDATE SET value = excluded.value
    `, key, value)
    if err != nil {
        return fmt.Errorf("set setting %s: %w", key, err)
    }
    return nil
}

// Get boolean setting
func (db *DB) GetBoolSetting(key string, defaultValue bool) bool {
    value := db.GetSetting(key, "")
    if value == "" {
        return defaultValue
    }
    return value == "true" || value == "1"
}

// Get integer setting
func (db *DB) GetIntSetting(key string, defaultValue int) int {
    value := db.GetSetting(key, "")
    if value == "" {
        return defaultValue
    }
    intValue, err := strconv.Atoi(value)
    if err != nil {
        return defaultValue
    }
    return intValue
}
```

### Cleanup Logic Pattern

Auto-cleanup preserves favorites:

```go
func (db *DB) CleanupOldArticles(maxAgeDays int) (int64, error) {
    cutoffDate := time.Now().AddDate(0, 0, -maxAgeDays)

    // IMPORTANT: Delete old articles EXCEPT favorites
    result, err := db.conn.Exec(`
        DELETE FROM articles
        WHERE published_at < ? AND is_favorite = 0
    `, cutoffDate)

    if err != nil {
        return 0, fmt.Errorf("cleanup articles: %w", err)
    }

    // Run VACUUM to reclaim space
    _, _ = db.conn.Exec("VACUUM")

    return result.RowsAffected()
}
```

**Critical**: Always exclude favorites (`is_favorite = 0`) in cleanup queries.

### Concurrent Processing Pattern

Use goroutines for parallel operations with proper error handling:

```go
func (f *Fetcher) FetchFeeds(feeds []models.Feed) map[int]error {
    errors := make(map[int]error)
    var mu sync.Mutex
    var wg sync.WaitGroup

    // Limit concurrent fetches
    semaphore := make(chan struct{}, 5) // Max 5 concurrent

    for _, feed := range feeds {
        wg.Add(1)
        go func(feed models.Feed) {
            defer wg.Done()

            // Acquire semaphore
            semaphore <- struct{}{}
            defer func() { <-semaphore }()

            // Fetch with timeout
            ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
            defer cancel()

            err := f.FetchFeed(ctx, feed)
            if err != nil {
                mu.Lock()
                errors[feed.ID] = err
                mu.Unlock()
            }
        }(feed)
    }

    wg.Wait()
    return errors
}
```

**Key Points**:

- Use `sync.WaitGroup` to wait for goroutines
- Use semaphore to limit concurrency
- Use `sync.Mutex` for shared state
- Always use context with timeout
- Capture loop variables properly

### Script Execution Pattern

Execute scripts safely with timeout and path validation:

```go
func (e *ScriptExecutor) ExecuteScript(ctx context.Context, scriptPath string) (*gofeed.Feed, error) {
    // Construct full path
    fullPath := filepath.Join(e.scriptsDir, scriptPath)
    fullPath = filepath.Clean(fullPath)
    cleanScriptsDir := filepath.Clean(e.scriptsDir)

    // Security: prevent directory traversal
    relPath, err := filepath.Rel(cleanScriptsDir, fullPath)
    if err != nil || strings.HasPrefix(relPath, "..") {
        return nil, fmt.Errorf("invalid script path: must be within scripts directory")
    }

    // Create context with timeout
    execCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
    defer cancel()

    // Determine command based on extension
    var cmd *exec.Cmd
    ext := strings.ToLower(filepath.Ext(fullPath))

    switch ext {
    case ".py":
        pythonCmd := "python3"
        if runtime.GOOS == "windows" {
            pythonCmd = "python"
        }
        cmd = exec.CommandContext(execCtx, pythonCmd, fullPath)
    case ".sh":
        if runtime.GOOS == "windows" {
            return nil, fmt.Errorf("shell scripts not supported on Windows")
        }
        cmd = exec.CommandContext(execCtx, "bash", fullPath)
    case ".js":
        cmd = exec.CommandContext(execCtx, "node", fullPath)
    default:
        cmd = exec.CommandContext(execCtx, fullPath)
    }

    // Set working directory
    cmd.Dir = e.scriptsDir

    // Capture output
    var stdout, stderr bytes.Buffer
    cmd.Stdout = &stdout
    cmd.Stderr = &stderr

    // Execute
    if err := cmd.Run(); err != nil {
        if stderr.Len() > 0 {
            return nil, fmt.Errorf("script failed: %v, stderr: %s", err, stderr.String())
        }
        return nil, fmt.Errorf("script failed: %v", err)
    }

    // Parse output as RSS feed
    parser := gofeed.NewParser()
    feed, err := parser.ParseString(stdout.String())
    if err != nil {
        return nil, fmt.Errorf("parse feed output: %w", err)
    }

    return feed, nil
}
```

**Security Checklist**:

- ✅ Path validation to prevent directory traversal
- ✅ Timeout enforcement (30 seconds)
- ✅ Working directory restriction
- ✅ No shell command concatenation
- ✅ Proper error handling with stderr capture

### Input Validation Pattern

Always validate user inputs, especially URLs and file paths:

```go
// Validate feed URL
func validateFeedURL(urlStr string) error {
    u, err := url.Parse(urlStr)
    if err != nil {
        return fmt.Errorf("invalid URL: %w", err)
    }

    if u.Scheme != "http" && u.Scheme != "https" {
        return errors.New("URL must use HTTP or HTTPS")
    }

    if u.Host == "" {
        return errors.New("URL must have a host")
    }

    return nil
}

// Validate file path to prevent directory traversal
func validateFilePath(baseDir, filePath string) error {
    cleanPath := filepath.Clean(filePath)
    cleanBase := filepath.Clean(baseDir)

    if !strings.HasPrefix(cleanPath, cleanBase) {
        return errors.New("invalid file path: path traversal detected")
    }

    return nil
}
```

### HTTP Handler Pattern

Standard HTTP handler with JSON response:

```go
func (h *Handler) HandleGetArticles(w http.ResponseWriter, r *http.Request) {
    // Parse query parameters
    feedIDStr := r.URL.Query().Get("feed_id")
    feedID, err := strconv.Atoi(feedIDStr)
    if err != nil {
        http.Error(w, "invalid feed_id", http.StatusBadRequest)
        return
    }

    // Call service layer
    articles, err := h.DB.GetArticles(feedID)
    if err != nil {
        log.Printf("Error getting articles: %v", err)
        http.Error(w, "internal server error", http.StatusInternalServerError)
        return
    }

    // Return JSON response
    w.Header().Set("Content-Type", "application/json")
    if err := json.NewEncoder(w).Encode(articles); err != nil {
        log.Printf("Error encoding response: %v", err)
    }
}
```

**Key Points**:

- Validate inputs from query params/body
- Return appropriate HTTP status codes
- Set proper Content-Type header
- Log errors (don't expose to client)
- Use `http.Error` for error responses

## Frontend Patterns (Vue)

### Vue Component Structure

#### Basic Component Pattern

```vue
<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted } from 'vue';
import { useAppStore } from '@/stores/app';
import { useI18n } from 'vue-i18n';

// Props with TypeScript
interface Props {
  article: Article;
  isActive?: boolean;
}

const props = withDefaults(defineProps<Props>(), {
  isActive: false
});

// Emits declaration
const emit = defineEmits<{
  update: [article: Article];
  delete: [id: number];
}>();

// Store and i18n
const store = useAppStore();
const { t } = useI18n();

// Reactive state
const isLoading = ref(false);
const items = ref<Article[]>([]);

// Computed properties
const filteredItems = computed(() =>
  items.value.filter(item => !item.isRead)
);

// Methods
async function loadData() {
  isLoading.value = true;
  try {
    const response = await fetch('/api/articles');
    items.value = await response.json();
  } catch (error) {
    console.error('Failed to load:', error);
    window.showToast(t('errorLoading'), 'error');
  } finally {
    isLoading.value = false;
  }
}

// Lifecycle
onMounted(() => {
  loadData();
});

onUnmounted(() => {
  // Cleanup timers, listeners, etc.
});
</script>

<template>
  <div class="component-container">
    <!-- Loading state -->
    <div v-if="isLoading" class="loading">
      {{ t('loading') }}
    </div>

    <!-- Empty state -->
    <div v-else-if="items.length === 0" class="empty">
      {{ t('noItems') }}
    </div>

    <!-- Content -->
    <div v-else class="items-list">
      <div
        v-for="item in filteredItems"
        :key="item.id"
        class="item"
        :class="{ 'active': item.id === props.article?.id }"
        @click="emit('update', item)"
      >
        {{ item.title }}
      </div>
    </div>
  </div>
</template>

<style scoped>
.component-container {
  @apply p-4 bg-bg-primary rounded-lg;
}

.item {
  @apply p-3 border border-border rounded cursor-pointer transition-colors;
}

.item:hover {
  @apply bg-bg-secondary;
}

.item.active {
  @apply bg-accent text-white;
}
</style>
```

**Key Points**:

- Use `<script setup>` for cleaner syntax
- Proper TypeScript typing for props and emits
- Separate concerns (state, computed, methods, lifecycle)
- Always use i18n for text (`t()` function)
- Handle loading and empty states

### Composables Pattern

#### Creating a Composable

```typescript
// composables/article/useArticleDetail.ts
import { ref, watch } from 'vue';
import { useI18n } from 'vue-i18n';
import type { Article } from '@/types/models';

export function useArticleDetail() {
  const { t } = useI18n();

  // State
  const article = ref<Article | null>(null);
  const isLoading = ref(false);
  const error = ref<string | null>(null);

  // Methods
  async function loadArticle(id: number) {
    isLoading.value = true;
    error.value = null;

    try {
      const response = await fetch(`/api/articles/${id}`);
      if (!response.ok) {
        throw new Error(`HTTP ${response.status}`);
      }
      article.value = await response.json();
    } catch (e) {
      error.value = e instanceof Error ? e.message : 'Unknown error';
      window.showToast(t('errorLoadingArticle'), 'error');
    } finally {
      isLoading.value = false;
    }
  }

  async function markAsRead(id: number) {
    try {
      await fetch(`/api/articles/${id}/read`, { method: 'POST' });
      if (article.value?.id === id) {
        article.value.isRead = true;
      }
    } catch (e) {
      console.error('Failed to mark as read:', e);
    }
  }

  // Return public API
  return {
    // State
    article,
    isLoading,
    error,

    // Methods
    loadArticle,
    markAsRead,

    // Translation
    t,
  };
}
```

**Key Points**:

- Export a function that returns reactive state and methods
- Include error handling
- Return only what's needed publicly
- Use proper TypeScript types

### Auto-Save Pattern

Debounced auto-save for settings (500ms delay):

```vue
<script setup lang="ts">
import { ref, watch, onUnmounted } from 'vue';

const settings = ref({
  theme: 'light',
  language: 'en',
  autoRefresh: true
});

let saveTimeout: NodeJS.Timeout | null = null;

async function autoSave() {
  try {
    await fetch('/api/settings', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(settings.value)
    });
    // Apply immediately for better UX
    store.applySettings(settings.value);
  } catch (error) {
    console.error('Auto-save failed:', error);
  }
}

function debouncedAutoSave() {
  if (saveTimeout) clearTimeout(saveTimeout);
  saveTimeout = setTimeout(autoSave, 500);
}

// Watch entire settings object deeply
watch(settings, debouncedAutoSave, { deep: true });

// Cleanup
onUnmounted(() => {
  if (saveTimeout) clearTimeout(saveTimeout);
});
</script>
```

**Key Points**:

- 500ms debounce delay
- Deep watch for nested objects
- Clear timeout on unmount to prevent memory leaks
- Apply settings immediately for better UX

## Styling Patterns

### Semantic Color Classes

Use these semantic class combinations for consistent theming:

#### Buttons

```html
<!-- Primary button -->
<button class="btn-primary">{{ t('save') }}</button>

<!-- Secondary button -->
<button class="btn-secondary">{{ t('cancel') }}</button>

<!-- Danger button -->
<button class="btn-danger">{{ t('delete') }}</button>
```

#### Form Elements

```html
<!-- Text input -->
<input
  class="input-field"
  type="text"
  :placeholder="t('enterText')"
/>

<!-- Textarea -->
<textarea class="input-field" rows="4"></textarea>

<!-- Select dropdown -->
<select class="input-field">
  <option value="">{{ t('selectOption') }}</option>
</select>
```

#### Cards and Containers

```html
<div class="bg-bg-primary border border-border rounded-lg p-4">
  <h3 class="text-text-primary font-semibold">{{ t('title') }}</h3>
  <p class="text-text-secondary text-sm">{{ t('description') }}</p>
</div>
```

### CSS Variables

Theme-aware colors using CSS variables:

```css
:root {
  --color-bg-primary: #ffffff;
  --color-bg-secondary: #f8fafc;
  --color-text-primary: #1e293b;
  --color-text-secondary: #64748b;
  --color-border: #e2e8f0;
  --color-accent: #3b82f6;
}

.dark-mode {
  --color-bg-primary: #0f172a;
  --color-bg-secondary: #1e293b;
  --color-text-primary: #f1f5f9;
  --color-text-secondary: #94a3b8;
  --color-border: #334155;
  --color-accent: #60a5fa;
}
```

### Component Styles

#### Button Styles

```css
.btn-primary {
  @apply px-4 py-2 bg-accent text-white rounded-lg font-medium transition-colors;
}

.btn-primary:hover {
  @apply brightness-110;
}

.btn-primary:disabled {
  @apply opacity-50 cursor-not-allowed;
}
```

#### Input Styles

```css
.input-field {
  @apply w-full px-3 py-2 border border-border rounded-lg bg-bg-primary text-text-primary;
}

.input-field:focus {
  @apply outline-none ring-2 ring-accent;
}
```

### Multimedia Styling

#### Images

```css
.prose :deep(img) {
  max-width: 100%;
  height: auto;
  border-radius: 0.5rem;
  margin: 1.5em 0;
  cursor: pointer;
  transition: opacity 0.2s;
}

.prose :deep(img:hover) {
  opacity: 0.9;
}
```

#### Audio Players

```css
.prose :deep(audio) {
  width: 100%;
  margin: 1.5em 0;
  border-radius: 0.75rem;
  background-color: var(--bg-secondary);
  border: 1px solid var(--border-color);
}
```

#### Video Players

```css
.prose :deep(video) {
  width: 100%;
  height: auto;
  margin: 1.5em 0;
  border-radius: 0.75rem;
  background-color: #000;
  box-shadow: 0 4px 6px -1px rgba(0, 0, 0, 0.1);
}
```

#### Embedded Content (iframes)

```css
.prose :deep(iframe) {
  width: 100%;
  aspect-ratio: 16 / 9;
  margin: 1.5em 0;
  border-radius: 0.75rem;
  border: none;
}
```

### Dark Mode Support

Use `:global(.dark-mode)` for dark mode styles:

```vue
<style scoped>
.button {
  background-color: rgba(255, 255, 255, 0.9);
  color: #212529;
}

:global(.dark-mode) .button {
  background-color: rgba(45, 45, 45, 0.9);
  color: #e0e0e0;
}
</style>
```

### Responsive Design

Use Tailwind responsive prefixes:

```html
<div class="p-2 sm:p-4 md:p-6">
  <h1 class="text-lg sm:text-xl md:text-2xl">{{ t('title') }}</h1>
</div>
```

## API Communication

### Frontend API Calls

MrRSS uses direct HTTP fetch (not Wails bindings) for better control.

#### GET Request

```javascript
// Simple GET
const response = await fetch('/api/articles');
const articles = await response.json();

// GET with query parameters
const params = new URLSearchParams({
  feed_id: '123',
  is_read: 'false',
  limit: '50'
});
const response = await fetch(`/api/articles?${params}`);
const articles = await response.json();
```

#### POST Request

```javascript
// POST with JSON body
await fetch('/api/settings', {
  method: 'POST',
  headers: { 'Content-Type': 'application/json' },
  body: JSON.stringify(settingsObject)
});

// POST without body
await fetch(`/api/articles/${id}/read`, {
  method: 'POST'
});
```

#### Error Handling

```javascript
try {
  const response = await fetch('/api/feeds');

  if (!response.ok) {
    throw new Error(`HTTP ${response.status}`);
  }

  const feeds = await response.json();
  // Process feeds...

} catch (error) {
  console.error('API call failed:', error);
  window.showToast(t('apiError'), 'error');
}
```

### Progress Tracking

For long-running operations (e.g., feed refresh):

```javascript
// Start operation
await fetch('/api/refresh', { method: 'POST' });

// Poll for progress
const pollInterval = setInterval(async () => {
  const response = await fetch('/api/progress');
  const data = await response.json();

  // Update progress
  progress.value = Math.round((data.current / data.total) * 100);

  // Check if complete
  if (!data.is_running) {
    clearInterval(pollInterval);
    // Refresh UI data
    await loadArticles();
  }
}, 500); // Poll every 500ms
```

### Backend HTTP Handlers

Standard pattern for HTTP handlers:

```go
func (h *Handler) HandleGetArticles(w http.ResponseWriter, r *http.Request) {
    // Parse query parameters
    feedIDStr := r.URL.Query().Get("feed_id")
    feedID, err := strconv.Atoi(feedIDStr)
    if err != nil {
        http.Error(w, "invalid feed_id", http.StatusBadRequest)
        return
    }

    // Call service/database
    articles, err := h.DB.GetArticles(feedID)
    if err != nil {
        log.Printf("Error: %v", err)
        http.Error(w, "internal error", http.StatusInternalServerError)
        return
    }

    // Return JSON
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(articles)
}
```

### API Endpoints

#### Articles

- `GET /api/articles` - List articles (with filters)
- `GET /api/articles/:id` - Get single article
- `POST /api/articles/:id/read` - Mark as read
- `POST /api/articles/:id/favorite` - Toggle favorite

#### Feeds

- `GET /api/feeds` - List all feeds
- `POST /api/feeds` - Add new feed
- `PUT /api/feeds/:id` - Update feed
- `DELETE /api/feeds/:id` - Delete feed
- `POST /api/feeds/:id/refresh` - Refresh single feed
- `POST /api/refresh` - Refresh all feeds

#### Settings

- `GET /api/settings` - Get all settings
- `POST /api/settings` - Save settings

#### Discovery

- `POST /api/discovery/single` - Discover from URL
- `POST /api/discovery/batch` - Batch discovery
- `GET /api/discovery/progress` - Get discovery progress

#### Summary

- `POST /api/summary` - Generate article summary

#### Translation

- `POST /api/translate` - Translate text
