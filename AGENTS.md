# AI Agent Guidelines for MrRSS

## Project Overview

**MrRSS** is a modern, privacy-focused, cross-platform desktop RSS reader built with:

- **Backend**: Go 1.24+ with Wails v2.11+ framework
- **Frontend**: Vue 3.5+ (Composition API) with Pinia state management, Tailwind CSS 3.3+, Vite 5+
- **Database**: SQLite with `modernc.org/sqlite` driver (pure Go implementation)
- **Build Tool**: Wails CLI v2.11+
- **Additional**: Phosphor Icons, vue-i18n for internationalization

### Core Functionality

- **Feed Management**: RSS/Atom feed subscription, parsing with `gofeed`, concurrent fetching, and real-time updates
- **Article Management**: Read/unread tracking, favorites, pagination, filtering, and search
- **Organization**: Category-based feed organization with expandable categories and smart filtering rules
- **Translation**: Auto-translation using Google Translate (free, no API key) or DeepL API
- **Data Portability**: OPML import/export for easy migration between RSS readers
- **Internationalization**: Full UI support for English and Chinese with extensible i18n system
- **Auto-Refresh**: Configurable interval for automatic feed updates (default 10 minutes)
- **Auto-Cleanup**: Configurable article retention by age (preserves favorites) and cache size management
- **Update System**: In-app update checking and installation with progress tracking and safe file handling
- **Theming**: Light/Dark/Auto modes with system preference detection and CSS variables
- **Keyboard Shortcuts**: Customizable shortcuts for power users with keyboard navigation
- **Smart Discovery**: HTML parsing to find RSS feeds from websites and related sources
- **Performance**: Virtual scrolling, background processing, optimized database queries with WAL mode

### Advanced Features

- **Security**: Input validation, safe file operations, no shell command injection vulnerabilities
- **Accessibility**: Keyboard navigation, ARIA labels, screen reader support
- **Error Handling**: Graceful degradation, user-friendly error messages, comprehensive logging
- **Concurrent Processing**: Goroutines for parallel feed fetching and background tasks
- **Database Optimization**: WAL mode, prepared statements, indexed queries, VACUUM operations
- **Progress Tracking**: Real-time progress updates for long-running operations
- **Settings Auto-Save**: Debounced settings persistence (500ms) without save buttons
- **Context Menus**: Right-click context menus with customizable actions
- **Toast Notifications**: Non-intrusive notifications with different types (success/error/info)
- **Modal System**: Reusable modal dialogs with proper focus management
- **Responsive Design**: Mobile-friendly interface with resizable panels

## Project Structure

