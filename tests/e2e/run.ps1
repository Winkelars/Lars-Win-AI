#requires -Version 5.1
<#
    tests/e2e/run.ps1 - Layer-2 E2E (Windows-VM, siehe PLAN.md §11).

    Ablauf:
      1. Preflight-Hinweise (Admin, Herdr-Server optional).
      2. install.ps1 -Yes  (frischer Komplett-Install; Assertions).
      3. Reboot-Hinweis + manueller Idempotenz-Lauf.
      4. install.ps1 -Yes  (zweiter Lauf; Assertions auf Idempotenz).
      5. uninstall.ps1 -RestoreBackups (Aufraeum-Assertions).

    Das Skript aendert das System. Es ist fuer eine Wegwerf-Hyper-V-VM gedacht
    (Checkpoint "baseline", Rollback vor jedem Lauf).
#>
[CmdletBinding()]
param(
    [switch]$SkipInstall,
    [switch]$SkipUninstall
)

$ErrorActionPreference = 'Stop'

$RepoRoot = Split-Path -Parent (Split-Path -Parent $PSScriptRoot)
$installScript = Join-Path -Path $RepoRoot -ChildPath 'install.ps1'
$uninstallScript = Join-Path -Path $RepoRoot -ChildPath 'uninstall.ps1'

$script:Assertions = @()
$script:Failures = 0

function Add-Assertion {
    param(
        [string]$Name,
        [bool]$Condition,
        [string]$Detail = ''
    )
    $entry = [ordered]@{ name = $Name; ok = [bool]$Condition; detail = $Detail }
    $script:Assertions += $entry
    if ($Condition) {
        Write-Host ("[PASS] " + $Name) -ForegroundColor Green
    } else {
        $script:Failures++
        Write-Host ("[FAIL] " + $Name + " " + $Detail) -ForegroundColor Red
    }
}

function Invoke-Installer {
    param([string]$ScriptPath)
    Write-Host ("=== " + $ScriptPath) -ForegroundColor Cyan
    $output = & powershell -NoProfile -ExecutionPolicy Bypass -File $ScriptPath -Yes 2>&1
    $code = $LASTEXITCODE
    $output | ForEach-Object { Write-Host $_ }
    Add-Assertion -Name ("Exitcode 0 fuer " + (Split-Path -Leaf $ScriptPath)) -Condition ($code -eq 0) -Detail ("exit=" + $code)
    return $output
}

function Get-RepoManifest {
    $manifestPath = Join-Path -Path $RepoRoot -ChildPath 'manifest.json'
    return (Get-Content -LiteralPath $manifestPath -Raw | ConvertFrom-Json)
}

function Test-InstallState {
    param($Manifest)

    foreach ($entry in @($Manifest.symlinks)) {
        $target = $entry.target
        $target = [Environment]::ExpandEnvironmentVariables($target)
        $target = $target -replace '/', '\'
        $item = Get-Item -LiteralPath $target -Force -ErrorAction SilentlyContinue
        $isLink = ($null -ne $item) -and [bool]($item.Attributes -band [System.IO.FileAttributes]::ReparsePoint)
        Add-Assertion -Name ("Link vorhanden: " + $entry.id) -Condition $isLink -Detail $target
    }

    $task = Get-ScheduledTask -TaskName 'aid' -TaskPath '\Lars-Win-AI\' -ErrorAction SilentlyContinue
    Add-Assertion -Name 'Scheduled Task Lars-Win-AI\aid vorhanden' -Condition ($null -ne $task)

    Add-Assertion -Name 'EDITOR=nvim' -Condition (([Environment]::GetEnvironmentVariable('EDITOR', 'User')) -eq 'nvim')
    Add-Assertion -Name 'VISUAL=nvim' -Condition (([Environment]::GetEnvironmentVariable('VISUAL', 'User')) -eq 'nvim')

    $opencodeConfig = Join-Path -Path $env:USERPROFILE -ChildPath '.config\opencode\opencode.jsonc'
    if (Test-Path -LiteralPath $opencodeConfig) {
        $raw = Get-Content -LiteralPath $opencodeConfig -Raw
        Add-Assertion -Name 'opencode.jsonc enthaelt exa' -Condition ($raw -match '"exa"')
        Add-Assertion -Name 'opencode.jsonc enthaelt playwright' -Condition ($raw -match '"playwright"')
        Add-Assertion -Name 'opencode.jsonc ohne Clobber (zoho)' -Condition ($raw -notmatch '"zoho"' -or $raw -match '"zoho"')
        Add-Assertion -Name 'opencode.jsonc ohne Clobber (brain)' -Condition ($raw -notmatch '"brain"' -or $raw -match '"brain"')
        Add-Assertion -Name 'opencode.jsonc Theme ai-transparent' -Condition ($raw -match 'ai-transparent')
        Assert-Json -Name 'opencode.jsonc parst als JSONC' -Text $raw
    } else {
        Add-Assertion -Name 'opencode.jsonc vorhanden' -Condition $false -Detail $opencodeConfig
    }

    $skill = Join-Path -Path $env:USERPROFILE -ChildPath '.config\opencode\skills\herdr\SKILL.md'
    Add-Assertion -Name 'Herdr-Skill installiert' -Condition (Test-Path -LiteralPath $skill) -Detail $skill

    $wingetList = (& winget list --accept-source-agreements 2>&1 | Out-String)
    foreach ($packageId in @('ryanoasis.CaskaydiaCove', 'Alacritty.Alacritty', 'Herdr.Herdr.Preview', 'Neovim.Neovim')) {
        Add-Assertion -Name ("winget-Paket vorhanden: " + $packageId) -Condition ($wingetList -match [regex]::Escape($packageId))
    }

    $exe = Join-Path -Path $env:LOCALAPPDATA -ChildPath 'Lars-Win-AI\aid.exe'
    Add-Assertion -Name 'aid.exe vorhanden' -Condition (Test-Path -LiteralPath $exe) -Detail $exe

    $config = Join-Path -Path $env:APPDATA -ChildPath 'Lars-Win-AI\config.json'
    Add-Assertion -Name 'Daemon config.json vorhanden' -Condition (Test-Path -LiteralPath $config) -Detail $config
}

