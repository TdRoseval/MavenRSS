# Build script for Windows using Wails

Write-Host "Building MrRSS with Wails..."

# Check if Wails is installed
if (-not (Get-Command "wails" -ErrorAction SilentlyContinue)) {
    Write-Error "Wails is not installed. Please run 'go install github.com/wailsapp/wails/v2/cmd/wails@latest'"
    exit 1
}

# Build the application
wails build -clean

if ($LASTEXITCODE -eq 0) {
    Write-Host "Build successful! Check the 'build/bin' directory."
} else {
    Write-Error "Build failed."
    exit 1
}
