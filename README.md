# MavenRSS

<a href="https://trendshift.io/repositories/15731" target="_blank"><img src="https://trendshift.io/api/badge/repositories/15731" alt="WCY-dt%2FMavenRSS | Trendshift" style="width: 250px; height: 55px;" width="250" height="55"/></a>

![Screenshot](imgs/og1.png)

<p>
   <strong>English</strong> | <a href="README_zh.md">简体中文</a>
</p>

[![Version](https://img.shields.io/badge/version-1.3.20-blue.svg)](https://github.com/WCY-dt/MavenRSS/releases)
[![License](https://img.shields.io/badge/license-GPLv3-green.svg)](LICENSE)
[![Go](https://img.shields.io/badge/Go-1.25+-00ADD8?logo=go)](https://go.dev/)
[![Wails](https://img.shields.io/badge/Wails-v3%20alpha-red)](https://wails.io/)
[![Vue.js](https://img.shields.io/badge/Vue.js-3.5+-4FC08D?logo=vue.js)](https://vuejs.org/)

## ✨ Features

- 🌐 **Auto-Translation & Summarization**: Automatically translate article titles and content, and generate concise summaries to help you get information quickly
- 🤖 **AI-Enhanced Features**: Integrated advanced AI technology for translation, summarization, recommendations, and more, making reading smarter
- 🔌 **Rich Plugin Ecosystem**: Supports integration with mainstream tools like Obsidian, Notion, FreshRSS, and RSSHub for easy feature extension
- 📡 **Diverse Subscription Methods**: Supports URL, XPath, scripts, newsletters, and other feed types to meet different needs
- 🏭 **Custom Scripts & Automation**: Built-in filters and scripting system supporting highly customizable automation workflows

## 🚀 Quick Start

### Download and Install

#### Option 1: Download Pre-built Installer (Recommended)

Download the latest installer for your platform from the [Releases](https://github.com/WCY-dt/MavenRSS/releases/latest) page.

<details>

<summary>Click to view the list of available installers</summary>

<div markdown="1">

**Standard Installation:**

- **Windows:** `MavenRSS-{version}-windows-amd64-installer.exe` / `MavenRSS-{version}-windows-arm64-installer.exe`
- **macOS:** `MavenRSS-{version}-darwin-universal.dmg`
- **Linux:** `MavenRSS-{version}-linux-amd64.AppImage` / `MavenRSS-{version}-linux-arm64.AppImage`

**Portable Version** (no installation required, all data in one folder):

- **Windows:** `MavenRSS-{version}-windows-{arch}-portable.zip`
- **Linux:** `MavenRSS-{version}-linux-{arch}-portable.tar.gz`
- **macOS:** `MavenRSS-{version}-darwin-{arch}-portable.zip`

</div>

</details>

#### Option 2: Build from Source

<details>

<summary>Click to expand the build from source guide</summary>

<div markdown="1">

### Prerequisites

Before you begin, ensure you have the following installed:

- [Go](https://go.dev/) (1.25 or higher)
- [Node.js](https://nodejs.org/) (20 LTS or higher with npm)
- [Wails v3](https://v3alpha.wails.io/getting-started/installation/) CLI

**Platform-specific requirements:**

- **Linux**: GTK3, WebKit2GTK 4.1, libsoup 3.0, GCC, pkg-config
- **Windows**: MinGW-w64 (for CGO support), NSIS (for installers)
- **macOS**: Xcode Command Line Tools

For detailed installation instructions, see [Build Requirements](docs/BUILD_REQUIREMENTS.md)

```bash
# Quick setup for Linux (Ubuntu 24.04+):
sudo apt-get install libgtk-3-dev libwebkit2gtk-4.1-dev libsoup-3.0-dev gcc pkg-config
```

### Installation

1. **Clone the repository**

   ```bash
   git clone https://github.com/WCY-dt/MavenRSS.git
   cd MavenRSS
   ```

2. **Install frontend dependencies**

   ```bash
   cd frontend
   npm install
   cd ..
   ```

3. **Install Wails v3 CLI**

   ```bash
   go install github.com/wailsapp/wails/v3/cmd/wails3@latest
   ```

4. **Build the application**

   ```bash
   # Using Task (recommended)
   task build

   # Or using Makefile
   make build

   # Or directly with wails3
   wails3 build
   ```

   The executable will be created in the `build/bin` directory.

5. **Run the application**

   - Windows: `build/bin/MavenRSS.exe`
   - macOS: `build/bin/MavenRSS.app`
   - Linux: `build/bin/MavenRSS`

</div>

</details>

### Data Storage

<details>

<summary>Click to expand data storage details</summary>

<div markdown="1">

**Normal Mode** (default):

- **Windows:** `%APPDATA%\MavenRSS\` (e.g., `C:\Users\YourName\AppData\Roaming\MavenRSS\`)
- **macOS:** `~/Library/Application Support/MavenRSS/`
- **Linux:** `~/.local/share/MavenRSS/`

**Portable Mode** (when `portable.txt` exists):

- All data stored in `data/` folder

This ensures your data persists across application updates and reinstalls.

</div>

</details>

## 🛠️ Development Guide

<details>

<summary>Click to expand the development guide</summary>

<div markdown="1">

### Running in Development Mode

Start the application with hot reloading:

```bash
# Using Wails v3
wails3 dev

# Or using Task
task dev
```

### Code Quality Tools

#### Using Make

We provide a `Makefile` for handling common development tasks (available on Linux/macOS/Windows):

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

### Pre-commit Hooks

This project uses pre-commit hooks to ensure code quality:

```bash
# Install hooks
pre-commit install

# Run on all files
pre-commit run --all-files
```

### Running Tests

```bash
make test
```

### Server Mode (API-only)

For server deployments and API integration, use the headless server version:

```bash
# Using Docker (recommended)
docker run -p 1234:1234 mavenrss-server:latest

# Or build from source
go build -tags server -o mavenrss-server .
./mavenrss-server
```

Pre-built server images based on ghcr.io are also available:

```bash
docker run -d -p 1234:1234 ghcr.io/wcy-dt/mavenrss:latest-amd64
docker run -d -p 1234:1234 ghcr.io/wcy-dt/mavenrss:latest-arm64
```

Please refer to the [Server Mode API Documentation](docs/SERVER_MODE/swagger.json) for a complete API reference.

</div>

</details>

## 🤝 Contributing

We welcome contributions! Please see our [Contributing Guidelines](CONTRIBUTING.md) for details.

<details>

<summary>Click to expand the contributing guidelines</summary>

<div markdown="1">

Before contributing:

1. Read the [Code of Conduct](CODE_OF_CONDUCT.md)
2. Check existing issues or create a new one
3. Fork the repository and create a feature branch
4. Make your changes and add tests
5. Submit a pull request

</div>

</details>

## 🔒 Security

If you discover a security vulnerability, please follow our [Security Policy](SECURITY.md).

## 📝 License

This project is licensed under the GPL-3.0 License - see the [LICENSE](LICENSE) file for details.

## 📮 Contact & Support

- **Issues**: [GitHub Issues](https://github.com/WCY-dt/MavenRSS/issues)
- **Discussions**: [GitHub Discussions](https://github.com/WCY-dt/MavenRSS/discussions)
- **Repository**: [github.com/WCY-dt/MavenRSS](https://github.com/WCY-dt/MavenRSS)

---

<div align="center">
  <img src="imgs/sponsor.png" alt="Sponsor MavenRSS"/>
  <p>Made with ❤️ by the MavenRSS Team</p>
  <p>⭐ Star us on GitHub if you find this project useful!</p>
</div>
