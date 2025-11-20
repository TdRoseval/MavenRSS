# Packaging MrRSS as a Standalone Windows Application with Wails

This guide explains how to package MrRSS as a native Windows application using [Wails](https://wails.io).

## Prerequisites

- [Go](https://go.dev/dl/) (version 1.21 or later) installed.
- [Wails](https://wails.io/docs/gettingstarted/installation) installed:

  ```powershell
  go install github.com/wailsapp/wails/v2/cmd/wails@latest
  ```

## How to Build

We have provided a PowerShell script to automate the build process.

1. Open a PowerShell terminal in the project root.
2. Run the build script:

    ```powershell
    .\build_windows.ps1
    ```

This will create the executable in `build/bin/MrRSS.exe`.

### Manual Build Command

If you prefer to run the command manually:

```powershell
wails build
```

## Architecture

The application uses Wails to wrap the existing Go backend and HTML/JS frontend.

- **Backend**: The Go HTTP handlers are reused and served via Wails' `AssetServer` handler.
- **Frontend**: The `frontend/` directory is embedded and served by Wails.
- **Communication**: The frontend makes standard `fetch` calls to `/api/...`, which are intercepted by the Wails application and routed to the Go handlers.

## Troubleshooting

- **Wails not found**: Ensure you have installed Wails and added the Go bin directory to your PATH.
- **Build fails**: Check if you have the necessary C++ build tools (MinGW or Visual Studio Build Tools) installed, which Wails requires for building the Windows frontend.
