#requires -Version 5.1
[CmdletBinding()]
param(
    [string[]]$Components,
    [switch]$Yes,
    [Alias('WhatIf')]
    [switch]$DryRun,
    [switch]$SkipDeps,
    [string]$Config
)

$ErrorActionPreference = 'Stop'

$RepoRoot = $PSScriptRoot
$libPath = Join-Path -Path $RepoRoot -ChildPath 'scripts\lib.ps1'
$logFile = Join-Path -Path $RepoRoot -ChildPath 'install.log'

. $libPath
$script:AidDryRun = [bool]$DryRun

function ConvertTo-HashtableDeep {
    param($InputObject)
    if ($null -eq $InputObject) { return $null }
    if ($InputObject -is [System.Collections.IDictionary]) {
        $hash = @{}
        foreach ($key in @($InputObject.Keys)) { $hash[$key] = ConvertTo-HashtableDeep -InputObject $InputObject[$key] }
        return $hash
    }
    if ($InputObject -is [pscustomobject]) {
        $hash = @{}
        foreach ($property in $InputObject.PSObject.Properties) { $hash[$property.Name] = ConvertTo-HashtableDeep -InputObject $property.Value }
        return $hash
    }
    if (($InputObject -is [System.Collections.IEnumerable]) -and -not ($InputObject -is [string])) {
        $list = @()
        foreach ($item in $InputObject) { $list += ,(ConvertTo-HashtableDeep -InputObject $item) }
        return ,$list
    }
    return $InputObject
}

function Get-DaemonConfig {
    param(
        [string]$RepositoryRoot,
        $ManifestObject,
        [string]$ConfigPath
    )
    $daemon = @{
        window_title       = 'AI-Assistant'
        process_name       = 'wezterm-gui.exe'
        monitor            = 2
        agent_name         = 'opencode'
        agent_kind         = 'opencode'
        agent_args         = @('--auto')
        wezterm_path       = 'C:\Program Files\WezTerm\wezterm.exe'
        wezterm_config     = (Join-Path -Path $RepositoryRoot -ChildPath 'config\wezterm\wezterm.lua')
        herdr_path         = 'herdr'
        pane_state_file    = '%APPDATA%\Lars-Win-AI\pane-state.json'
        hotkey             = @{ scan_code = 41; alt_only = $true; exclude_altgr = $true }
        animations         = $true
        startup_timeout_ms = 8000
        herdr_retry_ms     = 500
        herdr_retry_count  = 40
    }

    if ($ManifestObject.env) {
        foreach ($property in $ManifestObject.env.PSObject.Properties) {
            switch ($property.Name) {
                'AID_WINDOW_TITLE' { $daemon.window_title = [string]$property.Value }
                'AID_MONITOR' { $daemon.monitor = [int]$property.Value }
                'AID_AGENT_NAME' { $daemon.agent_name = [string]$property.Value }
                'AID_AGENT_KIND' { $daemon.agent_kind = [string]$property.Value }
                'AID_AGENT_ARGS' { $daemon.agent_args = @([string]$property.Value -split '\s+') }
            }
        }
    }

    if (-not [string]::IsNullOrWhiteSpace($ConfigPath) -and (Test-Path -LiteralPath $ConfigPath)) {
        $overrideRaw = Get-Content -LiteralPath $ConfigPath -Raw
        $override = ConvertTo-HashtableDeep -InputObject ($overrideRaw | ConvertFrom-Json)
        foreach ($key in @($override.Keys)) { $daemon[$key] = $override[$key] }
    }
    return $daemon
}

function Resolve-ComponentOrder {
    param([object[]]$Items)
    $sorted = New-Object System.Collections.ArrayList
    $sortedIds = New-Object System.Collections.ArrayList
    $remaining = @($Items | Sort-Object { $_.order })
    while ($remaining.Count -gt 0) {
        $progress = $false
        foreach ($item in @($remaining)) {
            $ready = $true
            foreach ($requirement in @($item.requires)) {
                if ($sortedIds -notcontains $requirement) { $ready = $false; break }
            }
            if ($ready) {
                [void]$sorted.Add($item)
                [void]$sortedIds.Add($item.id)
                $remaining = @($remaining | Where-Object { $_.id -ne $item.id })
                $progress = $true
                break
            }
        }
        if (-not $progress) {
            foreach ($item in $remaining) { [void]$sorted.Add($item) }
            $remaining = @()
        }
    }
    return $sorted.ToArray()
}

