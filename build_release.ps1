
# Build script for MagiTrickle Mod (Backend Only)
# Usage: .\build_release.ps1

Write-Host "Started MagiTrickle Mod Build..." -ForegroundColor Cyan

# Ensure bin directory exists
$BinDir = "bin_release"
if (!(Test-Path $BinDir)) {
    New-Item -ItemType Directory -Path $BinDir | Out-Null
}

$BackendDir = "src/backend"
$Version = "mod-v$(Get-Date -Format 'yyyy.MM.dd')"

# 1. MIPSLE Softfloat (Most keenetics: Viva, Lite, Omni, Extra...)
Write-Host "Building for MIPSLE (Softfloat)..." -NoNewline
$env:GOOS = "linux"
$env:GOARCH = "mipsle"
$env:GOMIPS = "softfloat"
go build -ldflags="-s -w -X 'magitrickle/constant.Version=$Version'" -tags "entware,entware_kn" -o "$BinDir/magitrickled_mipsle_softfloat" "$BackendDir/cmd/magitrickled"
if ($LASTEXITCODE -eq 0) { Write-Host " [OK]" -ForegroundColor Green } else { Write-Host " [FAILED]" -ForegroundColor Red }

# 2. MIPSLE Hardfloat (Old high-end: Giga II, Ultra)
Write-Host "Building for MIPSLE (Hardfloat)..." -NoNewline
$env:GOOS = "linux"
$env:GOARCH = "mipsle"
$env:GOMIPS = "hardfloat"
go build -ldflags="-s -w -X 'magitrickle/constant.Version=$Version'" -tags "entware,entware_kn" -o "$BinDir/magitrickled_mipsle_hardfloat" "$BackendDir/cmd/magitrickled"
if ($LASTEXITCODE -eq 0) { Write-Host " [OK]" -ForegroundColor Green } else { Write-Host " [FAILED]" -ForegroundColor Red }

# 3. ARMv7 (Giga III, Ultra II)
Write-Host "Building for ARMv7..." -NoNewline
$env:GOOS = "linux"
$env:GOARCH = "arm"
$env:GOARM = "7"
$env:GOMIPS = "" # Clear previous
go build -ldflags="-s -w -X 'magitrickle/constant.Version=$Version'" -tags "entware" -o "$BinDir/magitrickled_arm7" "$BackendDir/cmd/magitrickled"
if ($LASTEXITCODE -eq 0) { Write-Host " [OK]" -ForegroundColor Green } else { Write-Host " [FAILED]" -ForegroundColor Red }

# 4. ARM64 (Giga KN-1010, Ultra KN-1810, Peak)
Write-Host "Building for ARM64..." -NoNewline
$env:GOOS = "linux"
$env:GOARCH = "arm64"
$env:GOARM = "" # Clear previous
go build -ldflags="-s -w -X 'magitrickle/constant.Version=$Version'" -tags "entware" -o "$BinDir/magitrickled_arm64" "$BackendDir/cmd/magitrickled"
if ($LASTEXITCODE -eq 0) { Write-Host " [OK]" -ForegroundColor Green } else { Write-Host " [FAILED]" -ForegroundColor Red }

Write-Host "Done! Binaries are in $BinDir" -ForegroundColor Cyan
Write-Host "Note: It is recommended to compress binaries with UPX before deployment." -ForegroundColor Gray
