function Install-Font {
    [CmdletBinding()]
    param(
        [Parameter(Mandatory)]
        [hashtable]$Context
    )
    if (-not (Get-Command -Name Write-Log -ErrorAction SilentlyContinue)) { . $Context.LibPath }

    $result = New-AidResult -Component 'font'

    # Die eigentliche Arbeit (Download + per-User-Registrierung) liegt in
    # scripts/lib.ps1, damit der Uninstaller denselben Code nutzen kann.
    $font = Install-AidFont -LogFile $Context.LogFile -DryRun:$Context.DryRun

    $result.Messages = @($font.Messages)
    if (-not $font.Ok) {
        $result.Status = 'failed'
        return $result
    }
    $result.Status = 'installed'
    $result.Changed = [bool]$font.Changed
    return $result
}