$startedAt = (Get-Date).ToUniversalTime().ToString('o')
$aggregate = @{
    ok         = $true
    dryRun     = [bool]$DryRun
    startedAt  = $startedAt
    finishedAt = $null
    components = @()
    env        = @{ applied = @() }
    warnings   = @()
}

Write-Log "Lars-Win-AI Installer gestartet (DryRun=$([bool]$DryRun), Yes=$([bool]$Yes))." -Level Info -LogFile $logFile

$preflightOk = $true
foreach ($tool in @('winget', 'git')) {
    if (Test-Command -Name $tool) {
        Write-Log "Preflight: $tool gefunden." -Level Info -LogFile $logFile
    } else {
        Write-Log "Preflight: $tool fehlt." -Level Error -LogFile $logFile
        $aggregate.warnings += "Preflight fehlgeschlagen: $tool fehlt."
        $preflightOk = $false
    }
}
$isAdmin = Test-Admin
$developerMode = Test-DeveloperMode
Write-Log "Preflight: Administrator=$isAdmin, DeveloperMode=$developerMode." -Level Info -LogFile $logFile
if (-not $isAdmin -and -not $developerMode) {
    Write-Log 'Preflight: keine Admin-Rechte/DevMode - File-Symlinks fallen ggf. auf Copy zurück.' -Level Warn -LogFile $logFile
}

if (-not $preflightOk) {
    $aggregate.ok = $false
    $aggregate.finishedAt = (Get-Date).ToUniversalTime().ToString('o')
    Write-Output ($aggregate | ConvertTo-Json -Depth 12)
    exit 2
}

$manifestPath = Join-Path -Path $RepoRoot -ChildPath 'manifest.json'
$manifest = $null
try {
    $manifest = Get-Content -LiteralPath $manifestPath -Raw | ConvertFrom-Json
} catch {
    Write-Log "Manifest nicht lesbar: $($_.Exception.Message)" -Level Error -LogFile $logFile
    $aggregate.ok = $false
    $aggregate.warnings += "Manifest nicht lesbar: $($_.Exception.Message)"
    $aggregate.finishedAt = (Get-Date).ToUniversalTime().ToString('o')
    Write-Output ($aggregate | ConvertTo-Json -Depth 12)
    exit 1
}

$requiredKeys = @('name', 'version', 'components', 'symlinks', 'env', 'release')
$missing = @()
foreach ($key in $requiredKeys) {
    if (-not ($manifest.PSObject.Properties.Name -contains $key)) { $missing += $key }
}
if ($missing.Count -gt 0) {
    Write-Log "Manifest unvollständig: $($missing -join ', ')" -Level Error -LogFile $logFile
    $aggregate.ok = $false
    $aggregate.warnings += "Manifest unvollständig: $($missing -join ', ')"
    $aggregate.finishedAt = (Get-Date).ToUniversalTime().ToString('o')
    Write-Output ($aggregate | ConvertTo-Json -Depth 12)
    exit 1
}

foreach ($component in @($manifest.components)) {
    $expected = 'Install-' + $component.id.Substring(0, 1).ToUpperInvariant() + $component.id.Substring(1)
    if ($component.function -ne $expected) {
        Write-Log "Manifest: function '$($component.function)' passt nicht zu id '$($component.id)' (erwartet '$expected')." -Level Error -LogFile $logFile
        $aggregate.ok = $false
        $aggregate.warnings += "Manifest-Schemafehler bei Komponente '$($component.id)'."
        $aggregate.finishedAt = (Get-Date).ToUniversalTime().ToString('o')
        Write-Output ($aggregate | ConvertTo-Json -Depth 12)
        exit 1
    }
}

$allComponents = @($manifest.components | Sort-Object { $_.order })
$selectedIds = New-Object System.Collections.Generic.List[string]

