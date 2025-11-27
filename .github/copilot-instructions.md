# GitHub Copilot Instructions for MrRSS

## Project Context

MrRSS is a modern, privacy-focused, cross-platform desktop RSS reader built with Wails (Go + Vue.js). It emphasizes simplicity, privacy, and modern UI design with the following core principles:

- **Privacy-First**: No user data collection, all data stored locally
- **Cross-Platform**: Native desktop apps for Windows, macOS, and Linux
- **Modern UI**: Clean, responsive interface with dark/light/auto themes
- **Performance**: Efficient SQLite database with optimized queries
- **Accessibility**: Keyboard shortcuts, proper ARIA labels, screen reader support

## Tech Stack

- **Backend**: Go 1.24+ with Wails v2.11+ framework, SQLite with `modernc.org/sqlite`
- **Frontend**: Vue 3.5+ (Composition API), Pinia state management, Tailwind CSS 3.3+, Vite 5+
- **Tools**: Wails CLI v2.11+, npm, Go modules
- **Icons**: Phosphor Icons (Vue/Web)
- **Internationalization**: vue-i18n with English/Chinese support

## Code Patterns

### Backend (Go)

When writing Go code, follow these patterns:

#### Handler Methods
```go
// Always use context for exported methods
func (h *Handler) MethodName(ctx context.Context, param string) (Result, error) {
    if param == "" {
        return Result{}, errors.New("param is required")
    }

    // Implementation with proper error wrapping
    result, err := h.DB.SomeOperation(ctx, param)
    if err != nil {
        return Result{}, fmt.Errorf("operation failed: %w", err)
    }

    return result, nil
}
```

#### Database Operations
```go
// Use prepared statements for all queries
stmt, err := db.conn.Prepare(`
    SELECT id, title, url, content, published_at
    FROM articles
    WHERE feed_id = ? AND is_read = ?
    ORDER BY published_at DESC
`)
if err != nil {
    return nil, fmt.Errorf("prepare statement: %w", err)
}
defer stmt.Close()

rows, err := stmt.Query(feedID, false)
if err != nil {
    return nil, fmt.Errorf("execute query: %w", err)
}
defer rows.Close()

// Scan with proper error handling
for rows.Next() {
    var article models.Article
    err := rows.Scan(&article.ID, &article.Title, &article.URL,
                    &article.Content, &article.PublishedAt)
    if err != nil {
        return nil, fmt.Errorf("scan row: %w", err)
    }
    articles = append(articles, article)
}

return articles, rows.Err()
```

#### Settings Management
```go
// Settings stored as key-value strings in database
func (db *DB) GetSetting(key string) (string, error) {
    var value string
    err := db.conn.QueryRow("SELECT value FROM settings WHERE key = ?", key).Scan(&value)
    if err == sql.ErrNoRows {
        return "", nil // Return empty string for missing settings
    }
    return value, err
}

### Frontend (Vue 3 Composition API)

When writing Vue components, follow these patterns:

#### Component Structure
```vue
<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted } from 'vue';
import { useAppStore } from '@/stores/app';
import { useI18n } from 'vue-i18n';

// Props with proper typing
interface Props {
  item: Article;
  isActive?: boolean;
}
const props = withDefaults(defineProps<Props>(), {
  isActive: false
});

// Emits declaration
const emit = defineEmits<{
  update: [item: Article];
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
  items.value.filter(item => item.isRead === false)
);

// Async operations with error handling
async function loadData() {
  isLoading.value = true;
  try {
    const data = await fetch('/api/articles').then(r => r.json());
    items.value = data;
  } catch (error) {
    console.error('Failed to load data:', error);
    window.showToast(store.i18n.t('errorLoadingData'), 'error');
  } finally {
    isLoading.value = false;
  }
}

// Lifecycle
onMounted(() => {
  loadData();
});

// Cleanup
onUnmounted(() => {
  // Cleanup timers, listeners, etc.
});
</script>

