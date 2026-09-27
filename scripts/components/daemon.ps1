function Install-Daemon {
    [CmdletBinding()]
    param(
        [Parameter(Mandatory)]
        [hashtable]$Context
    )
    if (-not (Get-Command -Name Write-Log -ErrorAction SilentlyContinue)) { . $Context.LibPath }

    $result = New-AidResult -Component 'daemon'
    $messages = @()
    $changed = $false

    $localDir = Join-Path -Path $env:LOCALAPPDATA -ChildPath 'Lars-Win-AI'
    $appDir = Join-Path -Path $env:APPDATA -ChildPath 'Lars-Win-AI'
    $exePath = Join-Path -Path $localDir -ChildPath 'aid.exe'

    $release = $Context.Manifest.release
    if ($null -eq $release -or [string]::IsNullOrWhiteSpace($release.repo) -or [string]::IsNullOrWhiteSpace($release.asset)) {
        $result.Status = 'failed'
        $result.Messages += 'manifest.release unvollständig (repo/asset).'
        Write-Log 'daemon: manifest.release unvollständig.' -Level Error -LogFile $Context.LogFile
        return $result
    }

    $valid = $false
    if (Test-Path -LiteralPath $exePath) {
        try {
            $stream = [System.IO.File]::OpenRead($exePath)
            try {
                $b1 = $stream.ReadByte()
                $b2 = $stream.ReadByte()
            } finally {
                $stream.Close()
            }
            $valid = ($b1 -eq 0x4D -and $b2 -eq 0x5A)
        } catch {
            $valid = $false
        }
    }

    if ($valid) {
        Write-Log "daemon: aid.exe bereits vorhanden ($exePath)." -Level Info -LogFile $Context.LogFile
    } elseif ($Context.DryRun) {
        $url = "https://github.com/$($release.repo)/releases/latest/download/$($release.asset)"
        $messages += "[DryRun] aid.exe würde von $url geladen."
        $changed = $true
        Write-Log "daemon: [DryRun] aid.exe würde nach $exePath geladen." -Level Info -LogFile $Context.LogFile
    } else {
        $url = "https://github.com/$($release.repo)/releases/latest/download/$($release.asset)"
        try {
            New-Item -ItemType Directory -Path $localDir -Force | Out-Null
            Write-Log "daemon: lade $url" -Level Info -LogFile $Context.LogFile
            Invoke-WebRequest -Uri $url -OutFile $exePath -UseBasicParsing -ErrorAction Stop
            $stream = [System.IO.File]::OpenRead($exePath)
            try {
                $b1 = $stream.ReadByte()
                $b2 = $stream.ReadByte()
            } finally {
                $stream.Close()
            }
            if (-not ($b1 -eq 0x4D -and $b2 -eq 0x5A)) {
                throw 'Heruntergeladene Datei ist keine gültige PE-Datei.'
            }
            $changed = $true
            Write-Log "daemon: aid.exe geladen nach $exePath." -Level Success -LogFile $Context.LogFile
        } catch {
            $result.Status = 'failed'
            $result.Messages += "aid.exe-Download fehlgeschlagen: $($_.Exception.Message)"
            Write-Log "daemon: Download fehlgeschlagen: $($_.Exception.Message)" -Level Error -LogFile $Context.LogFile
            if (Test-Path -LiteralPath $exePath) { Remove-Item -LiteralPath $exePath -Force -ErrorAction SilentlyContinue }
            return $result
        }
    }

    $configPath = Join-Path -Path $appDir -ChildPath 'config.json'
    if (Test-Path -LiteralPath $configPath) {
        Write-Log "daemon: config.json vorhanden ($configPath)." -Level Info -LogFile $Context.LogFile
    } elseif ($Context.DryRun) {
        $messages += "[DryRun] config.json würde angelegt: $configPath"
        $changed = $true
        Write-Log "daemon: [DryRun] config.json würde angelegt." -Level Info -LogFile $Context.LogFile
    } else {
        New-Item -ItemType Directory -Path $appDir -Force | Out-Null
        $json = ConvertTo-Json -InputObject $Context.Config -Depth 12
        Set-Content -LiteralPath $configPath -Value $json -Encoding UTF8
        $changed = $true
        Write-Log "daemon: config.json angelegt ($configPath)." -Level Success -LogFile $Context.LogFile
    }

    $statePath = Join-Path -Path $appDir -ChildPath 'pane-state.json'
    if (Test-Path -LiteralPath $statePath) {
        Write-Log 'daemon: pane-state.json vorhanden.' -Level Info -LogFile $Context.LogFile
    } elseif ($Context.DryRun) {
        Write-Log 'daemon: [DryRun] pane-state.json würde angelegt.' -Level Info -LogFile $Context.LogFile
    } else {
        New-Item -ItemType Directory -Path $appDir -Force | Out-Null
        Set-Content -LiteralPath $statePath -Value '{}' -Encoding UTF8
        $changed = $true
        Write-Log 'daemon: pane-state.json angelegt.' -Level Success -LogFile $Context.LogFile
    }

    $envMapping = @(
        @{ Key = 'window_title';    Name = 'AID_WINDOW_TITLE' },
        @{ Key = 'monitor';         Name = 'AID_MONITOR' },
        @{ Key = 'agent_name';      Name = 'AID_AGENT_NAME' },
        @{ Key = 'agent_kind';      Name = 'AID_AGENT_KIND' },
        @{ Key = 'agent_args';      Name = 'AID_AGENT_ARGS' },
        @{ Key = 'alacritty_path';  Name = 'AID_ALACRITTY_PATH' },
        @{ Key = 'herdr_path';      Name = 'AID_HERDR_PATH' }
    )
    if (-not $Context.Result.ContainsKey('env') -or $null -eq $Context.Result.env) {
        $Context.Result['env'] = @{ applied = @() }
    }
    foreach ($mapping in $envMapping) {
        $raw = $Context.Config[$mapping.Key]
        if ($null -eq $raw) { continue }
        if ($raw -is [array]) { $value = ($raw -join ' ') } else { $value = [string]$raw }
        $current = [Environment]::GetEnvironmentVariable($mapping.Name, 'User')
        if ($current -eq $value) {
            if (@($Context.Result.env.applied) -notcontains $mapping.Name) { $Context.Result.env.applied += $mapping.Name }
            continue
        }
        if ($Context.DryRun) {
            Write-Log "daemon: [DryRun] $($mapping.Name) würde auf '$value' gesetzt." -Level Info -LogFile $Context.LogFile
            continue
        }
        [Environment]::SetEnvironmentVariable($mapping.Name, $value, 'User')
        $changed = $true
        if (@($Context.Result.env.applied) -notcontains $mapping.Name) { $Context.Result.env.applied += $mapping.Name }
        Write-Log "daemon: $($mapping.Name)=$value gesetzt." -Level Success -LogFile $Context.LogFile
    }

    $taskName = 'aid'
    $taskFolder = '\Lars-Win-AI\'
    $existingTask = Get-ScheduledTask -TaskName $taskName -TaskPath $taskFolder -ErrorAction SilentlyContinue
    if ($null -ne $existingTask) {
        Write-Log "daemon: Task $taskFolder$taskName bereits vorhanden." -Level Info -LogFile $Context.LogFile
    } elseif ($Context.DryRun) {
        $messages += "[DryRun] Scheduled Task $taskFolder$taskName würde registriert."
        $changed = $true
        Write-Log "daemon: [DryRun] Task $taskFolder$taskName würde registriert." -Level Info -LogFile $Context.LogFile
    } else {
        try {
            $action = New-ScheduledTaskAction -Execute $exePath -Argument 'run'
            $trigger = New-ScheduledTaskTrigger -AtLogOn
            $settings = New-ScheduledTaskSettingsSet -AllowStartIfOnBatteries -DontStopIfGoingOnBatteries -ExecutionTimeLimit ([TimeSpan]::Zero) -RestartCount 3 -RestartInterval (New-TimeSpan -Minutes 1)
            $principal = New-ScheduledTaskPrincipal -UserId "$env:USERDOMAIN\$env:USERNAME" -LogonType Interactive -RunLevel Limited
            Register-ScheduledTask -TaskName $taskName -TaskPath $taskFolder -Action $action -Trigger $trigger -Settings $settings -Principal $principal -Force | Out-Null
            $changed = $true
            Write-Log "daemon: Task $taskFolder$taskName registriert." -Level Success -LogFile $Context.LogFile
        } catch {
            $result.Status = 'failed'
            $result.Messages += "Task-Registrierung fehlgeschlagen: $($_.Exception.Message)"
            Write-Log "daemon: Task-Registrierung fehlgeschlagen: $($_.Exception.Message)" -Level Error -LogFile $Context.LogFile
            return $result
        }
    }

    $result.Changed = $changed
    $result.Status = 'installed'
    $result.Messages = $messages
    return $result
}
