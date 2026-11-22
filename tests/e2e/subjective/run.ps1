#Requires -Version 5.1
<#
.SYNOPSIS
    Gefuehrte subjektive Bewertung der aid-Slot-Umschaltung.

.DESCRIPTION
    Fuehrt dich durch vier Interaktionsbloecke und fragt danach drei Skalen ab
    (Latenz, Smoothness, Gesamterfahrung). Die Ergebnisse landen als JSON
    ausserhalb des Repos und koennen mit -Compare gegen einen frueheren Lauf
    gestellt werden.

    Bewertet wird bewusst VOR der Anzeige irgendwelcher Messwerte, damit die
    Zahlen die Wahrnehmung nicht ankern.

.PARAMETER OutDir
    Zielverzeichnis fuer die JSON-Ergebnisse.

.PARAMETER Compare
    Pfad zu einem frueheren Ergebnis-JSON fuer den Delta-Vergleich.

.PARAMETER Repetitions
    Wiederholungen pro Block (wird nur angezeigt).

.PARAMETER SkipWarmup
    Ueberspringt den Einlauf-Block.

.EXAMPLE
    powershell -NoProfile -ExecutionPolicy Bypass -File tests\e2e\subjective\run.ps1
#>
[CmdletBinding()]
param(
    [string]$OutDir = (Join-Path $env:LOCALAPPDATA 'Lars-Win-AI\bench\subjective'),
    [string]$Compare = '',
    [int]$Repetitions = 10,
    [switch]$SkipWarmup
)

$ErrorActionPreference = 'Stop'

function Read-Score {
    param(
        [Parameter(Mandatory)][string]$Label,
        [Parameter(Mandatory)][string]$Anchors
    )
    while ($true) {
        $raw = Read-Host ("  {0} [1-10]  ({1})" -f $Label, $Anchors)
        if ($raw -match '^\s*(10|[1-9])\s*$') { return [int]$raw }
        Write-Output '  Bitte eine ganze Zahl von 1 bis 10.'
    }
}

function Read-YesNo {
    param([Parameter(Mandatory)][string]$Label)
    while ($true) {
        $raw = Read-Host ("  {0} [j/n]" -f $Label)
        if ($raw -match '^\s*(j|ja|y|yes|n|nein|no)\s*$') {
            return ($raw -match '^\s*(j|ja|y|yes)\s*$')
        }
        Write-Output '  Bitte j oder n.'
    }
}

function Get-BuildInfo {
    param([Parameter(Mandatory)][string]$Path)
    if (-not (Test-Path -LiteralPath $Path)) { return $null }
    $item = Get-Item -LiteralPath $Path
    $hash = (Get-FileHash -LiteralPath $Path -Algorithm SHA256).Hash
    return [ordered]@{
        path     = $Path
        sha256   = $hash
        built_at = $item.LastWriteTime.ToString('o')
        size     = $item.Length
    }
}

function Save-Result {
    param(
        [Parameter(Mandatory)]$Result,
        [Parameter(Mandatory)][string]$Directory
    )
    if (-not (Test-Path -LiteralPath $Directory)) {
        New-Item -ItemType Directory -Path $Directory -Force | Out-Null
    }
    $name = 'subjective-' + (Get-Date -Format 'yyyyMMdd-HHmmss') + '.json'
    $path = Join-Path $Directory $name
    $Result | ConvertTo-Json -Depth 8 | Set-Content -LiteralPath $path -Encoding UTF8
    return $path
}

function Format-Delta {
    param([Parameter(Mandatory)][int]$Value)
    if ($Value -gt 0) { return "+$Value" }
    return "$Value"
}

function Show-Comparison {
    param(
        [Parameter(Mandatory)]$Current,
        [Parameter(Mandatory)][string]$Path
    )
    if (-not (Test-Path -LiteralPath $Path)) {
        Write-Output "Vergleichsdatei nicht gefunden: $Path"
        return
    }
    $base = Get-Content -LiteralPath $Path -Raw | ConvertFrom-Json
    Write-Output ''
    Write-Output ("Vergleich gegen {0}" -f (Split-Path -Leaf $Path))
    foreach ($block in $Current.blocks) {
        $old = $base.blocks | Where-Object { $_.id -eq $block.id } | Select-Object -First 1
        if ($null -eq $old) { continue }
        $dl = Format-Delta ([int]$block.latency - [int]$old.latency)
        $ds = Format-Delta ([int]$block.smoothness - [int]$old.smoothness)
        $do = Format-Delta ([int]$block.overall - [int]$old.overall)
        Write-Output ('  {0,-16} Latenz {1,3}  Smoothness {2,3}  Gesamt {3,3}' -f $block.id, $dl, $ds, $do)
    }
    Write-Output '  (positiv = besser als der Vergleichslauf)'
}

$repoRoot = (Resolve-Path (Join-Path $PSScriptRoot '..\..\..')).Path
$aidPath = Join-Path $env:LOCALAPPDATA 'Lars-Win-AI\aid.exe'
$configPath = Join-Path $env:APPDATA 'Lars-Win-AI\config.json'