<template>
  <div class="component-container">
    <h2 class="text-lg font-semibold">{{ t('title') }}</h2>

    <div v-if="isLoading" class="loading-spinner">
      {{ t('loading') }}
    </div>

    <div v-else-if="items.length === 0" class="empty-state">
      {{ t('noItems') }}
    </div>

    <div v-else class="items-list">
      <div
        v-for="item in filteredItems"
        :key="item.id"
        class="item-card"
        :class="{ 'active': item.id === props.item?.id }"
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

.item-card {
  @apply p-3 border border-border rounded cursor-pointer transition-colors;
}

.item-card:hover {
  @apply bg-bg-secondary;
}

.item-card.active {
  @apply bg-accent text-white;
}
</style>
```

#### Auto-Save Settings Pattern
```vue
<script setup lang="ts">
import { watch, onUnmounted } from 'vue';

let saveTimeout: NodeJS.Timeout | null = null;

// Debounced auto-save function (500ms delay)
async function autoSave() {
  try {
    await fetch('/api/settings', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(settings)
    });
    // Apply settings immediately for better UX
    store.applySettings(settings);
  } catch (error) {
    console.error('Failed to save settings:', error);
  }
}

function debouncedAutoSave() {
  if (saveTimeout) clearTimeout(saveTimeout);
  saveTimeout = setTimeout(autoSave, 500);
}

// Watch entire settings object deeply
watch(() => props.settings, debouncedAutoSave, { deep: true });

// Cleanup to prevent memory leaks
## Styling Guidelines

### Tailwind CSS Semantic Classes

Use these semantic class combinations for consistent theming:

```html
<!-- Buttons -->
<button class="btn-primary">{{ t('save') }}</button>
<button class="btn-secondary">{{ t('cancel') }}</button>
<button class="btn-danger">{{ t('delete') }}</button>

<!-- Form Elements -->
<input class="input-field" type="text" :placeholder="t('enterText')" />
<textarea class="input-field" rows="4"></textarea>
<select class="input-field">
  <option value="">{{ t('selectOption') }}</option>
</select>

<!-- Cards and Containers -->
<div class="bg-bg-primary border border-border rounded-lg p-4">
  <h3 class="text-text-primary font-semibold">{{ t('title') }}</h3>
  <p class="text-text-secondary text-sm">{{ t('description') }}</p>
</div>

<!-- Status Indicators -->
<div class="status-indicator status-unread">{{ t('unread') }}</div>
<div class="status-indicator status-read">{{ t('read') }}</div>
<div class="status-indicator status-favorite">{{ t('favorite') }}</div>

<!-- Modal/Dialog -->
<div class="fixed inset-0 z-50 flex items-center justify-center bg-black/50 backdrop-blur-sm p-4">
  <div class="bg-bg-primary w-full max-w-2xl rounded-2xl shadow-2xl border border-border">
    <div class="modal-header">
      h2 class="text-xl font-bold">{{ t('modalTitle') }}</h2>
      <button @click="close" class="btn-icon">
        <i class="ph ph-x"></i>
      </button>
    </div>
    <div class="modal-body">
      <!-- Content -->
    </div>
  </div>
</div>
```

### CSS Variables and Theme System

```css
/* Theme-aware colors using CSS variables */
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

/* Component styles */
.btn-primary {
  @apply px-4 py-2 bg-accent text-white rounded-lg font-medium transition-colors;
}

.btn-primary:hover {
  @apply brightness-110;
}

.btn-primary:disabled {
  @apply opacity-50 cursor-not-allowed;
}

.input-field {
  @apply w-full px-3 py-2 border border-border rounded-lg bg-bg-primary text-text-primary;
}

## Internationalization

Always use i18n for user-facing strings. Never hardcode text:

```vue
<!-- Template -->
<h1>{{ t('welcome') }}</h1>
<p>{{ t('articleCount', { count: articles.length }) }}</p>
<button :title="t('clickToOpen')">{{ t('open') }}</button>

