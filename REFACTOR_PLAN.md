# MavenRSS Architecture Refactoring Plan

This document outlines a comprehensive plan to refactor the MavenRSS codebase. The goal is to improve code cohesion, reduce directory nesting, and simplify context for both human developers and LLM assistants, without altering existing functionality.

## 0. Requirement Check & Adjustments

This plan is reviewed against the original requirements and current codebase state:

- **No functional or performance change**: All changes are structural only.
- **Shorter code using language features**: Consolidate repeated patterns using Go generics and Vue Composition API.
- **File length control**: **CRITICAL UPDATE**. Analysis shows several files are already too large (`media_proxy.go` ~2000 lines, `ArticleContent.vue` ~1200 lines). The plan now explicitly includes **splitting** strategies for these "God Files".
- **High cohesion, low coupling**: All feature logic stays together.
- **Directory clarity**: Paths must signal module intent.

Updates below add specific guardrails, file-size thresholds, and safe refactor sequencing.

---

## 1. Backend Refactoring (Go)

### 1.1 High-Level Directory Structure

We will transition from a "Layer-based" structure to a "Domain-based" structure.

**Key Change**: We will **NOT** merge all logic into single huge files. Instead, we will use **Cohesive Bundles** - multiple files in the same folder/package.

```text
internal/
├── api/                    # [Formerly handlers] HTTP API Endpoints
│   ├── article/            # Article Domain
│   │   ├── handler.go      # Basic CRUD
│   │   ├── search.go       # Search & Filter
│   │   └── export.go       # Export logic
│   ├── feed/               # Feed Domain
│   │   ├── handler.go
│   │   └── discovery.go
│   └── ...
├── core/                   # Pure Domain Logic & Interfaces
│   ├── model/              # [Formerly models] Data structures
│   └── ports/              # Interface definitions
├── engine/                 # [Formerly feed] The Core RSS Engine
│   ├── fetcher/            # Network fetching
│   ├── parser/             # RSS/Atom/JSON parsing
│   └── worker/             # Background tasks (Scheduler, Cleanup)
├── store/                  # [Formerly database] Data Access Layer
│   ├── sqlite/             # SQLite Implementation
│   │   ├── db.go           # Init & Generic Helpers
│   │   ├── article_read.go # Fetch, List, Search
│   │   ├── article_write.go# Save, Update, Delete
│   │   ├── feed.go         # Feed CRUD
│   │   └── ...
│   └── query/              # Complex SQL queries
└── pkg/                    # [Formerly utils] Shared Utilities
    ├── crypto/
    ├── httpx/
    └── logger/
```

### 1.2 File Consolidation & Splitting Strategy

**Problem**: `article_db.go` + `article_sync.go` + `article_bulk.go` would exceed 2000 lines if merged.
**Solution**: Group by **Responsibility** within the same package.

| Current Path | New Path | Strategy |
|:---|:---|:---|
| `internal/database/article_*.go` | `internal/store/sqlite/article_*.go` | Keep separate files (Read/Write/Sync) but in one package. |
| `internal/handlers/media/media_proxy.go` | `internal/api/media/proxy.go` + `service/media/optimizer.go` | **SPLIT**: Separate HTTP handling from Image Processing logic. |
| `internal/feed/subscription.go` | `internal/engine/fetcher/subscription.go` | Keep as is, but move to engine. |

### 1.3 Code Improvements (Shorter Code)
*   **Generic Repository**: Implement `Get[T]`, `List[T]`, `Exec` helpers in `store/sqlite/db.go`.
    *   *Goal*: Reduce `row.Scan` boilerplate by ~40%.
*   **Unified Response**: Replace `map[string]interface{}` with typed `api.Response[T]` structs in `internal/api`.

---

## 2. Frontend Refactoring (Vue 3 + TypeScript)

### 2.1 High-Level Directory Structure

Move to a "Feature Slice" architecture.

```text
frontend/src/
├── app/                    # App bootstrapping
├── features/               # Vertical Slices
│   ├── article/            # Reading Experience
│   │   ├── components/     # UI Components
│   │   │   ├── content/    # [New] Split ArticleContent.vue here
│   │   │   ├── list/       # [New] Split ArticleList.vue here
│   │   │   └── ...
│   │   ├── composables/    # [Formerly logic] Domain Logic
│   │   │   ├── useReading.ts
│   │   │   └── useNavigation.ts
│   │   └── store.ts
│   ├── feed/               # Feed Management
│   └── ...
├── shared/                 # Cross-feature code
│   ├── ui/                 # Dumb Components
│   └── lib/                # Pure TS Utils
```

### 2.2 Component Splitting Strategy

**Problem**: `ArticleContent.vue` (1200 lines) and `ArticleList.vue` (1000 lines) are too big.
**Solution**: Decompose into sub-components.

| Current Component | Refactoring Action |
|:---|:---|
| `ArticleContent.vue` | Extract `ArticleHeader.vue`, `ArticleBody.vue`, `ArticleFooter.vue`, `ArticleTags.vue`. |
| `ArticleList.vue` | Extract `ArticleListItem.vue` (already exists but maybe too big), `ListVirtualScroller.vue`. |
| `useArticleDetail.ts` | **SPLIT** into `useArticleNavigation.ts` (prev/next), `useArticleState.ts` (read/unread). |
| `stores/app.ts` | **SPLIT** into `features/article/store.ts` (reading state), `features/feed/store.ts` (feed list). |

### 2.3 Logic Consolidation
*   **Composable Aggregation**: Combine granular composables like `useArticleActions`, `useArticleDetail` into `useArticleController.ts` where they share tight coupling.

---

## 3. Implementation Roadmap

### Phase 1: Backend Foundation (Safe)
1.  Create `internal/store/sqlite`.
2.  Move and merge `internal/database` files into `internal/store`.
3.  Update all imports in `internal/handlers` (search & replace).
4.  Verify application builds and tests pass.

### Phase 2: Backend API Flattening
1.  Create `internal/api`.
2.  Move `internal/handlers/*` logic into `internal/api` domain files.
3.  Update `main.go` router registration.

### Phase 3: Frontend Shared Layer
1.  Create `src/shared/ui` and `src/shared/lib`.
2.  Move generic components (`BaseModal`, `Button`) and utils.
3.  Update imports in existing components.

### Phase 4: Frontend Feature Slices
1.  Create `src/features/article`, `src/features/feed`, etc.
2.  Move components and composables into their respective feature folders.
3.  Refactor `stores/app.ts` into feature stores.

## 4. Immediate Benefits
*   **Context Window Efficiency**: LLMs can load a single `article.go` file to understand the entire data layer for articles, instead of 6+ files.
*   **Cognitive Load**: "Where is the code for X?" is answered by the folder name `features/X`.
*   **Refactoring Safety**: High cohesion means changing the Feed logic is less likely to break the Article logic.