$commit = 'unbekannt'
try {
    $commit = (& git -C $repoRoot rev-parse --short HEAD 2>$null)
} catch {
    $commit = 'kein-git'
}

Add-Type -AssemblyName System.Windows.Forms
$screens = @([System.Windows.Forms.Screen]::AllScreens | ForEach-Object {
    [ordered]@{
        device  = $_.DeviceName
        primary = $_.Primary
        width   = $_.Bounds.Width
        height  = $_.Bounds.Height
        left    = $_.Bounds.Left
        top     = $_.Bounds.Top
    }
})

$result = [ordered]@{
    when        = (Get-Date).ToString('o')
    commit      = $commit
    host        = $env:COMPUTERNAME
    os          = [System.Environment]::OSVersion.VersionString
    repetitions = $Repetitions
    aid         = Get-BuildInfo -Path $aidPath
    config      = Get-BuildInfo -Path $configPath
    monitors    = $screens
    blocks      = @()
}

Write-Output ''
Write-Output 'Subjektive Bewertung der aid-Slot-Umschaltung'
Write-Output '============================================='
Write-Output ("Commit:      {0}" -f $commit)
Write-Output ("aid.exe:     {0}" -f $aidPath)
Write-Output ("Wiederholungen pro Block: {0}" -f $Repetitions)
Write-Output ''
Write-Output 'Voraussetzungen: aid laeuft (Task "aid"), die Slots sind konfiguriert.'
Write-Output 'Bewerte aus dem Bauch heraus, nicht analytisch.'
Write-Output ''

if (-not $SkipWarmup) {
    Write-Output 'Einlauf (wird nicht gewertet):'
    Write-Output ("  Wechsle {0}x zwischen zwei Apps hin und her." -f $Repetitions)
    Read-Host '  Enter, wenn du bereit bist' | Out-Null
    Write-Output ''
}

$blocks = @(
    [ordered]@{
        id          = 'toggle'
        label       = 'Umschalten derselben App'
        instruction = ("Druecke {0}x den Hotkey DERSELBEN App (z.B. Alt+3). Jeder Druck zeigt bzw. versteckt sie." -f $Repetitions)
    },
    [ordered]@{
        id          = 'switch'
        label       = 'Wechsel zwischen zwei Apps'
        instruction = ("Wechsle {0}x zwischen zwei Apps hin und her (z.B. Alt+3 und Alt+1)." -f $Repetitions)
    },
    [ordered]@{
        id          = 'rapid'
        label       = 'Schneller Wechsel'
        instruction = ("Wechsle {0}x so schnell du kannst zwischen denselben zwei Apps." -f $Repetitions)
    },
    [ordered]@{
        id          = 'cold'
        label       = 'Kaltstart'
        instruction = 'Schliesse eine App komplett und hole sie mit ihrem Hotkey zurueck. Wiederhole das 2-3x.'
    }
)

foreach ($block in $blocks) {
    Write-Output ('--- Block: {0} ---' -f $block.label)
    Write-Output ('  {0}' -f $block.instruction)
    Read-Host '  Enter, wenn der Block fertig ist' | Out-Null
    Write-Output ''
    Write-Output '  Bewertung (1 = schlecht, 10 = perfekt):'

    $latency = Read-Score -Label 'Latenz      ' -Anchors '1 = traege, 5 = merkbar, 10 = sofort'
    $smoothness = Read-Score -Label 'Smoothness  ' -Anchors '1 = ruckelt/flackert, 10 = butterweich'
    $overall = Read-Score -Label 'Gesamt      ' -Anchors '1 = unbrauchbar, 10 = perfekt'
    $flicker = Read-YesNo -Label 'Flackern gesehen?'
    $comment = Read-Host '  Kommentar (optional)'

    $result.blocks += [ordered]@{
        id         = $block.id
        label      = $block.label
        latency    = $latency
        smoothness = $smoothness
        overall    = $overall
        flicker    = $flicker
        comment    = $comment
    }
    Write-Output ''
}

$saved = Save-Result -Result $result -Directory $OutDir

Write-Output ''
Write-Output 'Zusammenfassung'
Write-Output '==============='
Write-Output ('{0,-16} {1,7} {2,11} {3,7} {4,8} {5}' -f 'Block', 'Latenz', 'Smoothness', 'Gesamt', 'Flackern', '')
foreach ($block in $result.blocks) {
    $flickerText = 'nein'
    if ($block.flicker) { $flickerText = 'ja' }
    Write-Output ('{0,-16} {1,7} {2,11} {3,7} {4,8} {5}' -f $block.label, $block.latency, $block.smoothness, $block.overall, $flickerText, $block.comment)
}
Write-Output ''
Write-Output ("Gespeichert: {0}" -f $saved)

if ($Compare -ne '') {
    Show-Comparison -Current $result -Path $Compare
}