```plaintext
MrRSS/
├── main.go                      # Application entry point, Wails app initialization
├── wails.json                   # Wails configuration, version info (2 version fields)
├── go.mod / go.sum              # Go dependencies (Go 1.24+)
├── internal/                    # Backend Go code (private, not exposed)
│   ├── database/
│   │   ├── article_db.go        # Article CRUD operations
│   │   ├── cleanup_db.go        # Auto-cleanup logic (preserves favorites)
│   │   ├── db.go                # Database initialization and core operations
│   │   ├── feed_db.go           # Feed CRUD operations
│   │   ├── settings_db.go       # Settings key-value store operations
│   │   ├── sqlite.go            # SQLite connection and utilities
│   │   ├── sqlite_test.go       # Database unit tests
│   │   └── unread_test.go       # Unread count tests
│   ├── discovery/
│   │   ├── discovery_test.go    # Discovery unit tests
│   │   ├── errors.go            # Discovery error types
│   │   ├── feed_discovery.go    # Feed discovery from URLs
│   │   ├── html_parser.go       # HTML parsing for RSS links
│   │   ├── rss_detector.go      # RSS feed detection logic
│   │   └── service.go           # Discovery service orchestration
│   ├── feed/
│   │   ├── fetcher.go           # RSS/Atom parsing with gofeed, concurrent fetching
│   │   ├── fetcher_test.go      # Feed parsing and fetching tests
│   │   └── models.go            # Feed-specific data models
│   ├── handlers/
│   │   ├── article_handlers.go  # Article-related HTTP endpoints
│   │   ├── discovery_handlers.go # Feed discovery endpoints
│   │   ├── feed_handlers.go     # Feed management endpoints
│   │   ├── handler.go           # Handler initialization and common utilities
│   │   ├── opml_handlers.go     # OPML import/export endpoints
│   │   ├── rules_handlers.go    # Filtering rules endpoints
│   │   ├── scheduler.go         # Background task scheduling
│   │   ├── settings_handlers.go # Settings management endpoints
│   │   ├── translation_handlers.go # Translation endpoints
│   │   └── update_handlers.go   # Update system endpoints
│   ├── models/
│   │   └── models.go           # Core data structures (Feed, Article, etc.)
│   ├── opml/
│   │   ├── handler.go          # OPML parsing and generation
│   │   └── handler_test.go     # OPML unit tests
│   ├── rules/
│   │   └── engine.go           # Filtering rules engine
│   ├── translation/
│   │   ├── deepl.go            # DeepL API integration
│   │   ├── google_free.go      # Google Translate (free, no API key)
│   │   ├── translator.go       # Translation interface and factory
│   │   └── translator_test.go  # Translation unit tests
│   ├── utils/
│   │   ├── paths.go            # Platform-specific data paths
│   │   └── startup.go          # Application startup utilities
│   └── version/
│       └── version.go          # Version constant (CRITICAL: update all 7 files)
├── frontend/
│   ├── index.html               # HTML template
│   ├── package.json             # Frontend dependencies and scripts (version field)
│   ├── package.json.md5         # Dependency hash for caching
│   ├── postcss.config.js        # PostCSS configuration
│   ├── tailwind.config.js       # Tailwind CSS configuration
│   ├── tsconfig.json            # TypeScript configuration
│   ├── tsconfig.node.json       # Node.js TypeScript configuration
│   ├── vite.config.js           # Vite build configuration
│   ├── assets/                  # Static assets
│   └── src/
│       ├── App.vue              # Root Vue component
│       ├── main.ts              # Vue application initialization
│       ├── style.css            # Global styles and theme variables
│       ├── vite-env.d.ts        # Vite environment types
│       ├── components/
│       │   ├── article/
│       │   │   ├── ArticleContent.vue      # Article content renderer
│       │   │   ├── ArticleDetail.vue       # Article detail view
│       │   │   ├── ArticleDetailToolbar.vue # Article toolbar
│       │   │   ├── ArticleItem.vue         # Individual article item
│       │   │   ├── ArticleList.vue         # Article list with virtual scrolling
│       │   │   └── ArticleToolbar.vue      # Article list toolbar
│       │   ├── common/
│       │   │   ├── ContextMenu.vue         # Right-click context menu
│       │   │   ├── ImageViewer.vue         # Image viewer modal
│       │   │   └── Toast.vue               # Toast notification component
│       │   └── modals/
│       │       ├── SettingsModal.vue       # Main settings modal
│       │       ├── common/
│       │       │   ├── ConfirmDialog.vue   # Confirmation dialog
│       │       │   └── InputDialog.vue     # Input dialog
│       │       ├── discovery/
│       │       │   └── DiscoverFeedsModal.vue # Feed discovery modal
│       │       ├── feed/
│       │       │   ├── AddFeedModal.vue    # Add feed modal
│       │       │   └── EditFeedModal.vue   # Edit feed modal
│       │       ├── filter/
│       │       │   └── FilterRulesModal.vue # Filtering rules modal
│       │       └── settings/
│       │           ├── AboutTab.vue         # About tab (version field)
│       │           ├── FeedsTab.vue         # Feed settings tab
│       │           └── GeneralTab.vue       # General settings tab
│       ├── composables/
│       │   ├── article/            # Article-related composables
│       │   ├── core/               # Core utilities
│       │   ├── discovery/          # Discovery composables
│       │   ├── feed/               # Feed management composables
│       │   ├── filter/             # Filtering composables
│       │   ├── rules/              # Rules composables
│       │   └── ui/                 # UI composables (notifications, keyboard, etc.)
│       ├── i18n/
│       │   ├── index.ts            # i18n configuration and messages
│       │   └── types.ts            # i18n type definitions
│       │   └── locales/            # Locale files (en, zh)
│       ├── stores/
│       │   └── app.ts              # Pinia store for global state
│       ├── types/
│       │   ├── discovery.ts        # Discovery-related types
│       │   ├── filter.ts           # Filter-related types
│       │   ├── global.d.ts         # Global type definitions
│       │   ├── models.ts           # Core data model types
│       │   └── settings.ts         # Settings-related types
│       └── utils/
│           └── date.ts             # Date formatting utilities
│       └── wailsjs/                # Auto-generated Go→JS bindings (don't edit)
├── test/
│   └── testdata/               # Test data files (OPML samples, etc.)
├── build/                       # Build scripts and installers
│   ├── windows/
│   │   ├── info.json            # Windows build metadata
│   │   └── installer.nsi        # NSIS installer script
│   ├── linux/
│   │   └── create-appimage.sh   # AppImage creation script
│   └── macos/
│       └── create-dmg.sh        # DMG creation script
├── website/                     # GitHub Pages website
├── imgs/                        # Screenshots and assets
├── CHANGELOG.md                 # Version history (update this!)
├── README.md                    # English documentation (version badge)
├── README_zh.md                 # Chinese documentation (version badge)
├── SECURITY.md                  # Security policy
├── CONTRIBUTING.md              # Contribution guidelines
├── LICENSE                      # GPLv3 License
└── AGENTS.md                    # This file
```

