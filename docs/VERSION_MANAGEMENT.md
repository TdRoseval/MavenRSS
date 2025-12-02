# Version Management

## Version Update Checklist

**CRITICAL**: When updating the version, you MUST modify ALL of these files:

### Required File Updates

1. **`internal/version/version.go`**

   ```go
   const Version = "1.2.3"
   ```

2. **`wails.json`** - TWO fields

   ```json
   {
     "version": "1.2.3",
     "info": {
       "productVersion": "1.2.3"
     }
   }
   ```

3. **`frontend/package.json`**

   ```json
   {
     "version": "1.2.3"
   }
   ```

4. **`frontend/package-lock.json`**

   ```json
   {
     "version": "1.2.3",
     "packages": {
       "": {
         "version": "1.2.3"
       }
     }
   }
   ```

5. **`frontend/src/components/modals/settings/about/AboutTab.vue`**

   ```vue
   const appVersion = ref('1.2.3');
   ```

6. **`website/package.json`**

   ```json
   {
     "version": "1.2.3"
   }
   ```

7. **`website/package-lock.json`**

   ```json
   {
     "version": "1.2.3"
   }
   ```

8. **`README.md`** - Version badge

   ```markdown
   [![Version](https://img.shields.io/badge/version-1.2.3-blue.svg)]
   ```

9. **`README_zh.md`** - Version badge

   ```markdown
   [![Version](https://img.shields.io/badge/version-1.2.3-blue.svg)]
   ```

10. **`CHANGELOG.md`** - Add new version entry

    ```markdown
    ## [1.2.3] - 2025-11-27

    ### Added
    - New feature description

    ### Changed
    - Changed feature description

    ### Fixed
    - Bug fix description
    ```

## Semantic Versioning

Follow [Semantic Versioning](https://semver.org/):

- **MAJOR** (x.0.0): Breaking changes
- **MINOR** (1.x.0): New features (backwards compatible)
- **PATCH** (1.2.x): Bug fixes (backwards compatible)

### Examples

- `1.2.3` → `2.0.0`: Breaking API change
- `1.2.3` → `1.3.0`: New feature added
- `1.2.3` → `1.2.4`: Bug fix

## Release Process

1. Update version in all required files
2. Update `CHANGELOG.md` with changes
3. Run tests: `make check` or `./scripts/check.sh`
4. Commit changes: `git commit -m "Bump version to x.y.z"`
5. Create tag: `git tag -a vx.y.z -m "Version x.y.z"`
6. Push: `git push && git push --tags`
7. Build releases: `wails build`
8. Create GitHub release with binaries

## Pre-Release Checklist

Run the pre-release script:

```bash
# Linux/macOS
./scripts/pre-release.sh

# Windows
.\scripts\pre-release.ps1
```

This checks:

- All version strings match
- Tests pass
- Builds succeed
- No uncommitted changes

## Continue Reading

- [Architecture Overview](ARCHITECTURE.md)
- [Contributing Guide](../CONTRIBUTING.md)
