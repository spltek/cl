# Installs the latest (or a specific) cl release for Windows.
#
# Usage:
#   iwr https://raw.githubusercontent.com/silviopola/cl/main/install.ps1 | iex
#   .\install.ps1 -Tag v0.1.0
#
# Env vars:
#   CL_INSTALL_DIR   Where to put the binary (default: $env:LOCALAPPDATA\cl\bin)
param(
    [string]$Tag = ""
)

$ErrorActionPreference = "Stop"

$Repo = "silviopola/cl"
$InstallDir = if ($env:CL_INSTALL_DIR) { $env:CL_INSTALL_DIR } else { Join-Path $env:LOCALAPPDATA "cl\bin" }

if (-not [Environment]::Is64BitOperatingSystem) {
    throw "cl: unsupported architecture (32-bit Windows is not supported)."
}
$Arch = "amd64"

if (-not $Tag) {
    Write-Host "Resolving latest release..."
    $release = Invoke-RestMethod -Uri "https://api.github.com/repos/$Repo/releases/latest"
    $Tag = $release.tag_name
}

if (-not $Tag) {
    throw "cl: could not determine the release to install."
}

$Version = $Tag.TrimStart("v")
$Archive = "cl_${Version}_windows_${Arch}.zip"
$Url = "https://github.com/$Repo/releases/download/$Tag/$Archive"

$TmpDir = Join-Path ([System.IO.Path]::GetTempPath()) ([System.Guid]::NewGuid())
New-Item -ItemType Directory -Path $TmpDir | Out-Null

try {
    $ArchivePath = Join-Path $TmpDir $Archive
    Write-Host "Downloading $Url ..."
    Invoke-WebRequest -Uri $Url -OutFile $ArchivePath

    Expand-Archive -Path $ArchivePath -DestinationPath $TmpDir -Force

    New-Item -ItemType Directory -Force -Path $InstallDir | Out-Null
    Copy-Item -Path (Join-Path $TmpDir "cl.exe") -Destination (Join-Path $InstallDir "cl.exe") -Force
}
finally {
    Remove-Item -Recurse -Force $TmpDir -ErrorAction SilentlyContinue
}

Write-Host "Installed cl $Tag to $InstallDir\cl.exe"

$pathEntries = $env:Path -split ";"
if (-not ($pathEntries -contains $InstallDir)) {
    Write-Host ""
    Write-Host "Note: $InstallDir is not on your PATH. Add it, e.g.:"
    Write-Host "  `$env:Path += ';$InstallDir'"
}

Write-Host ""
Write-Host "Next: add shell integration to your PowerShell profile (`$PROFILE) so picked commands land on your prompt:"
Write-Host '  Invoke-Expression (cl init powershell | Out-String)'
