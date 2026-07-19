# Uninstalls cl for Windows.
#
# Usage:
#   iwr https://raw.githubusercontent.com/spltek/cl/main/uninstall.ps1 -UseBasicParsing | iex
#   .\uninstall.ps1

$ErrorActionPreference = "Stop"

$InstallDir = if ($env:CL_INSTALL_DIR) { $env:CL_INSTALL_DIR } else { Join-Path $env:LOCALAPPDATA "cl\bin" }
$ConfigDir = Join-Path $env:APPDATA "cl"

Write-Host "Removing binary from $InstallDir..."
if (Test-Path "$InstallDir\cl.exe") {
    Remove-Item "$InstallDir\cl.exe" -Force
} else {
    Write-Host "  cl.exe not found, skipping."
}

Write-Host "Removing config from $ConfigDir..."
if (Test-Path $ConfigDir) {
    Remove-Item $ConfigDir -Recurse -Force
} else {
    Write-Host "  Config directory not found, skipping."
}

# Remove install dir from User PATH
$currentPath = [Environment]::GetEnvironmentVariable("PATH", "User")
if ($currentPath) {
    $newPath = $currentPath.Replace(";${InstallDir}", "").Replace("${InstallDir};", "").Replace("${InstallDir}", "")
    if ($newPath -ne $currentPath) {
        [Environment]::SetEnvironmentVariable("PATH", $newPath, "User")
        Write-Host "  Removed from PATH."
    } else {
        Write-Host "  Not found in PATH, skipping."
    }
}

Write-Host ""
Write-Host "cl uninstalled successfully."