## Key Technologies & Patterns

### Backend Architecture (Go 1.24+)

**Framework**: Wails v2.11+ with HTTP API endpoints (not Wails bindings for better control)
**Database**: SQLite with `modernc.org/sqlite` (pure Go implementation), WAL mode enabled
**RSS Parsing**: `gofeed` library for RSS/Atom feed parsing with concurrent fetching
**Translation**: Google Translate (free, no API key) and DeepL API integration
**Concurrency**: Goroutines for parallel feed fetching and background tasks
**Security**: Input validation, safe file operations, no shell command injection

### Frontend Architecture (Vue 3.5+)

**Framework**: Vue 3.5+ Composition API with TypeScript
**State Management**: Pinia store for global state (articles, feeds, filters, themes, refresh progress)
**Styling**: Tailwind CSS 3.3+ with semantic class system and CSS variables for theming
**Build Tool**: Vite 5+ for fast development and optimized production builds
**Internationalization**: vue-i18n with English/Chinese support
**Icons**: Phosphor Icons (Vue/Web) for consistent iconography

### Core Patterns

#### Database Operations

```go
// Always use prepared statements
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
```

#### Vue Composition API

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

// Reactive state and computed properties
const store = useAppStore();
const { t } = useI18n();
const isLoading = ref(false);

// Async operations with error handling
async function loadData() {
  isLoading.value = true;
  try {
    const data = await fetch('/api/articles').then(r => r.json());
    // Update store or local state
  } catch (error) {
    console.error('Failed to load data:', error);
    window.showToast(t('errorLoadingData'), 'error');
  } finally {
    isLoading.value = false;
  }
}

// Lifecycle and cleanup
onMounted(() => loadData());
onUnmounted(() => {
  // Cleanup timers, listeners, etc.
});
</script>
```

#### Auto-Save Pattern (500ms debounce)

```vue
<script setup>
import { watch } from 'vue';

let saveTimeout: NodeJS.Timeout | null = null;

