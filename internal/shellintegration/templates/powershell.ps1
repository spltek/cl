# cl shell integration for PowerShell.
# Add to your profile (see `$PROFILE`):
#   Invoke-Expression (cl init powershell | Out-String)
#
# This defines a `cl` function that shadows the `cl` binary on PATH.
# Informational commands are passed straight through so their output
# prints normally instead of being captured. Everything else opens
# the interactive picker, where adding/editing/renaming/deleting
# commands happens via ctrl+a/ctrl+e/ctrl+r/ctrl+d. Interactive
# picker selections try
# to use PSReadLine's Insert() to pre-fill the next line (same
# mechanism used by modules like PSFzf); if that is not available, it
# falls back to an explicit confirmation prompt before running the
# command.
function cl {
    $realCl = (Get-Command cl -CommandType Application | Select-Object -First 1 -ExpandProperty Source)

    $passthrough = @('init', '-v', '--version', '-h', '--help', 'help')
    if ($args.Count -gt 0 -and ($passthrough -contains $args[0])) {
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
