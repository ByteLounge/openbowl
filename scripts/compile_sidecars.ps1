# OpenBowl Sidecar Compilation Automation script
# Builds the Go Core sidecar server binaries for the Tauri desktop shell packaging.

$BinDir = Join-Path $PSScriptRoot "../apps/desktop/src-tauri/binaries"
if (!(Test-Path $BinDir)) {
    New-Item -ItemType Directory -Path $BinDir | Out-Null
}

$MainPath = Join-Path $PSScriptRoot "../packages/core/cmd/server/main.go"

# Navigate into the core module directory to resolve go.mod
Push-Location (Join-Path $PSScriptRoot "../packages/core")

# 1. Compile for Windows x86_64 target
Write-Host "Compiling Go Sidecar for Windows x86_64..."
$Env:GOOS = "windows"
$Env:GOARCH = "amd64"
$WinOutput = Join-Path $BinDir "server-x86_64-pc-windows-msvc.exe"
& "C:\Program Files\Go\bin\go.exe" build -o $WinOutput $MainPath
if ($LASTEXITCODE -eq 0) {
    Write-Host "Success: Created $WinOutput" -ForegroundColor Green
} else {
    Write-Warning "Failed to compile Windows x86_64 binary."
}

# 2. Compile for macOS arm64 (Apple Silicon) target
Write-Host "Compiling Go Sidecar for macOS arm64 (M1/M2)..."
$Env:GOOS = "darwin"
$Env:GOARCH = "arm64"
$MacArmOutput = Join-Path $BinDir "server-aarch64-apple-darwin"
& "C:\Program Files\Go\bin\go.exe" build -o $MacArmOutput $MainPath
if ($LASTEXITCODE -eq 0) {
    Write-Host "Success: Created $MacArmOutput" -ForegroundColor Green
} else {
    Write-Warning "Failed to compile macOS arm64 binary."
}

# 3. Compile for macOS x86_64 (Intel) target
Write-Host "Compiling Go Sidecar for macOS x86_64 (Intel)..."
$Env:GOOS = "darwin"
$Env:GOARCH = "amd64"
$MacIntelOutput = Join-Path $BinDir "server-x86_64-apple-darwin"
& "C:\Program Files\Go\bin\go.exe" build -o $MacIntelOutput $MainPath
if ($LASTEXITCODE -eq 0) {
    Write-Host "Success: Created $MacIntelOutput" -ForegroundColor Green
} else {
    Write-Warning "Failed to compile macOS x86_64 binary."
}

# 4. Compile for Linux x86_64 target
Write-Host "Compiling Go Sidecar for Linux x86_64..."
$Env:GOOS = "linux"
$Env:GOARCH = "amd64"
$LinuxOutput = Join-Path $BinDir "server-x86_64-unknown-linux-gnu"
& "C:\Program Files\Go\bin\go.exe" build -o $LinuxOutput $MainPath
if ($LASTEXITCODE -eq 0) {
    Write-Host "Success: Created $LinuxOutput" -ForegroundColor Green
} else {
    Write-Warning "Failed to compile Linux x86_64 binary."
}

# Reset Env variables
$Env:GOOS = ""
$Env:GOARCH = ""

Pop-Location
Write-Host "Sidecar compilation completed successfully!" -ForegroundColor Green
