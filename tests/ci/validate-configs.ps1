#Requires -Version 5.1
<#
.SYNOPSIS
    Validiert die Konfigurations-Assets von Lars-Win-AI ohne Systemaenderung.

.DESCRIPTION
    Prueft - sofern vorhanden - manifest.json, die opencode-Themes (JSON),
    Alacritty/Herdr (TOML), die LazyVim-Lua-Syntax sowie `install.ps1 -DryRun`.

    Fehlende Dateien werden uebersprungen und als ::notice:: gemeldet. Existiert
    eine Datei, wird sie zwingend validiert; jeder echte Fehler fuehrt zu Exit 1
    (kein stilles Ueberspringen).

.PARAMETER RepoRoot
    Optionaler Repo-Wurzelpfad. Default: zwei Ebenen ueber diesem Skript.

.EXAMPLE
    pwsh -NoProfile -File tests/ci/validate-configs.ps1
#>
[CmdletBinding()]
param(
    [string]$RepoRoot
)

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'

if (-not $RepoRoot) {
    $scriptDirectory = if ($PSScriptRoot) { $PSScriptRoot } else { Split-Path -Parent $MyInvocation.MyCommand.Path }
    $RepoRoot = Split-Path -Parent (Split-Path -Parent $scriptDirectory)
}
$RepoRoot = (Resolve-Path -LiteralPath $RepoRoot).Path

$script:Passed = 0
$script:Failed = 0
$script:Skipped = 0

function Write-Notice {
    param([string]$Message)
    if ($env:GITHUB_ACTIONS -eq 'true') { Write-Host "::notice::$Message" }
    else { Write-Host "[notice] $Message" }
}

function Write-WarningLine {
    param([string]$Message)
    if ($env:GITHUB_ACTIONS -eq 'true') { Write-Host "::warning::$Message" }
    else { Write-Host "[warn]   $Message" }
}

function Add-Success {
    param([string]$Message)
    $script:Passed++
    Write-Host "[ ok ]   $Message"
}

function Add-Failure {
    param([string]$Message)
    $script:Failed++
    if ($env:GITHUB_ACTIONS -eq 'true') { Write-Host "::error::$Message" }
    else { Write-Host "[FAIL]   $Message" }
}

function Write-SkippedNotice {
    param([string]$RelativePath)
    $script:Skipped++
    Write-Notice "uebersprungen (nicht vorhanden): $RelativePath"
    return $null
}

function Get-RelativeFullPath {
    param([string]$RelativePath)
    $full = Join-Path -Path $RepoRoot -ChildPath $RelativePath
    if (Test-Path -LiteralPath $full) { return (Resolve-Path -LiteralPath $full).Path }
    return Write-SkippedNotice $RelativePath
}

function Test-ThemeNoneKey {
    param($Node, [string]$Key)
    if ($null -eq $Node) { return $false }
    if ($Node -is [System.Management.Automation.PSCustomObject]) {
        foreach ($property in $Node.PSObject.Properties) {
            if ($property.Name -eq $Key -and "$($property.Value)" -eq 'none') { return $true }
            if (Test-ThemeNoneKey -Node $property.Value -Key $Key) { return $true }
        }
    } elseif ($Node -is [System.Collections.IEnumerable] -and $Node -isnot [string]) {
        foreach ($item in $Node) {
            if (Test-ThemeNoneKey -Node $item -Key $Key) { return $true }
        }
    }
    return $false
}

function Invoke-JsonValidation {
    param(
        [string]$RelativePath,
        [string]$Label,
        [scriptblock]$Assertion = $null
    )
    $full = Get-RelativeFullPath $RelativePath
    if (-not $full) { return }

    $text = Get-Content -LiteralPath $full -Raw -Encoding UTF8
    try {
        $data = $text | ConvertFrom-Json
    } catch {
        Add-Failure "$Label : ungueltiges JSON - $($_.Exception.Message)"
        return
    }

    if ($Assertion) {
        try {
            & $Assertion $data
        } catch {
            Add-Failure "$Label : $($_.Exception.Message)"
            return
        }
    }
    Add-Success "$Label : JSON ok"
}

