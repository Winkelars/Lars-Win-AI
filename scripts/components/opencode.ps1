function Install-Opencode {
    [CmdletBinding()]
    param(
        [Parameter(Mandatory)]
        [hashtable]$Context
    )
    if (-not (Get-Command -Name Write-Log -ErrorAction SilentlyContinue)) { . $Context.LibPath }

    $result = New-AidResult -Component 'opencode'
    $messages = @()
    $changed = $false
    $failed = $false

    $linkIds = @('opencode-themes', 'opencode-skill')
    foreach ($linkId in $linkIds) {
        $entry = @($Context.Manifest.symlinks | Where-Object { $_.id -eq $linkId })[0]
        if ($null -eq $entry) {
            $messages += "Manifest-Eintrag $linkId fehlt."
            Write-Log "opencode: Manifest-Eintrag $linkId fehlt." -Level Warn -LogFile $Context.LogFile
            continue
        }

        $source = Join-Path -Path $Context.RepoRoot -ChildPath $entry.source
        $target = Resolve-ManifestPath -Path $entry.target
        if (-not (Test-Path -LiteralPath $source)) {
            $messages += "Quelle fehlt: $source"
            Write-Log "opencode: Quelle fehlt ($source) - $linkId übersprungen." -Level Warn -LogFile $Context.LogFile
            continue
        }

        if ($entry.backup -and -not $Context.DryRun) {
            $existing = Get-Item -LiteralPath $target -Force -ErrorAction SilentlyContinue
            $isRealTarget = ($null -ne $existing) -and (-not $existing.LinkType)
            $backup = Backup-Path -Path $target
            if ($isRealTarget -and -not $backup) {
                $failed = $true
                $messages += "Backup fehlgeschlagen für $target - Ziel wird nicht überschrieben."
                Write-Log "opencode: Backup fehlgeschlagen ($target) - $linkId übersprungen." -Level Error -LogFile $Context.LogFile
                continue
            }
            if ($backup) {
                $messages += "Backup: $backup"
                Write-Log "opencode: vorhandenes Ziel gesichert nach $backup." -Level Info -LogFile $Context.LogFile
            }
        }

        $link = New-JunctionSafe -LinkPath $target -TargetPath $source -DryRun:$Context.DryRun -Force
        if (-not $link.Ok) {
            $failed = $true
            $messages += "Junction fehlgeschlagen: $target -> $source"
            Write-Log "opencode: Junction fehlgeschlagen ($target -> $source)." -Level Error -LogFile $Context.LogFile
            continue
        }
        if ($link.Changed) { $changed = $true }
        Write-Log "opencode: $linkId -> $target." -Level Success -LogFile $Context.LogFile
    }

    $fragment = @{}
    $snippetPath = Join-Path -Path $Context.RepoRoot -ChildPath 'config/opencode/mcp.snippet.jsonc'
    if (Test-Path -LiteralPath $snippetPath) {
        try {
            $raw = Get-Content -LiteralPath $snippetPath -Raw
            $snippet = ConvertFrom-Jsonc -Text $raw
            if ($snippet -and ($snippet.PSObject.Properties.Name -contains 'mcp')) {
                $fragment['mcp'] = $snippet.mcp
            } else {
                $messages += 'mcp.snippet.jsonc ohne mcp-Objekt.'
                Write-Log 'opencode: mcp.snippet.jsonc enthält kein mcp-Objekt.' -Level Warn -LogFile $Context.LogFile
            }
        } catch {
            $messages += "mcp.snippet.jsonc nicht lesbar: $($_.Exception.Message)"
            Write-Log "opencode: mcp.snippet.jsonc nicht lesbar: $($_.Exception.Message)" -Level Warn -LogFile $Context.LogFile
        }
    } else {
        $messages += "Quelle fehlt: $snippetPath"
        Write-Log "opencode: Quelle fehlt ($snippetPath) - MCP-Merge übersprungen." -Level Warn -LogFile $Context.LogFile
    }
    $fragment['theme'] = 'ai-transparent'

    $configPath = Join-Path -Path (Join-Path -Path $env:USERPROFILE -ChildPath '.config\opencode') -ChildPath 'opencode.jsonc'
    $merge = Merge-OpencodeJsonc -Path $configPath -Fragment $fragment -DryRun:$Context.DryRun
    if (-not $merge.Ok) {
        $failed = $true
        $messages += "Merge fehlgeschlagen: $configPath"
        Write-Log "opencode: Merge fehlgeschlagen ($configPath)." -Level Error -LogFile $Context.LogFile
    } else {
        if ($merge.Changed) {
            $changed = $true
            if ($merge.Backup) { $messages += "Backup: $($merge.Backup)" }
            Write-Log "opencode: opencode.jsonc aktualisiert ($($merge.Mode))." -Level Success -LogFile $Context.LogFile
        } else {
            Write-Log 'opencode: opencode.jsonc bereits aktuell.' -Level Info -LogFile $Context.LogFile
        }
    }

    if (-not $Context.Result.ContainsKey('env') -or $null -eq $Context.Result.env) {
        $Context.Result['env'] = @{ applied = @() }
    }
    foreach ($variableName in @('EDITOR', 'VISUAL')) {
        $current = [Environment]::GetEnvironmentVariable($variableName, 'User')
        if ($current -eq 'nvim') {
            if (@($Context.Result.env.applied) -notcontains $variableName) {
                $Context.Result.env.applied += $variableName
            }
            Write-Log "opencode: $variableName bereits nvim." -Level Debug -LogFile $Context.LogFile
            continue
        }
        if ($Context.DryRun) {
            Write-Log "opencode: [DryRun] $variableName würde auf nvim gesetzt." -Level Info -LogFile $Context.LogFile
            continue
        }
        [Environment]::SetEnvironmentVariable($variableName, 'nvim', 'User')
        $changed = $true
        if (@($Context.Result.env.applied) -notcontains $variableName) {
            $Context.Result.env.applied += $variableName
        }
        Write-Log "opencode: $variableName auf nvim gesetzt." -Level Success -LogFile $Context.LogFile
    }

    $result.Changed = $changed
    $result.Status = 'installed'
    if ($failed) { $result.Status = 'failed' }
    $result.Messages = $messages
    return $result
}
