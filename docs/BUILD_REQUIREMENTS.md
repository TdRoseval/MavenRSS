# Build Requirements

This document describes the system-level dependencies required for building MavenRSS on different platforms.

## Overview

MavenRSS uses Wails v3 (alpha) framework and a CGO SQLite stack:

- **Wails v3**: For the desktop application framework with built-in system tray
- **SQLite**: `github.com/mattn/go-sqlite3` with embedded `sqlite-vec`

## Important: CGO Requirement

⚠️ **CRITICAL**: Wails v3, `go-sqlite3`, and embedded `sqlite-vec` all require CGO to be enabled. You must set:

```bash
export CGO_ENABLED=1
```

Or when building:

```bash
CGO_ENABLED=1 wails3 build
```

## Platform-Specific Requirements

### Linux

#### Development Dependencies

```bash
sudo apt-get update
sudo apt-get install -y \
  gcc \
  pkg-config \
  libgtk-3-dev \
  libwebkit2gtk-4.1-dev \
  libsoup-3.0-dev
```

**Dependency Breakdown**:

- `gcc`: C compiler (required for CGO)
- `pkg-config`: Build tool for finding libraries
- `libgtk-3-dev`: GTK3 development headers (for Wails UI)
- `libwebkit2gtk-4.1-dev`: WebKit2GTK 4.1 development headers (for Wails webview, **required for Wails v3**)
- `libsoup-3.0-dev`: HTTP library 3.0 (required for Wails v3)
- `libm`: Provided by the system toolchain and linked automatically for `sqlite-vec`

**Important**: Wails v3 requires WebKit2GTK 4.1 and libsoup 3.0. Older versions (WebKit2GTK 4.0, libsoup 2.4) are not compatible.

**Note for Linux Mint**: Also install `libxapp-dev`

#### Runtime Dependencies

End users running the compiled binary will need:

- `libgtk-3-0`
- `libwebkit2gtk-4.1-0`
- `libsoup-3.0-0`

### Windows

#### Development Dependencies

Recommended for release builds and Windows ARM64:

```powershell
choco install zig nsis -y
```

**Dependency Breakdown**:

- `zig`: Recommended CGO compiler, required for Windows ARM64 builds
- `nsis`: Nullsoft Scriptable Install System (for creating installers)

Optional for native Windows AMD64 builds:

```powershell
choco install mingw -y
```

#### Alternative: Manual Installation

If not using Chocolatey:

