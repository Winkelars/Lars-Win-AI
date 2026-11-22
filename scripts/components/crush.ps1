function Install-Crush {
    [CmdletBinding()]
    param(
        [Parameter(Mandatory)]
        [hashtable]$Context
    )
    if (-not (Get-Command -Name Write-Log -ErrorAction SilentlyContinue)) { . $Context.LibPath }

    $result = New-AidResult -Component 'crush'

    # Crush erkennt/zeigt Herdr selbst als Agent-Pane (kein Reporter nötig).
    # Diese Komponente installiert nur den herdr-Skill als Junction.
    $entry = @($Context.Manifest.symlinks | Where-Object { $_.id -eq 'crush-skill' })[0]
    if ($null -eq $entry) {
        $result.Status = 'failed'
        $result.Messages += 'Manifest-Eintrag crush-skill fehlt.'
        Write-Log 'crush: Manifest-Eintrag crush-skill fehlt.' -Level Error -LogFile $Context.LogFile
        return $result
    }

    $source = Join-Path -Path $Context.RepoRoot -ChildPath $entry.source
    $target = Resolve-ManifestPath -Path $entry.target

    if (-not (Test-Path -LiteralPath $source)) {
        $result.Status = 'skipped'
        $result.Messages += "Quelle fehlt: $source (Crush-Skill-Asset noch nicht vorhanden)."
        Write-Log "crush: Quelle fehlt ($source) - übersprungen." -Level Warn -LogFile $Context.LogFile
        return $result
    }

    if ($entry.backup -and -not $Context.DryRun) {
        $existing = Get-Item -LiteralPath $target -Force -ErrorAction SilentlyContinue
        $isRealTarget = ($null -ne $existing) -and (-not $existing.LinkType)
        $backup = Backup-Path -Path $target
        if ($isRealTarget -and -not $backup) {
            $result.Status = 'failed'
            $result.Messages += "Backup fehlgeschlagen für $target - Ziel wird nicht überschrieben."
            Write-Log "crush: Backup fehlgeschlagen ($target) - Abbruch." -Level Error -LogFile $Context.LogFile
            return $result
        }
        if ($backup) {
            $result.Messages += "Backup: $backup"
            Write-Log "crush: vorhandenes Ziel gesichert nach $backup." -Level Info -LogFile $Context.LogFile
        }
    }

    $link = New-JunctionSafe -LinkPath $target -TargetPath $source -DryRun:$Context.DryRun -Force
    if (-not $link.Ok) {
        $result.Status = 'failed'
        $result.Messages += "Junction fehlgeschlagen: $target -> $source"
        Write-Log "crush: Junction fehlgeschlagen ($target -> $source)." -Level Error -LogFile $Context.LogFile
        return $result
    }

    $result.Changed = $link.Changed
    $result.Status = 'installed'
    if ($link.Changed) {
        Write-Log "crush: herdr-Skill als $($link.Mode) platziert ($target -> $source)." -Level Success -LogFile $Context.LogFile
    } else {
        Write-Log "crush: herdr-Skill bereits korrekt ($($link.Mode))." -Level Info -LogFile $Context.LogFile
    }
    return $result
}
