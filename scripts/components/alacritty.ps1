function Install-Alacritty {
    [CmdletBinding()]
    param(
        [Parameter(Mandatory)]
        [hashtable]$Context
    )
    if (-not (Get-Command -Name Write-Log -ErrorAction SilentlyContinue)) { . $Context.LibPath }

    $result = New-AidResult -Component 'alacritty'
    $entry = @($Context.Manifest.symlinks | Where-Object { $_.id -eq 'alacritty-config' })[0]
    if ($null -eq $entry) {
        $result.Status = 'failed'
        $result.Messages += 'Manifest-Eintrag alacritty-config fehlt.'
        Write-Log 'alacritty: Manifest-Eintrag alacritty-config fehlt.' -Level Error -LogFile $Context.LogFile
        return $result
    }

    $source = Join-Path -Path $Context.RepoRoot -ChildPath $entry.source
    $target = Resolve-ManifestPath -Path $entry.target

    if (-not (Test-Path -LiteralPath $source)) {
        $result.Status = 'skipped'
        $result.Messages += "Quelle fehlt: $source (Alacritty-Asset noch nicht vorhanden)."
        Write-Log "alacritty: Quelle fehlt ($source) - übersprungen." -Level Warn -LogFile $Context.LogFile
        return $result
    }

    if ($entry.backup -and -not $Context.DryRun) {
        $existing = Get-Item -LiteralPath $target -Force -ErrorAction SilentlyContinue
        $isRealTarget = ($null -ne $existing) -and (-not $existing.LinkType)
        $backup = Backup-Path -Path $target
        if ($isRealTarget -and -not $backup) {
            $result.Status = 'failed'
            $result.Messages += "Backup fehlgeschlagen für $target - Ziel wird nicht überschrieben."
            Write-Log "alacritty: Backup fehlgeschlagen ($target) - Abbruch." -Level Error -LogFile $Context.LogFile
            return $result
        }
        if ($backup) {
            $result.Messages += "Backup: $backup"
            Write-Log "alacritty: vorhandenes Ziel gesichert nach $backup." -Level Info -LogFile $Context.LogFile
        }
    }

    $link = New-JunctionSafe -LinkPath $target -TargetPath $source -DryRun:$Context.DryRun -Force
    if (-not $link.Ok) {
        $result.Status = 'failed'
        $result.Messages += "Junction konnte nicht erstellt werden: $target -> $source"
        Write-Log "alacritty: Junction fehlgeschlagen ($target -> $source)." -Level Error -LogFile $Context.LogFile
        return $result
    }

    $result.Changed = $link.Changed
    $result.Status = 'installed'
    if ($link.Changed) {
        Write-Log "alacritty: Junction $target -> $source." -Level Success -LogFile $Context.LogFile
    } else {
        Write-Log 'alacritty: Junction bereits korrekt.' -Level Info -LogFile $Context.LogFile
    }
    return $result
}
