function Install-Wezterm {
    [CmdletBinding()]
    param(
        [Parameter(Mandatory)]
        [hashtable]$Context
    )
    if (-not (Get-Command -Name Write-Log -ErrorAction SilentlyContinue)) { . $Context.LibPath }

    $result = New-AidResult -Component 'wezterm'
    $entry = @($Context.Manifest.symlinks | Where-Object { $_.id -eq 'wezterm-config' })[0]
    if ($null -eq $entry) {
        $result.Status = 'failed'
        $result.Messages += 'Manifest-Eintrag wezterm-config fehlt.'
        Write-Log 'wezterm: Manifest-Eintrag wezterm-config fehlt.' -Level Error -LogFile $Context.LogFile
        return $result
    }

    $source = Join-Path -Path $Context.RepoRoot -ChildPath $entry.source
    $target = Resolve-ManifestPath -Path $entry.target

    if (-not (Test-Path -LiteralPath $source)) {
        $result.Status = 'skipped'
        $result.Messages += "Quelle fehlt: $source (WezTerm-Asset noch nicht vorhanden)."
        Write-Log "wezterm: Quelle fehlt ($source) - übersprungen." -Level Warn -LogFile $Context.LogFile
        return $result
    }

    if ($entry.backup -and -not $Context.DryRun) {
        $existing = Get-Item -LiteralPath $target -Force -ErrorAction SilentlyContinue
        $isRealTarget = ($null -ne $existing) -and (-not $existing.LinkType)
        $backup = Backup-Path -Path $target
        if ($isRealTarget -and -not $backup) {
            $result.Status = 'failed'
            $result.Messages += "Backup fehlgeschlagen für $target - Ziel wird nicht überschrieben."
            Write-Log "wezterm: Backup fehlgeschlagen ($target) - Abbruch." -Level Error -LogFile $Context.LogFile
            return $result
        }
        if ($backup) {
            $result.Messages += "Backup: $backup"
            Write-Log "wezterm: vorhandenes Ziel gesichert nach $backup." -Level Info -LogFile $Context.LogFile
        }
    }

    $link = New-FileSymlinkOrCopy -LinkPath $target -TargetPath $source -DryRun:$Context.DryRun -Force
    if (-not $link.Ok) {
        $result.Status = 'failed'
        $result.Messages += "Config konnte nicht verlinkt/kopiert werden: $target -> $source"
        Write-Log "wezterm: Verlinkung fehlgeschlagen ($target -> $source)." -Level Error -LogFile $Context.LogFile
        return $result
    }

    # WezTerm verweigert den Start, wenn sowohl ~/.wezterm.lua als auch
    # ~/.config/wezterm/wezterm.lua existieren. Ein evtl. echtes Legacy-File
    # wird gesichert und entfernt, damit unsere Config eindeutig greift.
    $legacy = Join-Path -Path $env:USERPROFILE -ChildPath '.config\wezterm\wezterm.lua'
    if (Test-Path -LiteralPath $legacy) {
        $legacyItem = Get-Item -LiteralPath $legacy -Force
        if (-not $legacyItem.LinkType) {
            if ($Context.DryRun) {
                Write-Log 'wezterm: [DryRun] ~/.config/wezterm/wezterm.lua würde gesichert und entfernt.' -Level Info -LogFile $Context.LogFile
            } else {
                $legacyBackup = "$legacy.bak"
                Move-Item -LiteralPath $legacy -Destination $legacyBackup -Force
                $result.Messages += "Legacy-Config gesichert: $legacyBackup"
                Write-Log "wezterm: Legacy-Config gesichert nach $legacyBackup." -Level Warn -LogFile $Context.LogFile
            }
        }
    }

    $result.Changed = $link.Changed
    $result.Status = 'installed'
    $mode = $link.Mode
    if ($link.Changed) {
        Write-Log "wezterm: Config als $mode platziert ($target -> $source)." -Level Success -LogFile $Context.LogFile
    } else {
        Write-Log "wezterm: Config bereits korrekt ($mode)." -Level Info -LogFile $Context.LogFile
    }
    return $result
}
