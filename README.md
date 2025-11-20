# MrRSS

MrRSS is a modern, cross-platform desktop RSS reader built with [Wails](https://wails.io/), Go, and Vue.js.

## Features

- **Cross-Platform**: Runs on Windows, macOS, and Linux as a native desktop application.
- **RSS/Atom Feed Support**: Subscribe to your favorite feeds.
- **OPML Import/Export**: Easily migrate your subscriptions from other readers.
- **Translation**: Automatically translate article titles and content using Google Translate (Free) or DeepL.
- **Categories**: Organize your feeds into categories.
- **Favorites**: Save articles for later reading.
- **Read/Unread Tracking**: Keep track of what you've read.
- **Modern UI**: Clean and responsive interface built with Vue 3 and Tailwind CSS.

## Prerequisites

- [Go](https://go.dev/) (1.21+)
- [Node.js](https://nodejs.org/) (npm)
- [Wails](https://wails.io/docs/gettingstarted/installation) CLI

## Installation & Build

1. Clone the repository:

    ```bash
    git clone https://github.com/yourusername/MrRSS.git
    cd MrRSS
    ```

2. Install frontend dependencies:

    ```bash
    cd frontend
    npm install
    cd ..
    ```

3. Build the application:

    ```bash
    wails build
    ```

    The executable will be created in the `build/bin` directory.

## Development

To run the application in development mode with hot reloading:

```bash
wails dev
```

### Project Structure

- `main.go`: Wails application entry point.
- `wails.json`: Wails project configuration.
- `internal/`: Backend Go logic.
  - `database/`: SQLite database interactions.
  - `feed/`: Feed fetching and parsing logic.
  - `handlers/`: Application logic handlers exposed to frontend.
  - `models/`: Data structures.
  - `opml/`: OPML import/export processing.
  - `translation/`: Translation services (DeepL, Google).
- `frontend/`: Vue.js frontend.
  - `src/`: Source code (Vue components, stores, styles).
  - `wailsjs/`: Auto-generated Go bindings for JavaScript.

## License

[MIT](LICENSE)
