# OpenBowl System Tray Status Utility for Windows
# Creates a native Windows System Tray icon to monitor, start, and stop the Go backend server.

Add-Type -AssemblyName System.Windows.Forms
Add-Type -AssemblyName System.Drawing

$RootDir = (Resolve-Path "$PSScriptRoot/..").Path
$BinFile = Join-Path $RootDir "bin/openbowl-server.exe"

# Create context menu items
$contextMenu = New-Object System.Windows.Forms.ContextMenu
$menuStart = New-Object System.Windows.Forms.MenuItem("Start Server")
$menuStop = New-Object System.Windows.Forms.MenuItem("Stop Server")
$menuOpen = New-Object System.Windows.Forms.MenuItem("Open Dashboard")
$menuExit = New-Object System.Windows.Forms.MenuItem("Exit")

$contextMenu.MenuItems.AddRange(@($menuOpen, $menuStart, $menuStop, $menuExit))

# Create Tray Icon
$notifyIcon = New-Object System.Windows.Forms.NotifyIcon
$notifyIcon.ContextMenu = $contextMenu
$notifyIcon.Visible = $true

# Use standard Information icon for tray display
$notifyIcon.Icon = [System.Drawing.SystemIcons]::Information

# Monitor loop helper
function Get-ServerStatus {
    $conn = Get-NetTCPConnection -LocalPort 3010 -ErrorAction SilentlyContinue
    if ($conn) {
        return $true
    }
    return $false
}

function Update-TrayState {
    $isRunning = Get-ServerStatus
    if ($isRunning) {
        $notifyIcon.Text = "OpenBowl Server: Online (Port 3010)"
        $menuStart.Enabled = $false
        $menuStop.Enabled = $true
    } else {
        $notifyIcon.Text = "OpenBowl Server: Offline"
        $menuStart.Enabled = $true
        $menuStop.Enabled = $false
    }
}

# Start Server Handler
$menuStart.Add_Click({
    if (!(Test-Path $BinFile)) {
        [System.Windows.Forms.MessageBox]::Show("Server executable not found. Please compile it first by running install-startup.ps1", "OpenBowl Error")
        return
    }
    $WshShell = New-Object -ComObject WScript.Shell
    $WshShell.CurrentDirectory = $RootDir
    $WshShell.Run("`"$BinFile`"", 0, $false)
    
    Start-Sleep -Seconds 2
    Update-TrayState
    $notifyIcon.ShowBalloonTip(3000, "OpenBowl Server", "Server started successfully in the background.", [System.Windows.Forms.ToolTipIcon]::Info)
})

# Stop Server Handler
$menuStop.Add_Click({
    $conn = Get-NetTCPConnection -LocalPort 3010 -ErrorAction SilentlyContinue
    if ($conn) {
        Stop-Process -Id $conn.OwningProcess -Force -ErrorAction SilentlyContinue
        Start-Sleep -Seconds 1
        Update-TrayState
        $notifyIcon.ShowBalloonTip(3000, "OpenBowl Server", "Server stopped successfully.", [System.Windows.Forms.ToolTipIcon]::Info)
    }
})

# Open Dashboard Handler
$menuOpen.Add_Click({
    Start-Process "http://localhost:3000"
})

# Exit Handler
$menuExit.Add_Click({
    $notifyIcon.Visible = $false
    $notifyIcon.Dispose()
    [System.Windows.Forms.Application]::Exit()
})

# Setup timer to periodically refresh status
$timer = New-Object System.Windows.Forms.Timer
$timer.Interval = 2000
$timer.Add_Tick({ Update-TrayState })
$timer.Start()

# Initial state refresh
Update-TrayState
$notifyIcon.ShowBalloonTip(3000, "OpenBowl Active Monitor", "Monitoring OpenBowl Core on port 3010. Right-click to configure.", [System.Windows.Forms.ToolTipIcon]::Info)

# Run Application Message Loop
[System.Windows.Forms.Application]::Run()
