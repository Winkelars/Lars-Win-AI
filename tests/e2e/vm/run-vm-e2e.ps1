#requires -Version 5.1
<#
    tests/e2e/vm/run-vm-e2e.ps1 - Layer-2-E2E-Treiber (HOST-seitig, elevated).

    Steuert eine Hyper-V-VM remote ueber PowerShell Direct (VMBus, kein Netzwerk
    noetig), fuehrt den Installer-E2E (tests/e2e/run.ps1) im Guest aus und legt
    Screenshots + Logs in einem Zeitstempel-Ordner ab.

    Aufruf (in einer ELEVATED Windows PowerShell):
      powershell -NoProfile -ExecutionPolicy Bypass -File tests\e2e\vm\run-vm-e2e.ps1 `
        -GuestUser User -GuestPassword 'Passw0rd!'

    Hinweise:
      - Screenshots zeigen den VM-Konsolenbildschirm. Ist die VM gesperrt/am
        Lock-Screen, ist das Bild der Lock-Screen; nach Login sieht man den Desktop.
      - Guest-Zugangsdaten sind lokale Admin-Zugangsdaten der VM.
#>
[Diagnostics.CodeAnalysis.SuppressMessageAttribute('PSAvoidUsingConvertToSecureStringWithPlainText', '', Justification = 'Guest-Passwort wird zur Laufzeit uebergeben; kein persistiertes Secret.')]
[Diagnostics.CodeAnalysis.SuppressMessageAttribute('PSAvoidUsingPlainTextForPassword', '', Justification = 'Guest-Passwort als Parameter fuer PowerShell Direct.')]
[Diagnostics.CodeAnalysis.SuppressMessageAttribute('PSUseUsingScopeModifierInNewRunspaces', '', Justification = 'Werte werden via -ArgumentList/param uebergeben, nicht per Closure.')]
[CmdletBinding()]
param(
    [string]$VMName,
    [string]$GuestUser = 'User',
    [string]$GuestPassword = 'Passw0rd!',
    [string]$RepoUrl = 'https://github.com/Winkelars/Lars-Win-AI.git',
    [string]$GuestRepoPath = 'C:\Lars-Win-AI',
    [string]$OutDir = (Join-Path $PSScriptRoot 'out'),
    [switch]$SkipClone,
    [switch]$SkipUninstall,
    [switch]$SkipE2E
)
$ErrorActionPreference = 'Stop'
. (Join-Path $PSScriptRoot 'vm-lib.ps1')
Assert-HyperVElevated

$stamp = Get-Date -Format 'yyyyMMdd-HHmmss'
$runDir = Join-Path $OutDir $stamp
New-Item -ItemType Directory -Path $runDir -Force | Out-Null
$logFile = Join-Path $runDir 'driver.log'
function Write-DriverLog([string]$Message) {
    $line = ('[{0}] {1}' -f (Get-Date -Format 'HH:mm:ss'), $Message)
    Write-Host $line
    Add-Content -LiteralPath $logFile -Value $line -Encoding UTF8
}

$vm = Resolve-LabVm -Name $VMName
Write-DriverLog ('VM: ' + $vm.Name)
Start-LabVmAndWait -Vm $vm | Out-Null
Write-DriverLog 'VM laeuft'

$sec = ConvertTo-SecureString $GuestPassword -AsPlainText -Force
$cred = New-Object System.Management.Automation.PSCredential($GuestUser, $sec)
$session = New-LabSession -VMName $vm.Name -Credential $cred
Write-DriverLog 'PowerShell-Direct-Session offen'

$beforeShot = Join-Path $runDir '00-before.png'
if (-not (Save-GuestScreen -Session $session -LocalPath $beforeShot)) {
    Save-VmScreen -VMName $vm.Name -Path $beforeShot | Out-Null
}
Write-DriverLog 'Screenshot 00-before.png'

try {
    $prep = Invoke-Command -Session $session -ScriptBlock {
        param($rp, $ru, $skipClone)
        $ErrorActionPreference = 'Stop'
        if (-not (Get-Command git -ErrorAction SilentlyContinue)) { throw 'git fehlt im Guest.' }
        if (-not $skipClone) {
            if (Test-Path -LiteralPath $rp) {
                git -C $rp pull --ff-only
            } else {
                git clone $ru $rp
            }
        }
        'repo=' + $rp
    } -ArgumentList $GuestRepoPath, $RepoUrl, [bool]$SkipClone
    $prep | ForEach-Object { Write-DriverLog ('guest: ' + $_) }

    if (-not $SkipE2E) {
        $e2eArgs = @('-NoProfile', '-ExecutionPolicy', 'Bypass', '-File', (Join-Path $GuestRepoPath 'tests\e2e\run.ps1'))
        if ($SkipUninstall) { $e2eArgs += '-SkipUninstall' }
        Write-DriverLog ('guest cmd: powershell ' + ($e2eArgs -join ' '))

        $e2e = Invoke-Command -Session $session -ScriptBlock {
            param($arguments)
            $output = & powershell @arguments 2>&1 | Out-String
            [pscustomobject]@{ ExitCode = $LASTEXITCODE; Output = $output }
        } -ArgumentList (, $e2eArgs)

        Set-Content -LiteralPath (Join-Path $runDir 'e2e-run.txt') -Value $e2e.Output -Encoding UTF8
        Write-DriverLog ('E2E exit=' + $e2e.ExitCode)

        $afterShot = Join-Path $runDir '01-after.png'
        if (-not (Save-GuestScreen -Session $session -LocalPath $afterShot)) {
            Save-VmScreen -VMName $vm.Name -Path $afterShot | Out-Null
        }
        Write-DriverLog 'Screenshot 01-after.png'
    } else {
        $e2e = [pscustomobject]@{ ExitCode = 0 }
    }
} finally {
    Remove-PSSession -Session $session -ErrorAction SilentlyContinue
}

$summary = [ordered]@{
    vm           = $vm.Name
    runDir       = $runDir
    e2eExitCode  = $e2e.ExitCode
    screenshots  = @(Get-ChildItem -LiteralPath $runDir -Filter '*.png' | Select-Object -ExpandProperty Name)
}
$summary | ConvertTo-Json | Set-Content -LiteralPath (Join-Path $runDir 'summary.json') -Encoding UTF8
Write-DriverLog ('FERTIG. Ausgabe: ' + $runDir)
if ($e2e.ExitCode -ne 0) { exit 1 }
exit 0