function Get-TomlModuleSpec {
    if (-not (Get-Command node -ErrorAction SilentlyContinue)) { return $null }

    $candidates = New-Object System.Collections.Generic.List[string]
    if ($env:TOML_MODULE_DIR) { $candidates.Add($env:TOML_MODULE_DIR) }

    $resolved = & node -e "try{process.stdout.write(require.resolve('toml'))}catch(e){process.exit(1)}" 2>$null
    if ($LASTEXITCODE -eq 0 -and $resolved) { $candidates.Add((([string]$resolved).Trim())) }

    $candidates.Add('C:/Users/Makkis/.config/opencode/node_modules/toml')

    foreach ($candidate in $candidates) {
        if ($candidate -and (Test-Path -LiteralPath $candidate)) { return $candidate }
    }

    $tempDirectory = Join-Path ([System.IO.Path]::GetTempPath()) ("lars-win-ai-toml-" + [System.Guid]::NewGuid().ToString('N'))
    try {
        New-Item -ItemType Directory -Force -Path $tempDirectory | Out-Null
        & npm install --silent --no-save --prefix $tempDirectory toml 2>&1 | Out-Null
        $module = Join-Path $tempDirectory 'node_modules/toml'
        if (Test-Path -LiteralPath $module) { return $module }
    } catch {
        Write-WarningLine "npm-Fallback fuer den TOML-Parser fehlgeschlagen: $($_.Exception.Message)"
    }
    return $null
}

function Invoke-TomlValidation {
    param([string]$RelativePath, [string]$ModuleSpec)
    $full = Get-RelativeFullPath $RelativePath
    if (-not $full) { return }

    $helper = Join-Path $RepoRoot 'tests/ci/validate-toml.js'
    $output = & node $helper $ModuleSpec $full 2>&1
    if ($LASTEXITCODE -ne 0) {
        Add-Failure "$RelativePath : TOML-Parse fehlgeschlagen - $([string]::Join(' ', @($output)))"
    } else {
        Add-Success "$RelativePath : TOML ok"
    }
}