async function autoSave() {
  try {
    await fetch('/api/settings', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(settings)
    });
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
</script>
```

## Development Workflow

### Getting Started

1. **Prerequisites**: Go 1.24+, Node.js 18+, Wails CLI v2.11+
2. **Clone & Setup**:

   ```bash
   git clone https://github.com/username/MrRSS.git
   cd MrRSS
   go mod download
   cd frontend && npm install
   ```

3. **Development**:

   ```bash
   wails dev  # Starts development server with hot reload
   ```

4. **Building**:

   ```bash
   wails build  # Production build for current platform
   ```

### Cross-Platform Development Scripts

The project includes automated scripts for development tasks:

**Linux/macOS:**

```bash
# Run all quality checks (lint, test, build)
./scripts/check.sh

# Pre-release checks
./scripts/pre-release.sh

# Bump version
./scripts/bump-version.sh 1.2.4
```

**Windows (PowerShell):**

```powershell
# Run all quality checks (lint, test, build)
.\scripts\check.ps1

# Pre-release checks
.\scripts\pre-release.ps1

# Bump version
.\scripts\bump-version.ps1 -NewVersion 1.2.4
```

### Using Make

A cross-platform Makefile is available with common tasks:

```bash
# Show all available commands
make help

# Run full check (lint + test + build)
make check

# Clean build artifacts
make clean

# Setup development environment
make setup
```

### Code Organization

- **Backend**: `internal/` contains all private Go code
- **Frontend**: `frontend/src/` follows Vue.js project structure
- **Tests**: Backend tests in `*_test.go`, frontend tests in `frontend/src/**/*.test.js`
- **Build Scripts**: Platform-specific build scripts in `build/` directory

### Version Management (CRITICAL)

When updating version, modify ALL of these files:

1. `internal/version/version.go` - Version constant
2. `wails.json` - "version" and "info.productVersion" fields
3. `frontend/package.json` - "version" field
4. `frontend/src/components/modals/settings/AboutTab.vue` - appVersion ref default
5. `README.md` - Version badge
6. `README_zh.md` - Version badge
7. `CHANGELOG.md` - Add new version entry

## Coding Standards

### Go Standards

- Use `context.Context` for all exported methods
- Error wrapping with `fmt.Errorf("operation failed: %w", err)`
- Prepared statements for all database queries
- Proper resource cleanup with `defer`
- Comprehensive input validation
- No shell command concatenation (security risk)

### Vue/TypeScript Standards

- Composition API with `<script setup>`
- Proper TypeScript typing for all props and data
- vue-i18n for all user-facing strings (`t()` function)
- Tailwind semantic classes (no inline styles)
- Debounced operations for performance
- Proper component lifecycle management

### Security Practices

- Input validation for URLs and file paths
- Safe file operations (`os.Remove()` not shell commands)
- No XSS vulnerabilities (`v-html` avoided)
- Prepared SQL statements prevent injection
- Proper error handling without information leakage

## Testing Patterns

### Backend Tests (Go)

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

### Frontend Tests (Vitest)

```javascript
import { describe, it, expect } from 'vitest';
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

## Deployment Process

### Build Commands

```bash
# Development
wails dev

# Production build for current platform
wails build

# Cross-platform builds
wails build -platform windows/amd64
wails build -platform linux/amd64
wails build -platform darwin/amd64
```

### Platform-Specific Packaging

- **Windows**: NSIS installer (`build/windows/installer.nsi`)
- **Linux**: AppImage (`build/linux/create-appimage.sh`)
- **macOS**: DMG (`build/macos/create-dmg.sh`)

### Release Checklist

1. Update version in all files below:
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
2. Update CHANGELOG.md
3. Run full test suite
4. Build for all platforms
5. Test installers on clean systems
6. Create GitHub release with binaries
7. Update website if needed

## Troubleshooting

### Common Issues

**Database Issues**:

- Check SQLite file permissions
- Verify WAL mode is enabled
- Run `VACUUM` to reclaim space

**Build Issues**:

- Ensure Go 1.24+ and Wails CLI v2.11+
- Clear `frontend/node_modules` and reinstall
- Check for conflicting dependencies

**Runtime Issues**:

- Check logs in application data directory
- Verify network connectivity for feed fetching
- Ensure proper file permissions for updates

### Debug Commands

```bash
# Check Go version and modules
go version
go mod verify

# Check Node.js and dependencies
cd frontend
npm --version
npm ls --depth=0

# Check Wails installation
wails version

# Run backend tests
go test ./internal/...

# Run frontend tests
cd frontend
npm test
```

### Performance Optimization

- Use database indexes for frequent queries
- Implement virtual scrolling for large lists
- Debounce frequent operations (search, auto-save)
- Use goroutines for concurrent operations
- Enable SQLite WAL mode for better concurrency

---

This document provides comprehensive guidance for AI agents working on the MrRSS project. Always refer to the current codebase for the latest patterns and ensure all changes follow the established conventions.
