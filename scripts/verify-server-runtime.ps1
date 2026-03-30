param(
    [Parameter(Mandatory = $true)]
    [string]$BinaryPath,
    [int]$Port = 18123,
    [string]$WorkDir = "",
    [string]$BindHost = "127.0.0.1",
    [int]$StartupTimeoutSeconds = 45
)

$ErrorActionPreference = "Stop"

if (-not (Test-Path $BinaryPath)) {
    throw "Binary not found: $BinaryPath"
}

if ([string]::IsNullOrWhiteSpace($WorkDir)) {
    $WorkDir = Join-Path ([System.IO.Path]::GetTempPath()) ("mavenrss-runtime-" + [guid]::NewGuid().ToString("N"))
}

New-Item -ItemType Directory -Force -Path $WorkDir | Out-Null

$StdoutLog = Join-Path $WorkDir "server.stdout.log"
$StderrLog = Join-Path $WorkDir "server.stderr.log"
$AppLog = Join-Path $WorkDir "data\logs\debug.log"
$ResponseFile = Join-Path $WorkDir "version.json"
$process = $null

function Stop-ServerProcess {
    if ($null -ne $process -and -not $process.HasExited) {
        Stop-Process -Id $process.Id -Force -ErrorAction SilentlyContinue
        $process.WaitForExit()
    }
}

function Write-LogFile {
    param(
        [string]$Path,
        [string]$Label
    )

    if (Test-Path $Path) {
        Write-Host "===== $Label ====="
        Get-Content -Path $Path -Tail 200
    }
}

function Write-Diagnostics {
    Write-Host "Runtime verification failed"
    Write-Host "Binary: $BinaryPath"
    Write-Host "Work directory: $WorkDir"
    Write-Host "Port: $Port"
    Get-Item $BinaryPath | Format-List FullName, Length, LastWriteTime
    if (Test-Path (Split-Path -Parent $BinaryPath)) {
        Get-ChildItem -Path (Split-Path -Parent $BinaryPath) | Format-Table Name, Length, LastWriteTime -AutoSize
    }
    if (Test-Path $WorkDir) {
        Get-ChildItem -Path $WorkDir -Recurse -Force | Select-Object FullName, Length, LastWriteTime | Format-Table -AutoSize
    }
    Write-LogFile -Path $StdoutLog -Label "server.stdout.log"
    Write-LogFile -Path $StderrLog -Label "server.stderr.log"
    Write-LogFile -Path $AppLog -Label "debug.log"
}

try {
    $resolvedBinaryPath = (Resolve-Path $BinaryPath).Path
    $process = Start-Process -FilePath $resolvedBinaryPath `
        -ArgumentList @("-host", $BindHost, "-port", "$Port") `
        -WorkingDirectory $WorkDir `
        -RedirectStandardOutput $StdoutLog `
        -RedirectStandardError $StderrLog `
        -PassThru

    Write-Host "Started server process $($process.Id) using $resolvedBinaryPath"

    $deadline = (Get-Date).AddSeconds($StartupTimeoutSeconds)
    $response = $null

    while ((Get-Date) -lt $deadline) {
        if ($process.HasExited) {
            Write-Diagnostics
            throw "Server exited before becoming ready"
        }

        try {
            $response = Invoke-WebRequest -Uri "http://${BindHost}:${Port}/api/version" -UseBasicParsing -TimeoutSec 5
            if ($response.StatusCode -eq 200) {
                break
            }
        }
        catch {
            Start-Sleep -Seconds 1
        }
    }

    if ($null -eq $response -or $response.StatusCode -ne 200) {
        Write-Diagnostics
        throw "Timed out waiting for runtime verification endpoint"
    }

    Set-Content -Path $ResponseFile -Value $response.Content -Encoding utf8

    $dbFile = Join-Path $WorkDir "data\rss.db"
    if (-not (Test-Path $dbFile)) {
        Write-Diagnostics
        throw "Expected database file was not created"
    }

    if (-not (Test-Path $AppLog)) {
        Write-Diagnostics
        throw "Expected debug log was not created"
    }

    $appLogContent = Get-Content -Path $AppLog -Raw
    if (-not $appLogContent.Contains("SQLite self-check passed")) {
        Write-Diagnostics
        throw "SQLite self-check success log not found"
    }

    Write-Host "Runtime verification response:"
    Get-Content -Path $ResponseFile
    Write-Host "Runtime verification passed"
}
finally {
    Stop-ServerProcess
}
