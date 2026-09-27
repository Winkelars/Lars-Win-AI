#requires -Version 5.1
# Host-seitige Helfer fuer den Hyper-V-E2E-Treiber (tests/e2e/vm/run-vm-e2e.ps1).
# MUSS mit Administratorrechten laufen (Hyper-V-Cmdlets + Virtualisierungs-WMI).
# Nur Windows PowerShell 5.1 (Get-WmiObject/GetRelated fuer das VM-Thumbnail).

function Assert-HyperVElevated {
    $isAdmin = ([Security.Principal.WindowsPrincipal][Security.Principal.WindowsIdentity]::GetCurrent()).IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)
    if (-not $isAdmin) { throw 'Hyper-V-E2E benoetigt Administratorrechte (Hyper-V + WMI).' }
}

function Resolve-LabVm {
    param([string]$Name)
    Import-Module Hyper-V -ErrorAction Stop
    if ($Name) { return (Get-VM -Name $Name -ErrorAction Stop) }
    $vms = @(Get-VM)
    if ($vms.Count -eq 1) { return $vms[0] }
    if ($vms.Count -eq 0) { throw 'Keine Hyper-V-VM gefunden.' }
    throw ('Mehrere VMs - bitte -VMName angeben: ' + ($vms.Name -join ', '))
}

function Start-LabVmAndWait {
    [Diagnostics.CodeAnalysis.SuppressMessageAttribute('PSUseShouldProcessForStateChangingFunctions', '', Justification = 'Interner VM-Helfer; -WhatIf hier nicht sinnvoll.')]
    param([Parameter(Mandatory)]$Vm, [int]$TimeoutSec = 300)
    if ($Vm.State -ne 'Running') { Start-VM -VM $Vm | Out-Null }
    $deadline = (Get-Date).AddSeconds($TimeoutSec)
    while ((Get-Date) -lt $deadline) {
        $state = (Get-VM -Id $Vm.Id).State
        if ($state -eq 'Running') {
            $hb = Get-VMIntegrationService -VM $Vm -Name 'Heartbeat' -ErrorAction SilentlyContinue
            if ($hb -and "$($hb.PrimaryStatusDescription)" -match 'OK') { return }
        }
        Start-Sleep -Seconds 3
    }
}

function New-LabSession {
    [Diagnostics.CodeAnalysis.SuppressMessageAttribute('PSUseShouldProcessForStateChangingFunctions', '', Justification = 'Interner VM-Helfer; -WhatIf hier nicht sinnvoll.')]
    param(
        [Parameter(Mandatory)][string]$VMName,
        [Parameter(Mandatory)][pscredential]$Credential
    )
    New-PSSession -VMName $VMName -Credential $Credential -ErrorAction Stop
}

# Erstellt einen PNG-Screenshot des aktuellen VM-Bildschirms.
# Wichtig: GetVirtualSystemThumbnailImage erwartet das Msvm_VirtualSystemSettingData
# (nicht das ComputerSystem); sonst kommt ein leeres/weisses Bild zurueck.
function Save-VmScreen {
    [Diagnostics.CodeAnalysis.SuppressMessageAttribute('PSAvoidUsingWMICmdlet', '', Justification = 'Get-WmiObject/GetRelated ist fuer das Hyper-V-Thumbnail zuverlaessig.')]
    param(
        [Parameter(Mandatory)][string]$VMName,
        [Parameter(Mandatory)][string]$Path
    )
    Add-Type -AssemblyName System.Drawing
    $ns = 'root\virtualization\v2'
    $vmcs = Get-WmiObject -Namespace $ns -Class Msvm_ComputerSystem -Filter "ElementName='$VMName'" -ErrorAction Stop
    if (-not $vmcs) { throw "VM '$VMName' nicht via WMI gefunden." }

    $setting = $vmcs.GetRelated('Msvm_VirtualSystemSettingData') | Select-Object -First 1
    $video = $vmcs.GetRelated('Msvm_VideoHead') | Select-Object -First 1
    $x = if ($video) { [int]$video.CurrentHorizontalResolution[0] } else { 1024 }
    $y = if ($video) { [int]$video.CurrentVerticalResolution[0] } else { 768 }
    if ($x -le 0) { $x = 1024 }
    if ($y -le 0) { $y = 768 }

    $svc = Get-WmiObject -Namespace $ns -Class Msvm_VirtualSystemManagementService -ErrorAction Stop
    $data = $svc.GetVirtualSystemThumbnailImage($setting, $x, $y).ImageData
    if (-not $data) { throw 'Keine Bilddaten erhalten.' }

    $bmp = New-Object System.Drawing.Bitmap -ArgumentList $x, $y, ([System.Drawing.Imaging.PixelFormat]::Format16bppRgb565)
    $rect = New-Object System.Drawing.Rectangle 0, 0, $x, $y
    $bd = $bmp.LockBits($rect, [System.Drawing.Imaging.ImageLockMode]::WriteOnly, [System.Drawing.Imaging.PixelFormat]::Format16bppRgb565)
    [System.Runtime.InteropServices.Marshal]::Copy($data, 0, $bd.Scan0, $bd.Stride * $bd.Height)
    $bmp.UnlockBits($bd)

    $dir = Split-Path -Parent $Path
    if ($dir -and -not (Test-Path -LiteralPath $dir)) { New-Item -ItemType Directory -Path $dir -Force | Out-Null }
    $bmp.Save($Path, [System.Drawing.Imaging.ImageFormat]::Png)
    $bmp.Dispose()
    return $Path
}