if ($Components -and $Components.Count -gt 0) {
    foreach ($id in $Components) {
        $known = @($manifest.components | Where-Object { $_.id -eq $id })
        if ($known.Count -eq 0) {
            Write-Log "Unbekannte Komponente: $id" -Level Error -LogFile $logFile
            $aggregate.ok = $false
            $aggregate.warnings += "Unbekannte Komponente: $id"
            $aggregate.finishedAt = (Get-Date).ToUniversalTime().ToString('o')
            Write-Output ($aggregate | ConvertTo-Json -Depth 12)
            exit 1
        }
    }
    $queue = New-Object System.Collections.Generic.Queue[string]
    foreach ($id in $Components) { $queue.Enqueue($id) }
    while ($queue.Count -gt 0) {
        $id = $queue.Dequeue()
        if ($selectedIds.Contains($id)) { continue }
        $selectedIds.Add($id)
        $component = @($manifest.components | Where-Object { $_.id -eq $id })[0]
        foreach ($requirement in @($component.requires)) {
            if (-not $selectedIds.Contains($requirement)) { $queue.Enqueue($requirement) }
        }
    }
} else {
    foreach ($component in $allComponents) {
        $want = [bool]$component.default
        if (-not $Yes) {
            try {
                $answer = Read-Host "Komponente '$($component.id)' installieren? [J/n]"
                if (-not [string]::IsNullOrWhiteSpace($answer)) { $want = ($answer -notmatch '^[nN]') }
            } catch {
                $want = [bool]$component.default
            }
        }
        if ($want) { $selectedIds.Add($component.id) }
    }
}

$selectedComponents = @($manifest.components | Where-Object { $selectedIds.Contains($_.id) })
$ordered = Resolve-ComponentOrder -Items $selectedComponents

$daemonConfig = Get-DaemonConfig -RepositoryRoot $RepoRoot -ManifestObject $manifest -ConfigPath $Config

$context = @{
    RepoRoot           = $RepoRoot
    Manifest           = $manifest
    Config             = $daemonConfig
    Yes                = [bool]$Yes
    DryRun             = [bool]$DryRun
    SkipDeps           = [bool]$SkipDeps
    SelectedComponents = @($selectedIds)
    LogFile            = $logFile
    LibPath            = $libPath
    Result             = $aggregate
}

foreach ($component in $ordered) {
    $scriptPath = Join-Path -Path $RepoRoot -ChildPath ($component.script -replace '/', '\')
    if (-not (Test-Path -LiteralPath $scriptPath)) {
        $aggregate.ok = $false
        $aggregate.components += [ordered]@{ component = $component.id; status = 'failed'; changed = $false; messages = @("Script fehlt: $scriptPath") }
        $aggregate.warnings += "[$($component.id)] Script fehlt: $scriptPath"
        Write-Log "Komponente $($component.id): Script fehlt ($scriptPath)." -Level Error -LogFile $logFile
        continue
    }
    Write-Log "Komponente $($component.id) startet ($($component.function))." -Level Info -LogFile $logFile
    try {
        . $scriptPath
        $runtimeResult = & $component.function -Context $context
        if ($null -eq $runtimeResult) { throw 'Komponente lieferte kein Ergebnis.' }
        $status = [string]$runtimeResult.Status
        if ($status -eq 'failed') { $aggregate.ok = $false }
        $aggregate.components += [ordered]@{
            component = [string]$runtimeResult.Component
            status    = $status
            changed   = [bool]$runtimeResult.Changed
            messages  = @($runtimeResult.Messages)
        }
        if ($status -eq 'skipped' -or $status -eq 'failed') {
            foreach ($message in @($runtimeResult.Messages)) { $aggregate.warnings += "[$($component.id)] $message" }
        }
        Write-Log "Komponente $($component.id) beendet: $status (changed=$([bool]$runtimeResult.Changed))." -Level Info -LogFile $logFile
    } catch {
        $aggregate.ok = $false
        $aggregate.components += [ordered]@{ component = $component.id; status = 'failed'; changed = $false; messages = @($_.Exception.Message) }
        $aggregate.warnings += "[$($component.id)] $($_.Exception.Message)"
        Write-Log "Komponente $($component.id) fehlgeschlagen: $($_.Exception.Message)" -Level Error -LogFile $logFile
    }
}

$aggregate.finishedAt = (Get-Date).ToUniversalTime().ToString('o')
Write-Output ($aggregate | ConvertTo-Json -Depth 12)

if (-not $aggregate.ok) { exit 1 }
exit 0
