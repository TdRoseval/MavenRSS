# Refactoring Progress

This document tracks the progress of the refactoring plan outlined in `REFACTOR_PLAN.md`.

## Phase 1: Backend Foundation (Safe)
- [x] 1. Create `internal/store/sqlite` directory.
- [x] 2. Move `internal/database` files to `internal/store/sqlite`.
- [x] 3. Update imports in `internal` packages.
- [x] 4. Verify application builds and tests pass.

## Phase 2: Backend API Flattening
- [x] 1. Create `internal/api` directory (Renamed from `internal/handlers`).
- [x] 2. Move `internal/handlers/*` logic into `internal/api` domain files.
- [x] 3. Update `main.go` router registration (Imports updated).

## Phase 3: Frontend Shared Layer
- [x] 1. Create `src/shared/ui` and `src/shared/lib` directories.
- [x] 2. Move generic components and utils.
- [x] 3. Update imports in existing components.

## Phase 4: Frontend Feature Slices
- [x] 1. Create `src/features` directory structure (Article, Feed, Discovery).
- [x] 2. Move components and composables into feature folders (Article, Feed, Discovery).
- [x] 3. Refactor `stores/app.ts` into feature stores (`ArticleStore`, `FeedStore`).
- [x] 4. Feed Slice
- [x] 5. Discovery Slice

## Phase 5: Cleanup & Verification
- [x] 1. Run full application tests.
- [x] 2. Remove unused files and directories.
- [x] 3. Verify all features work as expected.
