function Install-Deps {
    [Diagnostics.CodeAnalysis.SuppressMessageAttribute('PSUseSingularNouns', '', Justification = 'Function name frozen by CONTRACTS.md (WS-I).')]
    [CmdletBinding()]
    param(
        [Parameter(Mandatory)]
        [hashtable]$Context
    )
    if (-not (Get-Command -Name Write-Log -ErrorAction SilentlyContinue)) { . $Context.LibPath }

    $result = New-AidResult -Component 'deps'

    if ($Context.SkipDeps) {
        Write-Log 'deps: -SkipDeps aktiv, winget wird übersprungen.' -Level Warn -LogFile $Context.LogFile
        $result.Status = 'skipped'
        $result.Messages += 'winget übersprungen (-SkipDeps).'
        return $result
    }

    $messages = @()
    $changed = $false

    $font = Invoke-Winget -Id 'ryanoasis.CaskaydiaCove' -Name 'CaskaydiaCove Nerd Font' -DryRun:$Context.DryRun -SkipDeps:$Context.SkipDeps
    if (-not $font.Ok) {
        $result.Status = 'failed'
        $result.Messages += 'winget nicht verfügbar oder Installation fehlgeschlagen (ryanoasis.CaskaydiaCove).'
        Write-Log 'deps: winget-Aufruf fehlgeschlagen.' -Level Error -LogFile $Context.LogFile
        return $result
    }

    if ($Context.DryRun) {
        if ($font.Changed) {
            Write-Log 'deps: [DryRun] ryanoasis.CaskaydiaCove würde installiert.' -Level Info -LogFile $Context.LogFile
        } else {
            Write-Log 'deps: [DryRun] ryanoasis.CaskaydiaCove bereits vorhanden.' -Level Info -LogFile $Context.LogFile
        }
    } elseif ($font.Changed) {
        Write-Log 'deps: ryanoasis.CaskaydiaCove installiert.' -Level Success -LogFile $Context.LogFile
    } elseif ($font.Installed) {
        Write-Log 'deps: ryanoasis.CaskaydiaCove bereits vorhanden.' -Level Info -LogFile $Context.LogFile
    }
    $changed = $font.Changed

    foreach ($tool in @('alacritty', 'herdr', 'nvim')) {
        if (Test-Command -Name $tool) {
            Write-Log "deps: $tool gefunden." -Level Debug -LogFile $Context.LogFile
        } else {
            $messages += "$tool nicht gefunden - bitte manuell installieren (keine Auto-Installation)."
            Write-Log "deps: $tool nicht gefunden (nur Hinweis)." -Level Warn -LogFile $Context.LogFile
        }
    }

    $result.Changed = $changed
    $result.Status = 'installed'
    $result.Messages = $messages
    return $result
}
