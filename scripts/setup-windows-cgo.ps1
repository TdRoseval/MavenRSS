param(
    [ValidateSet("amd64", "arm64")]
    [string]$Arch = "amd64"
)

$ErrorActionPreference = "Stop"

function Get-CommandPath {
    param([string]$Name)

    $command = Get-Command $Name -ErrorAction SilentlyContinue
    if ($null -eq $command) {
        return $null
    }
    return $command.Source
}

$zigPath = Get-CommandPath "zig"
$gccPath = Get-CommandPath "gcc"
$gxxPath = Get-CommandPath "g++"

$compiler = @{
    CGO_ENABLED = "1"
}

if ($Arch -eq "arm64") {
    if ([string]::IsNullOrWhiteSpace($zigPath)) {
        throw "Windows ARM64 build requires Zig in PATH."
    }

    $compiler.CC = $zigPath + " cc -target aarch64-windows-gnu"
    $compiler.CXX = $zigPath + " c++ -target aarch64-windows-gnu"
} elseif (-not [string]::IsNullOrWhiteSpace($zigPath)) {
    $compiler.CC = $zigPath + " cc -target x86_64-windows-gnu"
    $compiler.CXX = $zigPath + " c++ -target x86_64-windows-gnu"
} elseif (-not [string]::IsNullOrWhiteSpace($gccPath) -and -not [string]::IsNullOrWhiteSpace($gxxPath)) {
    $compiler.CC = $gccPath
    $compiler.CXX = $gxxPath
} elseif (-not [string]::IsNullOrWhiteSpace($gccPath)) {
    $compiler.CC = $gccPath
} else {
    throw "No Windows CGO compiler found. Install Zig, or install MinGW-w64 for native AMD64 builds."
}

foreach ($entry in $compiler.GetEnumerator()) {
    Set-Item -Path "Env:$($entry.Key)" -Value $entry.Value
    if ($env:GITHUB_ENV) {
        Add-Content -Path $env:GITHUB_ENV -Value "$($entry.Key)=$($entry.Value)"
    }
}

Write-Host "Configured Windows CGO toolchain for $Arch"
foreach ($entry in $compiler.GetEnumerator() | Sort-Object Key) {
    Write-Host "$($entry.Key)=$($entry.Value)"
}
