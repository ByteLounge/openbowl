# OpenBowl Startup Installer for Windows
# Compiles the Go sidecar server and configures it to run silently in the background on startup.

$RootDir = (Resolve-Path "$PSScriptRoot/..").Path
$BinDir = Join-Path $RootDir "bin"
$BinFile = Join-Path $BinDir "openbowl-server.exe"
$MainPath = Join-Path $RootDir "packages/core/cmd/server/main.go"

# 1. Ensure bin directory exists
if (!(Test-Path $BinDir)) {
    New-Item -ItemType Directory -Path $BinDir | Out-Null
}

# 2. Build the Go Server binary
Write-Host "Building Go Sidecar Server..." -ForegroundColor Cyan
Push-Location (Join-Path $RootDir "packages/core")
& go build -o $BinFile $MainPath
Pop-Location

if ($LASTEXITCODE -ne 0) {
    Write-Error "Failed to build the Go server binary."
    exit 1
}
Write-Host "Go server compiled successfully at: $BinFile" -ForegroundColor Green

# 3. Create the Silent Launcher VBS file in Windows Startup Folder
$StartupDir = "$env:APPDATA\Microsoft\Windows\Start Menu\Programs\Startup"
$VbsPath = Join-Path $StartupDir "OpenBowlLauncher.vbs"

Write-Host "Configuring silent startup launcher..." -ForegroundColor Cyan
$VbsContent = @"
Dim WinScriptHost
Set WinScriptHost = CreateObject("WScript.Shell")
WinScriptHost.CurrentDirectory = "$RootDir"
WinScriptHost.Run """$BinFile""", 0
Set WinScriptHost = Nothing
"@

[System.IO.File]::WriteAllText($VbsPath, $VbsContent)
Write-Host "Created Startup Script at: $VbsPath" -ForegroundColor Green

# 4. Stop any existing instances running on port 3010
$ExistingProcess = Get-NetTCPConnection -LocalPort 3010 -ErrorAction SilentlyContinue
if ($ExistingProcess) {
    Write-Host "Stopping existing OpenBowl instance..." -ForegroundColor Yellow
    Stop-Process -Id $ExistingProcess.OwningProcess -Force -ErrorAction SilentlyContinue
    Start-Sleep -Seconds 1
}

# 5. Launch the server silently now so it is active immediately
Write-Host "Starting OpenBowl server silently in background..." -ForegroundColor Cyan
$WshShell = New-Object -ComObject WScript.Shell
$WshShell.CurrentDirectory = $RootDir
$WshShell.Run("`"$BinFile`"", 0, $false)

Write-Host "OpenBowl is now running silently in the background on http://localhost:3010!" -ForegroundColor Green
Write-Host "It will also start automatically every time you log into Windows." -ForegroundColor Green
