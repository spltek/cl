# cl shell integration for PowerShell.
# Add to your profile (see `$PROFILE`):
#   Invoke-Expression (cl init powershell | Out-String)
#
# This defines a `cl` function that shadows the `cl` binary on PATH.
# Management commands (-add, -remove, init) are passed straight
# through. Interactive selections try to use PSReadLine's Insert()
# to pre-fill the next line (same mechanism used by modules like
# PSFzf); if that is not available, it falls back to an explicit
# confirmation prompt before running the command.
function cl {
    $realCl = (Get-Command cl -CommandType Application | Select-Object -First 1 -ExpandProperty Source)

    if ($args.Count -gt 0 -and ($args[0] -eq '-add' -or $args[0] -eq '-remove' -or $args[0] -eq 'init')) {
        & $realCl @args
        return
    }

    $out = & $realCl @args
    if ($out) {
        $inserted = $false
        try {
            [Microsoft.PowerShell.PSConsoleReadLine]::Insert($out)
            $inserted = $true
        } catch {
            $inserted = $false
        }

        if (-not $inserted) {
            $confirm = Read-Host "Run: $out ? [Y/n]"
            if ($confirm -eq '' -or $confirm -eq 'y' -or $confirm -eq 'Y') {
                Invoke-Expression $out
            }
        }
    }
}
