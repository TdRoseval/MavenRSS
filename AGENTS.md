# AI Agent Guidelines for MrRSS

> **Quick Links**: [Architecture](docs/ARCHITECTURE.md) | [Code Patterns](docs/CODE_PATTERNS.md) | [Testing](docs/TESTING.md) | [Custom Scripts](docs/CUSTOM_SCRIPTS.md) | [Version Management](docs/VERSION_MANAGEMENT.md)

## Project Overview

**MrRSS** is a modern, privacy-focused, cross-platform desktop RSS reader.

### Tech Stack

- **Backend**: Go 1.24+ with Wails v2.11+ framework, SQLite with `modernc.org/sqlite`
- **Frontend**: Vue 3.5+ Composition API, Pinia, Tailwind CSS 3.3+, Vite 5+, TypeScript
- **Communication**: HTTP REST API (not Wails bindings)
- **Icons**: Phosphor Icons | **I18n**: vue-i18n (English/Chinese)

### Key Features

- 📰 **Feed Management**: RSS/Atom subscription with custom script support (Python, Shell, Node.js, Ruby, PowerShell)
- 📝 **Article Summarization**: Local TF-IDF + TextRank algorithms (no API required)
- 🌐 **Translation**: Google Translate (free) or DeepL API for titles, content, and summaries
- 🔍 **Smart Discovery**: Batch feed discovery from friend links with progress tracking
- 📋 **Smart Rules**: "If-then" filtering rules for automatic article organization
- 🎨 **Multimedia Support**: Enhanced rendering for images, audio, video, embedded content
- ⚡ **Performance**: Virtual scrolling, concurrent fetching, optimized SQLite queries
- 🎯 **Modern UI**: Dark/Light/Auto themes, keyboard shortcuts, context menus

📚 **Detailed Feature Documentation**: See [ARCHITECTURE.md](docs/ARCHITECTURE.md)

## Project Structure

```plaintext
MrRSS/
├── main.go                      # Application entry point
├── wails.json                   # Wails configuration
├── internal/                    # Backend Go code
│   ├── database/                # SQLite operations
│   ├── discovery/               # Feed discovery system
│   ├── feed/                    # RSS/Atom parsing, script execution
│   ├── handlers/                # HTTP API handlers (organized by feature)
│   │   ├── core/                # Core handler and scheduler
│   │   ├── article/             # Article operations
│   │   ├── feed/                # Feed management
│   │   ├── discovery/           # Discovery endpoints
│   │   ├── script/              # Custom script execution
│   │   ├── summary/             # Article summarization
│   │   ├── translation/         # Translation services
│   │   └── ...
│   ├── models/                  # Core data structures
│   ├── opml/                    # OPML import/export
│   ├── rules/                   # Filtering rules engine
│   ├── summary/                 # TF-IDF + TextRank algorithms
│   ├── translation/             # Google Translate + DeepL
│   ├── utils/                   # Platform utilities
│   └── version/                 # Version constant
├── frontend/src/
│   ├── components/
│   │   ├── article/             # Article display (List, Detail, Content, parts/)
│   │   ├── sidebar/             # Feed list sidebar
│   │   ├── common/              # Reusable (Toast, ContextMenu, ImageViewer)
│   │   └── modals/              # Modal dialogs
│   │       ├── settings/        # Settings tabs (general/, feeds/, rules/, shortcuts/, about/)
│   │       ├── feed/            # Add/Edit feed
│   │       ├── filter/          # Article filters
│   │       ├── rules/           # Rules editor
│   │       └── discovery/       # Feed discovery
│   ├── composables/             # Reusable logic (article/, feed/, discovery/, rules/, ui/, core/)
│   ├── stores/                  # Pinia global state
│   ├── types/                   # TypeScript definitions
│   └── i18n/                    # Translations (en, zh)
├── docs/                        # Detailed documentation
│   ├── ARCHITECTURE.md          # System architecture
│   ├── CODE_PATTERNS.md         # Backend patterns
│   ├── CODE_PATTERNS_FRONTEND.md # Frontend patterns
│   ├── CODE_PATTERNS_STYLING.md # Styling patterns
│   ├── CODE_PATTERNS_API.md     # API communication
│   ├── TESTING.md               # Testing guide
│   ├── VERSION_MANAGEMENT.md    # Version update checklist
│   └── CUSTOM_SCRIPTS.md        # Custom scripts guide
├── scripts/                     # Development scripts (check, pre-release)
└── build/                       # Build configuration (windows/, linux/, macos/)
```

📚 **Detailed Structure**: See [ARCHITECTURE.md](docs/ARCHITECTURE.md)

## Key Technologies & Patterns

### Backend Architecture (Go 1.24+)

- **Framework**: Wails v2.11+ with HTTP API endpoints (not Wails bindings)
- **Database**: SQLite with `modernc.org/sqlite`, WAL mode enabled
- **RSS Parsing**: `gofeed` library with concurrent fetching
- **Translation**: Google Translate (free) + DeepL API
- **Concurrency**: Goroutines for parallel operations
- **Security**: Input validation, safe file operations, no shell injection

### Frontend Architecture (Vue 3.5+)

- **Framework**: Vue 3.5+ Composition API with TypeScript
- **State**: Pinia store for global state management
- **Styling**: Tailwind CSS 3.3+ with semantic classes and CSS variables
- **Build**: Vite 5+ for fast development
- **I18n**: vue-i18n with English/Chinese support
- **Icons**: Phosphor Icons

📚 **Detailed Patterns**: See [CODE_PATTERNS.md](docs/CODE_PATTERNS.md)

## Development Workflow

### Getting Started

1. **Prerequisites**: Go 1.24+, Node.js 18+, Wails CLI v2.11+
2. **Setup**: `go mod download && cd frontend && npm install`
3. **Development**: `wails dev` (hot reload enabled)
4. **Build**: `wails build` (production build)

### Development Scripts

**Linux/macOS:**

```bash
./scripts/check.sh            # Run all checks
./scripts/pre-release.sh      # Pre-release validation
```

**Windows (PowerShell):**

```powershell
.\scripts\check.ps1           # Run all checks
.\scripts\pre-release.ps1     # Pre-release validation
```

**Make:**

```bash
make help    # Show available commands
make check   # Run lint + test + build
make clean   # Clean artifacts
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
4. `frontend/package-lock.json` - "version" field
5. `frontend/src/components/modals/settings/about/AboutTab.vue` - appVersion ref default
6. `website/package.json` - "version" field
7. `website/package-lock.json` - "version" field
8. `README.md` - Version badge
9. `README_zh.md` - Version badge
10. `CHANGELOG.md` - Add new version entry

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

- Input validation for URLs, file paths, and user data
- Safe file operations (`os.Remove()` instead of shell commands)
- Prepared SQL statements prevent injection
- No `v-html` for user content (XSS risk)
- Script execution with timeout and path sandboxing

📚 **Code Examples**: See [CODE_PATTERNS.md](docs/CODE_PATTERNS.md)