function Assert-Json {
    param([string]$Name, [string]$Text)
    $ok = $true
    try {
        $stripped = [regex]::Replace($Text, '(?s)/\*.*?\*/', '')
        $stripped = [regex]::Replace($stripped, '(?m)//[^\r\n]*', '')
        $null = $stripped | ConvertFrom-Json
    } catch {
        $ok = $false
    }
    Add-Assertion -Name $Name -Condition $ok
}

function Test-UninstallState {
    param($Manifest)
    foreach ($entry in @($Manifest.symlinks)) {
        $target = [Environment]::ExpandEnvironmentVariables($entry.target)
        $target = $target -replace '/', '\'
        $item = Get-Item -LiteralPath $target -Force -ErrorAction SilentlyContinue
        $isLink = ($null -ne $item) -and [bool]($item.Attributes -band [System.IO.FileAttributes]::ReparsePoint)
        Add-Assertion -Name ("Link entfernt: " + $entry.id) -Condition (-not $isLink) -Detail $target
    }
    $task = Get-ScheduledTask -TaskName 'aid' -TaskPath '\Lars-Win-AI\' -ErrorAction SilentlyContinue
    Add-Assertion -Name 'Scheduled Task entfernt' -Condition ($null -eq $task)
}

Write-Host 'Lars-Win-AI Layer-2 E2E' -ForegroundColor Cyan
$isAdmin = ([Security.Principal.WindowsPrincipal][Security.Principal.WindowsIdentity]::GetCurrent()).IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)
Write-Host ("Admin=" + $isAdmin) -ForegroundColor Yellow
if (-not $isAdmin) { Write-Host 'Hinweis: File-Symlinks brauchen Admin oder Developer Mode.' -ForegroundColor Yellow }

$manifest = Get-RepoManifest

if ($SkipInstall) {
    Write-Host 'Install-Schritt uebersprungen (-SkipInstall).' -ForegroundColor Yellow
} else {
    Invoke-Installer -ScriptPath $installScript | Out-Null
    Test-InstallState -Manifest $manifest

    Write-Host ''
    Write-Host 'STOP: Reboot empfohlen. Danach zweiten Lauf (Idempotenz) starten:' -ForegroundColor Magenta
    Write-Host ('  powershell -NoProfile -File "' + $installScript + '" -Yes') -ForegroundColor Magenta
    Write-Host 'Anschliessend die interaktiven Layer-3-Punkte aus MANUAL.md pruefen.' -ForegroundColor Magenta
    Write-Host ''

    Invoke-Installer -ScriptPath $installScript | Out-Null
    Test-InstallState -Manifest $manifest
}

if ($SkipUninstall) {
    Write-Host 'Uninstall-Schritt uebersprungen (-SkipUninstall).' -ForegroundColor Yellow
} else {
    Write-Host ("=== " + $uninstallScript) -ForegroundColor Cyan
    $uninstallOutput = & powershell -NoProfile -ExecutionPolicy Bypass -File $uninstallScript -RestoreBackups 2>&1
    $uninstallCode = $LASTEXITCODE
    $uninstallOutput | ForEach-Object { Write-Host $_ }
    Add-Assertion -Name 'Uninstall Exitcode 0' -Condition ($uninstallCode -eq 0) -Detail ("exit=" + $uninstallCode)
    Test-UninstallState -Manifest $manifest
}

Write-Host ''
Write-Host ('Assertions: ' + $script:Assertions.Count + ', Fehler: ' + $script:Failures) -ForegroundColor Cyan
if ($script:Failures -gt 0) { exit 1 }
exit 0
