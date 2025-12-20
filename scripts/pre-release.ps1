# scripts/pre-release.ps1 - Pre-release checks

Write-Host "🚀 Running pre-release checks..." -ForegroundColor Green

# Run all checks
& "$PSScriptRoot\check.ps1"
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }

# Additional release checks
Write-Host "📦 Checking Go modules..." -ForegroundColor Blue
go mod tidy
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }

$goModStatus = git status --porcelain go.mod go.sum
if ($goModStatus) {
    Write-Host "❌ Go modules are not clean. Commit changes first." -ForegroundColor Red
    Write-Host $goModStatus
    exit 1
}
Write-Host "✅ Go modules clean" -ForegroundColor Green

Write-Host "📦 Checking frontend dependencies..." -ForegroundColor Blue
Push-Location frontend
npm audit --audit-level=moderate
if ($LASTEXITCODE -ne 0) { Pop-Location; exit $LASTEXITCODE }
Pop-Location
Write-Host "✅ Frontend dependencies OK" -ForegroundColor Green

# Check version consistency
Write-Host "🏷️  Checking version consistency..." -ForegroundColor Blue
$frontendVersion = (Get-Content package.json | ConvertFrom-Json).version
Write-Host "Frontend version: $frontendVersion" -ForegroundColor Cyan

$goVersion = (Get-Content internal/version/version.go | Select-String 'const Version = "(.+)"').Matches.Groups[1].Value
Write-Host "Backend version: $goVersion" -ForegroundColor Cyan

if ($frontendVersion -ne $goVersion) {
    Write-Host "❌ Version mismatch! Frontend: $frontendVersion, Backend: $goVersion" -ForegroundColor Red
    exit 1
}

$packageVersion = (Get-Content frontend/package.json | ConvertFrom-Json).version
Write-Host "Wails version: $packageVersion" -ForegroundColor Cyan

if ($frontendVersion -ne $packageVersion) {
    Write-Host "❌ Version mismatch! Frontend: $frontendVersion, Wails: $packageVersion" -ForegroundColor Red
    exit 1
}

Write-Host "✅ Version consistency OK" -ForegroundColor Green
Write-Host "🎉 Ready for release!" -ForegroundColor Green
