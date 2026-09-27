function Install-Herdr {
    [CmdletBinding()]
    param(
        [Parameter(Mandatory)]
        [hashtable]$Context
    )
    if (-not (Get-Command -Name Write-Log -ErrorAction SilentlyContinue)) { . $Context.LibPath }

    $result = New-AidResult -Component 'herdr'
    $entry = @($Context.Manifest.symlinks | Where-Object { $_.id -eq 'herdr-config' })[0]
    if ($null -eq $entry) {
        $result.Status = 'failed'
        $result.Messages += 'Manifest-Eintrag herdr-config fehlt.'
        Write-Log 'herdr: Manifest-Eintrag herdr-config fehlt.' -Level Error -LogFile $Context.LogFile
        return $result
    }

    $source = Join-Path -Path $Context.RepoRoot -ChildPath $entry.source
    $target = Resolve-ManifestPath -Path $entry.target

    if (-not (Test-Path -LiteralPath $source)) {
        $result.Status = 'skipped'
        $result.Messages += "Quelle fehlt: $source (Herdr-Asset noch nicht vorhanden)."
        Write-Log "herdr: Quelle fehlt ($source) - übersprungen." -Level Warn -LogFile $Context.LogFile
        return $result
    }

    if ($entry.backup -and -not $Context.DryRun) {
        $existing = Get-Item -LiteralPath $target -Force -ErrorAction SilentlyContinue
        $isRealTarget = ($null -ne $existing) -and (-not $existing.LinkType)
        $backup = Backup-Path -Path $target
        if ($isRealTarget -and -not $backup) {
            $result.Status = 'failed'
            $result.Messages += "Backup fehlgeschlagen für $target - Ziel wird nicht überschrieben."
            Write-Log "herdr: Backup fehlgeschlagen ($target) - Abbruch." -Level Error -LogFile $Context.LogFile
            return $result
        }
        if ($backup) {
            $result.Messages += "Backup: $backup"
            Write-Log "herdr: vorhandenes Ziel gesichert nach $backup." -Level Info -LogFile $Context.LogFile
        }
    }

    $link = New-FileSymlinkOrCopy -LinkPath $target -TargetPath $source -DryRun:$Context.DryRun -Force
    if (-not $link.Ok) {
        $result.Status = 'failed'
        $result.Messages += "Config konnte nicht verlinkt/kopiert werden: $target -> $source"
        Write-Log "herdr: Verlinkung fehlgeschlagen ($target -> $source)." -Level Error -LogFile $Context.LogFile
        return $result
    }

    if (Test-Command -Name 'herdr') {
        if ($Context.DryRun) {
            Write-Log 'herdr: [DryRun] herdr integration install opencode wuerde ausgeführt.' -Level Info -LogFile $Context.LogFile
        } else {
            try {
                & herdr integration install opencode 2>&1 | Out-Null
                if ($LASTEXITCODE -ne 0) {
                    $result.Messages += "herdr integration install opencode endete mit Exitcode $LASTEXITCODE."
                    Write-Log "herdr: integration install opencode Exitcode $LASTEXITCODE." -Level Warn -LogFile $Context.LogFile
                } else {
                    Write-Log 'herdr: integration install opencode ausgeführt.' -Level Success -LogFile $Context.LogFile
                }
            } catch {
                $result.Messages += "herdr integration fehlgeschlagen: $($_.Exception.Message)"
                Write-Log "herdr: integration fehlgeschlagen: $($_.Exception.Message)" -Level Warn -LogFile $Context.LogFile
            }
        }
    } else {
        $result.Messages += 'herdr nicht gefunden - integration install opencode übersprungen.'
        Write-Log 'herdr: herdr nicht gefunden - Integration übersprungen.' -Level Warn -LogFile $Context.LogFile
    }

    $result.Changed = $link.Changed
    $result.Status = 'installed'
    $mode = $link.Mode
    if ($link.Changed) {
        Write-Log "herdr: Config als $mode platziert ($target -> $source)." -Level Success -LogFile $Context.LogFile
    } else {
        Write-Log "herdr: Config bereits korrekt ($mode)." -Level Info -LogFile $Context.LogFile
    }
    return $result
}