function Invoke-LuaValidation {
    $directory = Join-Path $RepoRoot 'config/nvim'
    if (-not (Test-Path -LiteralPath $directory)) {
        Write-Notice 'uebersprungen (nicht vorhanden): config/nvim'
        return
    }

    $files = @(Get-ChildItem -LiteralPath $directory -Recurse -Filter *.lua -File)
    if ($files.Count -eq 0) {
        Write-Notice 'config/nvim enthaelt keine Lua-Dateien'
        return
    }

    if (-not (Get-Command nvim -ErrorAction SilentlyContinue)) {
        Add-Failure "nvim nicht gefunden, aber $($files.Count) Lua-Datei(en) vorhanden - Syntaxpruefung nicht moeglich"
        return
    }

    foreach ($file in $files) {
        $path = $file.FullName -replace '\\', '/'
        $code = "lua local f,e=loadfile([[$path]]); if not f then io.stderr:write(tostring(e)..'\n'); vim.cmd('cquit 1') end"
        $output = & nvim --headless -u NONE -c $code -c qa 2>&1
        if ($LASTEXITCODE -ne 0) {
            Add-Failure "Lua-Syntaxfehler: $($file.FullName) - $([string]::Join(' ', @($output)))"
        } else {
            $relative = $file.FullName.Substring($RepoRoot.Length).TrimStart('\')
            Add-Success "Lua ok: $relative"
        }
    }
}

function Invoke-InstallDryRun {
    $install = Join-Path $RepoRoot 'install.ps1'
    if (-not (Test-Path -LiteralPath $install)) {
        Write-Notice 'uebersprungen (nicht vorhanden): install.ps1 -DryRun'
        return
    }

    $executable = if (Get-Command pwsh -ErrorAction SilentlyContinue) { 'pwsh' } else { 'powershell' }
    if (-not (Get-Command $executable -ErrorAction SilentlyContinue)) {
        Add-Failure 'Keine PowerShell-Runtime fuer install.ps1 -DryRun gefunden'
        return
    }

    $logFile = Join-Path $RepoRoot 'install.log'
    $logExisted = Test-Path -LiteralPath $logFile

    $output = & $executable -NoProfile -ExecutionPolicy Bypass -File $install -DryRun -Yes 2>&1
    $exitCode = $LASTEXITCODE

    if ($exitCode -ne 0) {
        Add-Failure "install.ps1 -DryRun Exitcode $exitCode - $([string]::Join(' ', @($output)))"
        return
    }
    if ((-not $logExisted) -and (Test-Path -LiteralPath $logFile)) {
        Add-Failure 'install.ps1 -DryRun hat install.log geschrieben (DryRun muss schreibfrei sein)'
        return
    }
    Add-Success 'install.ps1 -DryRun ok (kein Journal geschrieben)'
}

# --- manifest.json ---------------------------------------------------------

$manifestAssertion = {
    param($manifest)

    if (-not $manifest.name) { throw 'Feld name fehlt' }
    if (-not $manifest.version) { throw 'Feld version fehlt' }
    if (-not $manifest.components -or @($manifest.components).Count -eq 0) { throw 'components fehlt oder leer' }

    $seenIds = @{}
    $seenOrders = @{}
    foreach ($component in @($manifest.components)) {
        foreach ($field in @('id', 'function', 'script', 'order', 'default', 'requires', 'description')) {
            if ($null -eq $component.PSObject.Properties[$field]) { throw "Komponente '$($component.id)': Feld '$field' fehlt" }
        }
        if ($component.id -notmatch '^[a-z]+$') { throw "Komponente '$($component.id)': id muss [a-z]+ sein" }
        if ($seenIds.ContainsKey($component.id)) { throw "Komponente '$($component.id)': id doppelt" }
        $seenIds[$component.id] = $true

        $pascal = $component.id.Substring(0, 1).ToUpperInvariant() + $component.id.Substring(1)
        if ($component.function -ne "Install-$pascal") {
            throw "Komponente '$($component.id)': function muss 'Install-$pascal' sein (ist '$($component.function)')"
        }
        if ($seenOrders.ContainsKey([string]$component.order)) { throw "Komponente '$($component.id)': order $($component.order) doppelt" }
        $seenOrders[[string]$component.order] = $component.id
    }

    if (-not $manifest.symlinks -or @($manifest.symlinks).Count -eq 0) { throw 'symlinks fehlt oder leer' }
    foreach ($link in @($manifest.symlinks)) {
        foreach ($field in @('id', 'kind', 'source', 'target', 'fallback', 'backup', 'component')) {
            if ($null -eq $link.PSObject.Properties[$field]) { throw "Symlink '$($link.id)': Feld '$field' fehlt" }
        }
        if ($link.kind -notin @('junction', 'symlink')) { throw "Symlink '$($link.id)': kind muss 'junction' oder 'symlink' sein" }
        if ($link.fallback -notin @('copy', 'error')) { throw "Symlink '$($link.id)': fallback muss 'copy' oder 'error' sein" }
    }

    if (-not $manifest.env -or -not $manifest.env.AID_WINDOW_TITLE) { throw 'env.AID_WINDOW_TITLE fehlt' }
    if (-not $manifest.release -or $manifest.release.asset -ne 'aid.exe') { throw "release.asset muss 'aid.exe' sein" }
}

# --- opencode-Themes -------------------------------------------------------

$themeAssertion = {
    param($theme)
    foreach ($key in @('background', 'backgroundPanel', 'backgroundElement', 'backgroundMenu')) {
        if (-not (Test-ThemeNoneKey -Node $theme -Key $key)) { throw "Theme-Schluessel '$key' muss 'none' enthalten" }
    }
}

# --- Main ------------------------------------------------------------------

Write-Host '== Lars-Win-AI Config-Validierung =='
Write-Host "Repo: $RepoRoot"

Invoke-JsonValidation -RelativePath 'manifest.json' -Label 'manifest.json' -Assertion $manifestAssertion

$themesDirectory = Join-Path $RepoRoot 'config/opencode/themes'
if (Test-Path -LiteralPath $themesDirectory) {
    $themes = @(Get-ChildItem -LiteralPath $themesDirectory -Filter *.json -File)
    if ($themes.Count -eq 0) {
        Write-Notice 'config/opencode/themes enthaelt keine JSON-Dateien'
    }
    foreach ($theme in $themes) {
        $relative = "config/opencode/themes/$($theme.Name)"
        $assertion = if ($theme.Name -eq 'ai-transparent.json') { $themeAssertion } else { $null }
        Invoke-JsonValidation -RelativePath $relative -Label $relative -Assertion $assertion
    }
} else {
    Write-Notice 'uebersprungen (nicht vorhanden): config/opencode/themes'
}

$tomlTargets = @(@('config/alacritty/alacritty.toml', 'config/herdr/config.toml') |
    Where-Object { Test-Path -LiteralPath (Join-Path $RepoRoot $_) })
if ($tomlTargets.Count -gt 0) {
    $moduleSpec = Get-TomlModuleSpec
    if (-not $moduleSpec) {
        Add-Failure "TOML-Dateien vorhanden ($([string]::Join(', ', @($tomlTargets)))), aber Parser (node 'toml') nicht verfuegbar"
    } else {
        foreach ($relative in $tomlTargets) {
            Invoke-TomlValidation -RelativePath $relative -ModuleSpec $moduleSpec
        }
    }
} else {
    Write-Notice 'uebersprungen (nicht vorhanden): TOML-Configs (Alacritty/Herdr)'
}

Invoke-LuaValidation
Invoke-InstallDryRun

Write-Host ''
Write-Host "Ergebnis: $script:Passed ok, $script:Failed Fehler, $script:Skipped uebersprungen."
if ($script:Failed -gt 0) { exit 1 }
exit 0
