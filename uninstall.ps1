#requires -Version 5.1
[CmdletBinding()]
param(
    [switch]$RestoreBackups,
    [Alias('WhatIf')]
    [switch]$DryRun
)

$ErrorActionPreference = 'Stop'

$RepoRoot = $PSScriptRoot
$libPath = Join-Path -Path $RepoRoot -ChildPath 'scripts\lib.ps1'
$logFile = Join-Path -Path $RepoRoot -ChildPath 'uninstall.log'

. $libPath
$script:AidDryRun = [bool]$DryRun

$startedAt = (Get-Date).ToUniversalTime().ToString('o')
$result = @{
    ok           = $true
    dryRun       = [bool]$DryRun
    startedAt    = $startedAt
    finishedAt   = $null
    removedLinks = @()
    restored     = @()
    removedTask  = $false
    removedFont  = $false
    warnings     = @()
}

Write-Log "Lars-Win-AI Uninstaller gestartet (DryRun=$([bool]$DryRun), RestoreBackups=$([bool]$RestoreBackups))." -Level Info -LogFile $logFile

$taskName = 'aid'
$taskFolder = '\Lars-Win-AI\'
try {
    $existingTask = Get-ScheduledTask -TaskName $taskName -TaskPath $taskFolder -ErrorAction SilentlyContinue
} catch {
    $existingTask = $null
}
if ($null -ne $existingTask) {
    if ($DryRun) {
        Write-Log "uninstall: [DryRun] Task $taskFolder$taskName würde entfernt." -Level Info -LogFile $logFile
    } else {
        Unregister-ScheduledTask -TaskName $taskName -TaskPath $taskFolder -Confirm:$false -ErrorAction Stop
        $result.removedTask = $true
        Write-Log "uninstall: Task $taskFolder$taskName entfernt." -Level Success -LogFile $logFile
    }
} else {
    Write-Log "uninstall: Task $taskFolder$taskName nicht vorhanden." -Level Info -LogFile $logFile
}

$manifestPath = Join-Path -Path $RepoRoot -ChildPath 'manifest.json'
$symlinks = @()
if (Test-Path -LiteralPath $manifestPath) {
    try {
        $manifest = Get-Content -LiteralPath $manifestPath -Raw | ConvertFrom-Json
        if ($manifest.symlinks) { $symlinks = @($manifest.symlinks) }
    } catch {
        $result.ok = $false
        $result.warnings += "Manifest nicht lesbar: $($_.Exception.Message)"
        Write-Log "uninstall: Manifest nicht lesbar: $($_.Exception.Message)" -Level Error -LogFile $logFile
    }
} else {
    $result.warnings += "Manifest fehlt: $manifestPath"
    Write-Log "uninstall: Manifest fehlt ($manifestPath)." -Level Warn -LogFile $logFile
}

foreach ($entry in $symlinks) {
    $target = Resolve-ManifestPath -Path $entry.target
    if ([string]::IsNullOrWhiteSpace($target)) { continue }

    $item = Get-Item -LiteralPath $target -Force -ErrorAction SilentlyContinue
    $isLink = $false
    if ($null -ne $item) { $isLink = [bool]($item.Attributes -band [System.IO.FileAttributes]::ReparsePoint) }

    if ($isLink) {
        if ($DryRun) {
            Write-Log "uninstall: [DryRun] Link würde entfernt: $target" -Level Info -LogFile $logFile
        } else {
            Remove-LinkSafe -Path $target
            Write-Log "uninstall: Link entfernt: $target" -Level Success -LogFile $logFile
        }
        $result.removedLinks += $target
    } else {
        Write-Log "uninstall: kein Link vorhanden: $target" -Level Debug -LogFile $logFile
    }

    if ($RestoreBackups) {
        $backup = $target + '.bak'
        if (Test-Path -LiteralPath $backup) {
            $current = Get-Item -LiteralPath $target -Force -ErrorAction SilentlyContinue
            $currentReal = ($null -ne $current) -and (-not [bool]($current.Attributes -band [System.IO.FileAttributes]::ReparsePoint))
            if ($currentReal) {
                $result.warnings += "Backup nicht zurückgespielt (Ziel ist echte Datei): $target"
                Write-Log "uninstall: Backup übersprungen, Ziel ist echte Datei: $target" -Level Warn -LogFile $logFile
            } elseif ($DryRun) {
                Write-Log "uninstall: [DryRun] Backup würde zurückgespielt: $backup -> $target" -Level Info -LogFile $logFile
            } else {
                Move-Item -LiteralPath $backup -Destination $target -Force -ErrorAction Stop
                $result.restored += $target
                Write-Log "uninstall: Backup zurückgespielt: $backup -> $target" -Level Success -LogFile $logFile
            }
        }
    }
}

$opencodeConfig = Join-Path -Path (Join-Path -Path $env:USERPROFILE -ChildPath '.config\opencode') -ChildPath 'opencode.jsonc'
if ($RestoreBackups) {
    $opencodeBackup = $opencodeConfig + '.bak'
    if (Test-Path -LiteralPath $opencodeBackup) {
        if ($DryRun) {
            Write-Log "uninstall: [DryRun] opencode.jsonc-Backup würde zurückgespielt: $opencodeBackup" -Level Info -LogFile $logFile
        } else {
            Move-Item -LiteralPath $opencodeBackup -Destination $opencodeConfig -Force -ErrorAction Stop
            $result.restored += $opencodeConfig
            Write-Log "uninstall: opencode.jsonc aus Backup wiederhergestellt." -Level Success -LogFile $logFile
        }
    }
}

# Nerd Font entfernen (pro Benutzer installiert, kein winget-Artefakt).
$fontRemoved = Uninstall-AidFont -DryRun:$DryRun
$result.removedFont = [bool]$fontRemoved
if ($fontRemoved) {
    if ($DryRun) {
        Write-Log 'uninstall: [DryRun] CaskaydiaCove Nerd Font würde entfernt.' -Level Info -LogFile $logFile
    } else {
        Write-Log 'uninstall: CaskaydiaCove Nerd Font entfernt.' -Level Success -LogFile $logFile
    }
} else {
    Write-Log 'uninstall: CaskaydiaCove Nerd Font nicht vorhanden.' -Level Info -LogFile $logFile
}

$result.finishedAt = (Get-Date).ToUniversalTime().ToString('o')
Write-Output ($result | ConvertTo-Json -Depth 12)

if (-not $result.ok) { exit 1 }
exit 0
