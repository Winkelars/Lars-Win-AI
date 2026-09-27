function Install-Neovim {
    [CmdletBinding()]
    param(
        [Parameter(Mandatory)]
        [hashtable]$Context
    )
    if (-not (Get-Command -Name Write-Log -ErrorAction SilentlyContinue)) { . $Context.LibPath }

    $result = New-AidResult -Component 'neovim'
    $entry = @($Context.Manifest.symlinks | Where-Object { $_.id -eq 'nvim-config' })[0]
    if ($null -eq $entry) {
        $result.Status = 'failed'
        $result.Messages += 'Manifest-Eintrag nvim-config fehlt.'
        Write-Log 'neovim: Manifest-Eintrag nvim-config fehlt.' -Level Error -LogFile $Context.LogFile
        return $result
    }

    $source = Join-Path -Path $Context.RepoRoot -ChildPath $entry.source
    $target = Resolve-ManifestPath -Path $entry.target

    if (-not (Test-Path -LiteralPath $source)) {
        $result.Status = 'skipped'
        $result.Messages += "Quelle fehlt: $source (LazyVim-Asset noch nicht vorhanden)."
        Write-Log "neovim: Quelle fehlt ($source) - übersprungen." -Level Warn -LogFile $Context.LogFile
        return $result
    }

    if ($entry.backup -and -not $Context.DryRun) {
        $existing = Get-Item -LiteralPath $target -Force -ErrorAction SilentlyContinue
        $isRealTarget = ($null -ne $existing) -and (-not $existing.LinkType)
        $backup = Backup-Path -Path $target
        if ($isRealTarget -and -not $backup) {
            $result.Status = 'failed'
            $result.Messages += "Backup fehlgeschlagen für $target - Ziel wird nicht überschrieben."
            Write-Log "neovim: Backup fehlgeschlagen ($target) - Abbruch." -Level Error -LogFile $Context.LogFile
            return $result
        }
        if ($backup) {
            $result.Messages += "Backup: $backup"
            Write-Log "neovim: vorhandenes Ziel gesichert nach $backup." -Level Info -LogFile $Context.LogFile
        }
    }

    $link = New-JunctionSafe -LinkPath $target -TargetPath $source -DryRun:$Context.DryRun -Force
    if (-not $link.Ok) {
        $result.Status = 'failed'
        $result.Messages += "Junction konnte nicht erstellt werden: $target -> $source"
        Write-Log "neovim: Junction fehlgeschlagen ($target -> $source)." -Level Error -LogFile $Context.LogFile
        return $result
    }

    $result.Changed = $link.Changed
    $result.Status = 'installed'
    if ($link.Changed) {
        Write-Log "neovim: Junction $target -> $source." -Level Success -LogFile $Context.LogFile
    } else {
        Write-Log 'neovim: Junction bereits korrekt.' -Level Info -LogFile $Context.LogFile
    }
    return $result
}