<!-- Script -->
const message = t('errorMessage');
window.showToast(t('successMessage'), 'success');
```

To add new strings, edit `frontend/src/i18n/index.ts`:

```typescript
export const messages = {
  en: {
    welcome: 'Welcome to MrRSS',
    articleCount: 'Found {count} articles',
    clickToOpen: 'Click to open',
    open: 'Open',
    errorMessage: 'An error occurred',
    successMessage: 'Operation completed successfully',
  },
## Common Patterns

### API Calls Pattern

MrRSS uses direct HTTP fetch calls (not Wails bindings) for better control:

```javascript
// GET request
const response = await fetch('/api/articles');
const articles = await response.json();

// POST request with body
await fetch('/api/settings', {
  method: 'POST',
  headers: { 'Content-Type': 'application/json' },
  body: JSON.stringify(settingsObject)
});

// Error handling
try {
  const res = await fetch('/api/feeds');
  if (!res.ok) throw new Error(`HTTP ${res.status}`);
  const feeds = await res.json();
} catch (error) {
  console.error('API call failed:', error);
  window.showToast(t('apiError'), 'error');
}
```

### Progress Tracking Pattern

For long-running operations like feed refresh:

```vue
<template>
  <button @click="handleRefresh" :disabled="refreshing">
    <i v-if="refreshing" class="ph ph-circle-notch animate-spin"></i>
    {{ refreshing ? `${t('refreshing')} ${progress}%` : t('refresh') }}
  </button>

  <!-- Progress bar -->
  <div v-if="refreshing" class="w-full bg-bg-tertiary rounded-full h-2 overflow-hidden">
    <div class="bg-accent h-full transition-all duration-300" :style="{ width: progress + '%' }"></div>
  </div>
</template>

<script setup>
const refreshing = ref(false);
const progress = ref(0);

async function handleRefresh() {
  refreshing.value = true;
  progress.value = 0;

  try {
    // Start the operation
    await fetch('/api/refresh', { method: 'POST' });

    // Poll for progress
    const pollInterval = setInterval(async () => {
      const res = await fetch('/api/progress');
      const data = await res.json();

      progress.value = Math.round((data.current / data.total) * 100);

      if (!data.is_running) {
        clearInterval(pollInterval);
        refreshing.value = false;
        // Refresh UI data
        await store.fetchArticles();
        await store.fetchUnreadCounts();
      }
    }, 500);
  } catch (error) {
    refreshing.value = false;
    console.error('Refresh failed:', error);
  }
}
</script>
```

### Toast Notifications

```javascript
// Success message
window.showToast(message, 'success');

// Error message
window.showToast(t('operationFailed'), 'error');

// Info message with custom duration
window.showToast(t('updateAvailable'), 'info', 5000);
```

### Confirm Dialogs

```javascript
const confirmed = await window.showConfirm(
  t('confirmDelete'),
  t('deleteWarning'),
  true  // isDanger - shows red confirmation button
);

if (confirmed) {
  // Proceed with dangerous operation
}
```

### Context Menu Pattern

```vue
<script setup>
import { useContextMenu } from '@/composables/ui/useContextMenu';

const { contextMenu, openContextMenu, closeContextMenu } = useContextMenu();

// Define menu items
const menuItems = [
  { label: t('edit'), action: 'edit', icon: 'ph-pencil' },
  { label: t('delete'), action: 'delete', icon: 'ph-trash', danger: true },
  { type: 'divider' },
  { label: t('markAsRead'), action: 'mark-read' }
];

// Handle right-click
function handleRightClick(event: MouseEvent, item: Article) {
  event.preventDefault();
  openContextMenu(event, menuItems, item);
}

// Handle menu action
function handleMenuAction(action: string, item: Article) {
  switch (action) {
    case 'edit':
      // Handle edit
      break;
    case 'delete':
      // Handle delete
      break;
    case 'mark-read':
      // Handle mark as read
      break;
  }
}
</script>

<template>
  <div @contextmenu="handleRightClick($event, item)">
    <!-- Item content -->
## Database Schema and Operations

### Core Tables

```sql
-- Feeds table
CREATE TABLE feeds (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    title TEXT NOT NULL,
    url TEXT UNIQUE NOT NULL,
    link TEXT,  -- Website homepage
    description TEXT,
    category TEXT,
    image_url TEXT,
    last_updated DATETIME,
    last_error TEXT,
    discovery_completed BOOLEAN DEFAULT FALSE
);

-- Articles table
CREATE TABLE articles (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    feed_id INTEGER NOT NULL,
    title TEXT NOT NULL,
    url TEXT UNIQUE NOT NULL,
    image_url TEXT,
    content TEXT,
    published_at DATETIME NOT NULL,
    is_read BOOLEAN DEFAULT FALSE,
    is_favorite BOOLEAN DEFAULT FALSE,
    is_hidden BOOLEAN DEFAULT FALSE,
    translated_title TEXT,
    FOREIGN KEY (feed_id) REFERENCES feeds(id) ON DELETE CASCADE
);

-- Settings table (key-value store)
CREATE TABLE settings (
    key TEXT PRIMARY KEY,
    value TEXT NOT NULL
);

-- Indexes for performance
CREATE INDEX idx_articles_feed_id ON articles(feed_id);
CREATE INDEX idx_articles_published_at ON articles(published_at);
CREATE INDEX idx_articles_is_read ON articles(is_read);
CREATE INDEX idx_articles_is_favorite ON articles(is_favorite);
```

### Cleanup Logic

Auto-cleanup preserves favorites:

```go
func (db *DB) CleanupOldArticles(maxAgeDays int) (int64, error) {
    cutoffDate := time.Now().AddDate(0, 0, -maxAgeDays)

    // Delete old articles EXCEPT favorites
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

## Security Best Practices

### Input Validation

Always validate user inputs, especially URLs and file paths:

```go
// Validate URL format and scheme
func validateFeedURL(urlStr string) error {
    u, err := url.Parse(urlStr)
    if err != nil {
        return fmt.Errorf("invalid URL: %w", err)
    }

    if u.Scheme != "http" && u.Scheme != "https" {
        return errors.New("URL must use HTTP or HTTPS")
    }

    return nil
}

// Validate file path to prevent directory traversal
func validateFilePath(baseDir, filePath string) error {
    cleanPath := filepath.Clean(filePath)
    if !strings.HasPrefix(cleanPath, filepath.Clean(baseDir)) {
        return errors.New("invalid file path: path traversal detected")
    }
    return nil
}
```

### Safe Command Execution

**NEVER** use shell command concatenation:

```go
// ❌ BAD: Command injection vulnerability
cmd := exec.Command("sh", "-c", "rm " + filePath)

// ✅ GOOD: Use Go standard library
if err := os.Remove(filePath); err != nil {
    return fmt.Errorf("remove file: %w", err)
}

// ✅ GOOD: If external command is necessary, use separate args
cmd := exec.Command("installer.exe", "/S") // No concatenation
```

### File Operations

Always clean up temporary files and use proper error handling:

```go
// Schedule cleanup with timeout
scheduleCleanup := func(filePath string, delay time.Duration) {
    go func() {
        time.Sleep(delay)
        if err := os.Remove(filePath); err != nil {
            log.Printf("Failed to cleanup %s: %v", filePath, err)
        } else {
            log.Printf("Cleaned up temporary file: %s", filePath)
        }
    }()
}

## Testing Patterns

### Backend Tests

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

### Frontend Tests

```javascript
import { describe, it, expect, vi } from 'vitest';
import { mount } from '@vue/test-utils';
import ArticleItem from './ArticleItem.vue';

describe('ArticleItem', () => {
  it('renders article title', () => {
    const article = { id: 1, title: 'Test Article', isRead: false };
    const wrapper = mount(ArticleItem, {
      props: { article }
    });

    expect(wrapper.text()).toContain('Test Article');
  });

  it('emits mark-read event when clicked', async () => {
    const article = { id: 1, title: 'Test Article', isRead: false };
    const wrapper = mount(ArticleItem, {
      props: { article }
    });

    await wrapper.trigger('click');
    expect(wrapper.emitted('mark-read')).toBeTruthy();
  });
});
```

## Version Management

**CRITICAL**: When updating version, modify ALL of these files:

1. `internal/version/version.go` - Version constant
2. `wails.json` - "version" and "info.productVersion" fields
3. `frontend/package.json` - "version" field
4. `frontend/src/components/modals/settings/AboutTab.vue` - appVersion ref default
5. `README.md` - Version badge
6. `README_zh.md` - Version badge
7. `CHANGELOG.md` - Add new version entry

Example version update:

```go
// internal/version/version.go
const Version = "1.3.0"
```

```json
// wails.json
{
  "version": "1.3.0",
  "info": {
    "productVersion": "1.3.0",
    ...
  }
}
```

## Don'ts

❌ **Don't**:

- Use `var` declarations in Vue (use `ref` or `reactive`)
- Hardcode user-facing strings (always use i18n `t()`)
- Use inline styles (use Tailwind classes or scoped styles)
- Forget error handling in async operations
- Use `any` type without strong justification
- Commit API keys, secrets, or sensitive data
- Use `v-html` for user content (XSS risk)
- Make breaking changes without migration path
- Use shell command concatenation (security risk)
- Create multiple deep watchers when one suffices
- Forget to clean up timers/intervals on component unmount
- Delete favorited articles during cleanup operations
- Use synchronous operations in UI thread for long tasks

## Do's

✅ **Do**:

- Use TypeScript with proper type annotations
- Follow existing code patterns and conventions
- Write comprehensive tests for new features
- Keep functions small and focused (single responsibility)
- Use meaningful variable and function names
- Handle all edge cases and error conditions
- Validate inputs thoroughly (URLs, file paths, user data)
- Log errors with appropriate context for debugging
- Use semantic HTML with proper ARIA attributes
- Debounce frequent operations (auto-save, search, etc.)
- Use `os.Remove()` instead of shell commands for file operations
- Clean up resources (timers, goroutines, event listeners) properly
- Preserve favorited articles during any cleanup operation
- Update all files below when bumping version:
  - `internal/version/version.go`
  - `wails.json`
  - `frontend/package.json`
  - `frontend/package-lock.json`
  - `frontend/src/components/modals/settings/AboutTab.vue`
  - `website/package.json`
  - `website/package-lock.json`
  - `README.md`
  - `README_zh.md`
  - `CHANGELOG.md`
- Use prepared statements for all database queries
- Implement proper loading states and progress indicators
- Follow semantic versioning (MAJOR.MINOR.PATCH)
- Document exported functions and complex logic
- Use goroutines for concurrent operations
- Implement graceful degradation for network failures

## Quick Reference

**Store Access**: `const store = useAppStore()`
**i18n**: `const { t } = useI18n()`
**Theme**: `store.theme` returns `'light'` or `'dark'`
**Language**: `store.i18n.locale.value` returns `'en'` or `'zh'`
**Toast**: `window.showToast(message, type)`
**Confirm**: `await window.showConfirm(title, message, isDanger)`
**Settings API**: `GET/POST /api/settings`
**Articles API**: `GET /api/articles` with query params
**Progress Polling**: `GET /api/progress` for async operations

---

When generating code, prioritize:

1. **Correctness**: Code that works and handles errors properly
2. **Consistency**: Follow existing patterns in the codebase
3. **Clarity**: Easy to understand and maintain
4. **Performance**: Efficient queries, minimal re-renders, proper cleanup
5. **Security**: Input validation, safe file operations, no injection vulnerabilities
6. **User Experience**: Loading states, progress indicators, error messages