1. Install [Zig](https://ziglang.org/download/) and add it to PATH
2. Install [NSIS](https://nsis.sourceforge.io/) if you need installers
3. For Windows AMD64 native builds, you may use [MinGW-w64](https://www.mingw-w64.org/) instead of Zig

Before `task windows:build`, you can prepare the compiler environment with:

```powershell
.\scripts\setup-windows-cgo.ps1 -Arch amd64
.\scripts\setup-windows-cgo.ps1 -Arch arm64
```

#### Build Flags

To avoid opening a console at application startup:

```bash
go build -ldflags "-H=windowsgui"
```

Or with Wails:

```bash
wails3 build -ldflags "-H=windowsgui"
```

#### Runtime Dependencies

Windows binaries are statically linked and don't require additional runtime dependencies.

### macOS

#### Development Dependencies

Install Xcode Command Line Tools (if not already installed):

```bash
xcode-select --install
```

**Note**: macOS has native support for systray through AppKit, so no additional libraries are needed.

#### Application Bundle

macOS requires an application bundle structure:

```plaintext
MavenRSS.app/
  Contents/
    Info.plist
    MacOS/
      MavenRSS
    Resources/
      MavenRSS.icns
```

Wails automatically creates this structure during build.

#### Info.plist Settings

Add these keys for better macOS integration:

```xml
<!-- High resolution support -->
<key>NSHighResolutionCapable</key>
<string>True</string>

<!-- Hide from Dock (optional, for menu bar only apps) -->
<key>LSUIElement</key>
<string>1</string>
```

#### Runtime Dependencies

macOS binaries are self-contained and don't require additional runtime dependencies.

## Building with Wails

### Standard Build

```bash
# Development build with hot reload
wails3 dev

# Production build (recommended: use Task)
task build

# Or directly with wails3
wails3 build

# Platform-specific build with Task
task linux:build
task windows:build
task darwin:build
```

### Build Configuration

Wails v3 uses `build/config.yml` for build configuration and Taskfile for platform-specific builds:

- **Frontend**: Automatically built via `frontend/package.json` scripts
- **Backend**: CGO-enabled Go build with platform-specific flags
- **Installers**: Created via platform-specific scripts (NSIS, create-dmg.sh, create-appimage.sh)

### Cross-Compilation

**Note**: Cross-compilation with CGO is complex. For best results:

- Build Linux binaries on Linux
- Build Windows binaries on Windows
- Build macOS binaries on macOS

GitHub Actions handles this automatically using platform-specific runners, runs SQLite startup self-check tests on Linux/Windows, and uses Zig for Windows release builds.

## GitHub Actions

Our CI/CD pipeline automatically installs all required dependencies:

### Test Workflow

- Installs Linux dependencies for backend tests
- Sets `CGO_ENABLED=1`

### Release Workflow

- Platform-specific dependency installation
- Cross-platform builds using native runners
- SQLite startup self-checks on Linux/Windows before packaging
- Artifact creation (installers, AppImages, DMGs)

## Troubleshooting

### "CGO is disabled" Error

**Solution**: Enable CGO before building:

```bash
export CGO_ENABLED=1
wails3 build
```

### SQLite startup self-check fails at `vec_version()`

If startup logs contain `Error running sqlite startup self-check` together with `query sqlite vec version`, the binary started but the embedded `sqlite-vec` extension was not available to SQLite.

Use this checklist:

1. Confirm CGO is enabled before building:

   ```bash
   export CGO_ENABLED=1
   ```

2. Confirm the repository still contains the local replacement dependency:

   - `third_party/sqlite-vec-go-bindings`
   - `go.mod` contains `replace github.com/asg017/sqlite-vec-go-bindings => ./third_party/sqlite-vec-go-bindings`

3. Refresh dependencies and rebuild:

   ```bash
   go mod tidy
   task build
   ```

4. Re-run the runtime smoke check:

   ```bash
   ./scripts/verify-server-runtime.sh build/bin/MavenRSS-server
   ```

   On Windows:

   ```powershell
   .\scripts\verify-server-runtime.ps1 -BinaryPath build/bin/MavenRSS-server.exe
   ```

If the smoke check still fails, inspect `data/logs/debug.log` for the exact self-check error before packaging artifacts.

### Build or packaging environment is missing `sqlite-vec`

Typical symptoms include build errors around `sqlite-vec.h`, linker errors for SQLite vector symbols, or a packaged binary that fails the startup self-check immediately on launch.

This project vendors `sqlite-vec` inside the repository instead of downloading it dynamically at runtime, so the fix is usually to restore a complete source tree in the build environment:

1. Ensure `third_party/sqlite-vec-go-bindings` is present in the workspace copied into CI, Docker, or packaging jobs.
2. Run `go mod tidy` after restoring the directory so the local `replace` target resolves correctly.
3. Rebuild with CGO enabled.

For cross-platform builds, prefer the provided Task workflows because they already prepare the expected CGO toolchain and build context.

### Linux: "Package webkit2gtk-4.1 was not found"

**Solution**: Install webkit2gtk-4.1 development headers:

```bash
sudo apt-get install libwebkit2gtk-4.1-dev
```

### Linux: "Package ayatana-appindicator3-0.1 was not found"

This error is from older versions. Wails v3 uses its own system tray implementation.

### Linux: "Package libsoup-3.0 was not found"

**Solution**: Install libsoup3 development headers:

```bash
sudo apt-get install libsoup-3.0-dev
```

### Linux: Binary starts to launch but exits because runtime libraries are missing

Typical errors include `error while loading shared libraries`, missing `libwebkit2gtk-4.1.so.0`, missing `libsoup-3.0.so.0`, or no `SQLite self-check passed` line appearing in `data/logs/debug.log`.

Install the runtime packages:

```bash
sudo apt-get update
sudo apt-get install -y libgtk-3-0 libwebkit2gtk-4.1-0 libsoup-3.0-0
```

Then verify the shipped binary directly:

```bash
./scripts/verify-server-runtime.sh build/bin/MavenRSS-server
```

If you are validating a desktop build instead of the server binary, start the executable from a terminal first so the missing shared library name is printed before re-packaging.

### Windows: "gcc: command not found"

**Solution**: Install Zig for release/ARM64 builds, or MinGW-w64 for native AMD64 builds:

```powershell
choco install zig -y
choco install mingw -y
```

Then run:

```powershell
.\scripts\setup-windows-cgo.ps1 -Arch amd64
```

### macOS: Missing Xcode Command Line Tools

**Solution**: Install Xcode Command Line Tools:

```bash
xcode-select --install
```

## Development Environment Setup

### Quick Setup Scripts

**Linux/macOS**:

```bash
# Install Go dependencies
go mod download

# Install frontend dependencies
cd frontend
npm install
cd ..

# Run development server
wails3 dev
```

**Windows (PowerShell)**:

```powershell
# Install Go dependencies
go mod download

# Install frontend dependencies
cd frontend
npm install
cd ..

# Run development server
wails3 dev
```

## Related Documentation

- [Architecture Overview](ARCHITECTURE.md)
- [Code Patterns](CODE_PATTERNS.md)
- [Testing Guide](TESTING.md)
