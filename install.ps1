# Installs the latest (or a specific) cl release for Windows, and
# wires up PowerShell profile integration so a brand-new terminal
# works right away.
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

# --- Persist PATH for future terminals -------------------------------------
# Writing the User-scope PATH environment variable only touches the
# current user's registry hive (HKCU) and needs no admin/elevated
# rights - it's equivalent in permission terms to editing a file in
# %APPDATA%. Machine-scope PATH would need admin, so we deliberately
# don't touch that.
$userPath = [Environment]::GetEnvironmentVariable("Path", "User")
$userPathEntries = @()
if ($userPath) { $userPathEntries = $userPath -split ";" | Where-Object { $_ -ne "" } }

if (-not ($userPathEntries -contains $InstallDir)) {
    $newUserPath = if ($userPath) { "$userPath;$InstallDir" } else { $InstallDir }
    [Environment]::SetEnvironmentVariable("Path", $newUserPath, "User")
    Write-Host "  added $InstallDir to your User PATH (persists in new terminals)"
}

# Also update the current session so `cl` works immediately here too.
if (-not (($env:Path -split ";") -contains $InstallDir)) {
    $env:Path += ";$InstallDir"
}

# --- Wire up the PowerShell profile -----------------------------------------
$profileLine = "Invoke-Expression (cl init powershell | Out-String)"

if (-not (Test-Path $PROFILE)) {
    New-Item -ItemType File -Force -Path $PROFILE | Out-Null
}

$profileContent = Get-Content -Path $PROFILE -Raw -ErrorAction SilentlyContinue
if (-not $profileContent -or ($profileContent -notmatch [regex]::Escape($profileLine))) {
    Add-Content -Path $PROFILE -Value "`n# Added by cl installer`n$profileLine"
    Write-Host "  updated $PROFILE"
}

# --- Check whether the profile will actually be allowed to run -------------
# This is the one real "special permission" concern on Windows: it's
# not a filesystem/admin issue (both PATH and $PROFILE above are
# per-user, no elevation needed), it's PowerShell's execution policy.
# If it resolves to Restricted/AllSigned, PowerShell won't run the
# profile script at all, silently skipping our integration line. We
# deliberately don't change this ourselves since it's a
# security-relevant choice the user should make consciously.
$effectivePolicy = Get-ExecutionPolicy
$blocksProfile = $effectivePolicy -eq "Restricted" -or $effectivePolicy -eq "AllSigned"

Write-Host ""
if ($blocksProfile) {
    Write-Host "Warning: your PowerShell execution policy is '$effectivePolicy', which blocks profile scripts from running." -ForegroundColor Yellow
    Write-Host "The integration line was added to your profile, but it will NOT run until you allow it, e.g.:"
    Write-Host "  Set-ExecutionPolicy -Scope CurrentUser RemoteSigned"
    Write-Host "(This only affects your user account, no admin rights required, but it is a security setting you should decide on yourself.)"
} else {
    Write-Host "Done. Open a new PowerShell window and cl is ready to use."
}
