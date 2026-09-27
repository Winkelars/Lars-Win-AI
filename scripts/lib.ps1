# scripts/lib.ps1 - WS-I helper contract (docs/CONTRACTS.md §4)
#
# Pure PowerShell (Windows PowerShell 5.1). No external modules.
# Every writing helper respects -DryRun. The installer/uninstaller additionally
# sets $script:AidDryRun so that the shared journal sink (Write-Log) never
# touches the filesystem while planning.

if (-not (Get-Variable -Name 'AidDryRun' -Scope Script -ErrorAction SilentlyContinue)) {
    $script:AidDryRun = $false
}

function Write-Log {
    [Diagnostics.CodeAnalysis.SuppressMessageAttribute('PSAvoidUsingWriteHost', '', Justification = 'Write-Log is the single console + journal sink; host output is intended.')]
    param(
        [Parameter(Position = 0)]
        [string]$Message,
        [ValidateSet('Info', 'Warn', 'Error', 'Success', 'Debug')]
        [string]$Level = 'Info',
        [string]$LogFile
    )
    if ($null -eq $Message) { $Message = '' }
    $stamp = (Get-Date).ToString('yyyy-MM-dd HH:mm:ss')
    $line = '[' + $stamp + '] [' + $Level.ToUpperInvariant() + '] ' + $Message
    $color = 'Gray'
    switch ($Level) {
        'Warn' { $color = 'Yellow' }
        'Error' { $color = 'Red' }
        'Success' { $color = 'Green' }
        'Debug' { $color = 'DarkGray' }
    }
    Write-Host $line -ForegroundColor $color

    if (-not [string]::IsNullOrWhiteSpace($LogFile) -and -not $script:AidDryRun) {
        try {
            $dir = Split-Path -Parent $LogFile
            if ($dir -and -not (Test-Path -LiteralPath $dir)) {
                New-Item -ItemType Directory -Path $dir -Force | Out-Null
            }
            Add-Content -LiteralPath $LogFile -Value $line -Encoding UTF8
        } catch {
            Write-Host ('Write-Log: Journal nicht schreibbar: ' + $_.Exception.Message) -ForegroundColor DarkYellow
        }
    }
}

function Test-Command {
    param([string]$Name)
    if ([string]::IsNullOrWhiteSpace($Name)) { return $false }
    if (Test-Path -LiteralPath $Name) { return $true }
    return [bool](Get-Command -Name $Name -ErrorAction SilentlyContinue)
}

function Test-Admin {
    try {
        $identity = [Security.Principal.WindowsIdentity]::GetCurrent()
        $principal = New-Object Security.Principal.WindowsPrincipal($identity)
        return $principal.IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)
    } catch {
        return $false
    }
}

function Test-DeveloperMode {
    try {
        $key = 'HKLM:\SOFTWARE\Microsoft\Windows\CurrentVersion\AppModelUnlock'
        $value = Get-ItemProperty -Path $key -Name 'AllowDevelopmentWithoutDevLicense' -ErrorAction Stop
        return ($value.AllowDevelopmentWithoutDevLicense -eq 1)
    } catch {
        return $false
    }
}

function Resolve-ManifestPath {
    param([string]$Path)
    if ([string]::IsNullOrWhiteSpace($Path)) { return $null }
    $resolved = $Path.Trim()
    if ($resolved -eq '~') {
        $resolved = $env:USERPROFILE
    } elseif ($resolved.StartsWith('~/') -or $resolved.StartsWith('~\')) {
        $resolved = Join-Path $env:USERPROFILE $resolved.Substring(2)
    }
    $resolved = [Environment]::ExpandEnvironmentVariables($resolved)
    $resolved = $resolved -replace '/', '\'
    return $resolved
}

function Get-NormalizedPath {
    # Kanonisiert einen Pfad fuer den Zielvergleich von Links/Junctions:
    # entfernt \\?\-, normalisiert / -> \ und zieht, wenn der Pfad existiert,
    # den realen FullName (loest u. a. 8.3-Kurznamen auf).
    param([string]$Path)
    if ([string]::IsNullOrWhiteSpace($Path)) { return '' }
    $resolved = ([string]$Path).Trim()
    $resolved = $resolved -replace '^\\\\\?\\', ''
    $resolved = $resolved -replace '/', '\'
    if (Test-Path -LiteralPath $resolved) {
        $item = Get-Item -LiteralPath $resolved -Force -ErrorAction SilentlyContinue
        if ($null -ne $item) {
            return ([string]$item.FullName).TrimEnd('\')
        }
    }
    return $resolved.TrimEnd('\')
}

function Backup-Path {
    [Diagnostics.CodeAnalysis.SuppressMessageAttribute('PSUseApprovedVerbs', '', Justification = 'Function name frozen by CONTRACTS.md (WS-I).')]
    param([string]$Path)
    if ([string]::IsNullOrWhiteSpace($Path)) { return $null }
    if (-not (Test-Path -LiteralPath $Path)) { return $null }
    $item = Get-Item -LiteralPath $Path -Force -ErrorAction SilentlyContinue
    if ($null -eq $item) { return $null }
    if ($item.LinkType) { return $null }
    $backup = $Path + '.bak'
    if (Test-Path -LiteralPath $backup) {
        return $backup
    }
    try {
        Copy-Item -LiteralPath $Path -Destination $backup -Recurse -Force -ErrorAction Stop
        return $backup
    } catch {
        return $null
    }
}

function Remove-LinkSafe {
    [Diagnostics.CodeAnalysis.SuppressMessageAttribute('PSUseShouldProcessForStateChangingFunctions', '', Justification = 'Internal reparse-point removal helper; caller controls DryRun.')]
    param([string]$Path)
    if ([string]::IsNullOrWhiteSpace($Path)) { return }
    $item = Get-Item -LiteralPath $Path -Force -ErrorAction SilentlyContinue
    if ($null -eq $item) { return }
    if ($item.PSIsContainer) {
        [System.IO.Directory]::Delete($Path, $false)
    } else {
        [System.IO.File]::Delete($Path)
    }
}

function New-JunctionSafe {
    [Diagnostics.CodeAnalysis.SuppressMessageAttribute('PSUseShouldProcessForStateChangingFunctions', '', Justification = 'Contract-mandated helper; -DryRun provides WhatIf semantics.')]
    param(
        [string]$LinkPath,
        [string]$TargetPath,
        [switch]$DryRun,
        [switch]$Force
    )
    $result = @{ Ok = $false; Changed = $false; Mode = 'junction' }
    if ([string]::IsNullOrWhiteSpace($LinkPath) -or [string]::IsNullOrWhiteSpace($TargetPath)) { return $result }
    if (-not (Test-Path -LiteralPath $TargetPath)) { return $result }

    $targetNorm = Get-NormalizedPath -Path $TargetPath
    $existing = Get-Item -LiteralPath $LinkPath -Force -ErrorAction SilentlyContinue

    if ($null -ne $existing) {
        $isReparse = [bool]($existing.Attributes -band [System.IO.FileAttributes]::ReparsePoint)
        if ($isReparse) {
            $links = @($existing.Target)
            $targetMatches = $false
            foreach ($candidate in $links) {
                if ([string]::IsNullOrWhiteSpace($candidate)) { continue }
                if ((Get-NormalizedPath -Path $candidate) -ieq $targetNorm) { $targetMatches = $true }
            }
            if ($targetMatches) {
                $result.Ok = $true
                $result.Changed = $false
                return $result
            }
            if (-not $DryRun) {
                Remove-LinkSafe -Path $LinkPath
            }
            $result.Changed = $true
        } else {
            if (-not $Force) { return $result }
            if (-not $DryRun) {
                Remove-Item -LiteralPath $LinkPath -Recurse -Force -ErrorAction SilentlyContinue
            }
            $result.Changed = $true
        }
    } else {
        $result.Changed = $true
    }

    if ($DryRun) {
        $result.Ok = $true
        return $result
    }

    try {
        $parent = Split-Path -Parent $LinkPath
        if ($parent -and -not (Test-Path -LiteralPath $parent)) {
            New-Item -ItemType Directory -Path $parent -Force | Out-Null
        }
        New-Item -ItemType Junction -Path $LinkPath -Target $TargetPath -ErrorAction Stop | Out-Null
        $result.Ok = $true
    } catch {
        $result.Ok = $false
    }
    return $result
}

function New-FileSymlinkOrCopy {
    [Diagnostics.CodeAnalysis.SuppressMessageAttribute('PSUseShouldProcessForStateChangingFunctions', '', Justification = 'Contract-mandated helper; -DryRun provides WhatIf semantics.')]
    param(
        [string]$LinkPath,
        [string]$TargetPath,
        [switch]$DryRun,
        [switch]$Force
    )
    $result = @{ Ok = $false; Changed = $false; Mode = 'symlink' }
    if ([string]::IsNullOrWhiteSpace($LinkPath) -or [string]::IsNullOrWhiteSpace($TargetPath)) { return $result }
    if (-not (Test-Path -LiteralPath $TargetPath)) { return $result }

    $targetNorm = Get-NormalizedPath -Path $TargetPath
    $existing = Get-Item -LiteralPath $LinkPath -Force -ErrorAction SilentlyContinue

    if ($null -ne $existing) {
        $isReparse = [bool]($existing.Attributes -band [System.IO.FileAttributes]::ReparsePoint)
        if ($isReparse) {
            $links = @($existing.Target)
            foreach ($candidate in $links) {
                if ([string]::IsNullOrWhiteSpace($candidate)) { continue }
                if ((Get-NormalizedPath -Path $candidate) -ieq $targetNorm) {
                    $result.Ok = $true
                    $result.Changed = $false
                    $result.Mode = 'symlink'
                    return $result
                }
            }
            if (-not $DryRun) {
                Remove-LinkSafe -Path $LinkPath
            }
            $result.Changed = $true
        } else {
            $same = $false
            try {
                $same = ((Get-FileHash -LiteralPath $LinkPath -Algorithm SHA256).Hash -eq (Get-FileHash -LiteralPath $TargetPath -Algorithm SHA256).Hash)
            } catch {
                $same = $false
            }
            if ($same) {
                $result.Ok = $true
                $result.Changed = $false
                $result.Mode = 'copy'
                return $result
            }
            if (-not $Force) { return $result }
            if (-not $DryRun) {
                Remove-Item -LiteralPath $LinkPath -Force -ErrorAction SilentlyContinue
            }
            $result.Changed = $true
        }
    } else {
        $result.Changed = $true
    }

    if ($DryRun) {
        $result.Ok = $true
        $result.Mode = 'symlink'
        return $result
    }

    $parent = Split-Path -Parent $LinkPath
    if ($parent -and -not (Test-Path -LiteralPath $parent)) {
        New-Item -ItemType Directory -Path $parent -Force | Out-Null
    }

    try {
        New-Item -ItemType SymbolicLink -Path $LinkPath -Target $TargetPath -ErrorAction Stop | Out-Null
        $result.Ok = $true
        $result.Mode = 'symlink'
    } catch {
        try {
            Copy-Item -LiteralPath $TargetPath -Destination $LinkPath -Force -ErrorAction Stop
            $result.Ok = $true
            $result.Mode = 'copy'
        } catch {
            $result.Ok = $false
        }
    }
    return $result
}

function ConvertFrom-Jsonc {
    param([string]$Text)
    if ([string]::IsNullOrWhiteSpace($Text)) { return $null }
    $sb = New-Object System.Text.StringBuilder
    $inString = $false
    $escaped = $false
    $inLineComment = $false
    $inBlockComment = $false
    for ($i = 0; $i -lt $Text.Length; $i++) {
        $c = $Text[$i]
        if ($inLineComment) {
            if ($c -eq "`n") { $inLineComment = $false; [void]$sb.Append($c) }
            continue
        }
        if ($inBlockComment) {
            if ($c -eq '*' -and ($i + 1) -lt $Text.Length -and $Text[$i + 1] -eq '/') {
                $inBlockComment = $false
                $i++
            }
            continue
        }
        if ($inString) {
            [void]$sb.Append($c)
            if ($escaped) { $escaped = $false }
            elseif ($c -eq '\') { $escaped = $true }
            elseif ($c -eq '"') { $inString = $false }
            continue
        }
        if ($c -eq '"') {
            $inString = $true
            [void]$sb.Append($c)
            continue
        }
        if ($c -eq '/' -and ($i + 1) -lt $Text.Length) {
            $next = $Text[$i + 1]
            if ($next -eq '/') { $inLineComment = $true; $i++; continue }
            if ($next -eq '*') { $inBlockComment = $true; $i++; continue }
        }
        [void]$sb.Append($c)
    }
    return ($sb.ToString() | ConvertFrom-Json)
}

function Get-JsoncClosingIndex {
    param([string]$Text, [int]$OpenIndex)
    if ($OpenIndex -lt 0 -or $OpenIndex -ge $Text.Length) { return -1 }
    $open = $Text[$OpenIndex]
    if ($open -eq '{') { $close = '}' } elseif ($open -eq '[') { $close = ']' } else { return -1 }
    $depth = 0
    $inString = $false
    $escaped = $false
    $inLineComment = $false
    $inBlockComment = $false
    for ($i = $OpenIndex; $i -lt $Text.Length; $i++) {
        $c = $Text[$i]
        if ($inLineComment) {
            if ($c -eq "`n") { $inLineComment = $false }
            continue
        }
        if ($inBlockComment) {
            if ($c -eq '*' -and ($i + 1) -lt $Text.Length -and $Text[$i + 1] -eq '/') { $inBlockComment = $false; $i++ }
            continue
        }
        if ($inString) {
            if ($escaped) { $escaped = $false }
            elseif ($c -eq '\') { $escaped = $true }
            elseif ($c -eq '"') { $inString = $false }
            continue
        }
        if ($c -eq '"') { $inString = $true; continue }
        if ($c -eq '/' -and ($i + 1) -lt $Text.Length) {
            $next = $Text[$i + 1]
            if ($next -eq '/') { $inLineComment = $true; $i++; continue }
            if ($next -eq '*') { $inBlockComment = $true; $i++; continue }
        }
        if ($c -eq $open) { $depth++ }
        elseif ($c -eq $close) {
            $depth--
            if ($depth -eq 0) { return $i }
        }
    }
    return -1
}

function Get-JsoncMemberRange {
    param([string]$Text, [string]$Key)
    $result = @{ Found = $false; ObjectStart = -1; ObjectEnd = -1 }
    if ([string]::IsNullOrEmpty($Text) -or [string]::IsNullOrEmpty($Key)) { return $result }
    $pattern = '"' + [regex]::Escape($Key) + '"\s*:'
    $match = [regex]::Match($Text, $pattern)
    if (-not $match.Success) { return $result }
    $result.Found = $true
    $i = $match.Index + $match.Length
    while ($i -lt $Text.Length -and [char]::IsWhiteSpace($Text[$i])) { $i++ }
    if ($i -lt $Text.Length -and $Text[$i] -eq '{') {
        $close = Get-JsoncClosingIndex -Text $Text -OpenIndex $i
        if ($close -ge 0) {
            $result.ObjectStart = $i
            $result.ObjectEnd = $close
        }
    }
    return $result
}

function Get-JsoncEntry {
    param($Value)
    $entries = @()
    if ($null -eq $Value) { return $entries }
    if ($Value -is [System.Collections.IDictionary]) {
        foreach ($key in @($Value.Keys)) {
            $entries += @{ Key = [string]$key; Value = $Value[$key] }
        }
    } elseif ($Value -is [pscustomobject]) {
        foreach ($property in $Value.PSObject.Properties) {
            $entries += @{ Key = $property.Name; Value = $property.Value }
        }
    }
    return $entries
}

function Add-JsoncMember {
    param([string]$Text, [int]$CloseIndex, [string]$Member, [bool]$HasContent)
    if ($CloseIndex -lt 0 -or $CloseIndex -gt $Text.Length) { return $Text }
    $separator = ' '
    if ($HasContent) { $separator = ', ' }
    return $Text.Substring(0, $CloseIndex) + $separator + $Member + $Text.Substring($CloseIndex)
}

function Add-JsoncTopLevel {
    param([string]$Text, [string]$Member)
    $close = $Text.LastIndexOf('}')
    if ($close -lt 0) { return $Text }
    $open = $Text.IndexOf('{')
    $hasContent = $false
    if ($open -ge 0 -and $close -gt $open) {
        $inner = $Text.Substring($open + 1, $close - $open - 1)
        $stripped = [regex]::Replace($inner, '(?s)/\*.*?\*/', '')
        $stripped = [regex]::Replace($stripped, '(?m)//[^\r\n]*', '')
        $hasContent = $stripped.Trim().Length -gt 0
    }
    $separator = ' '
    if ($hasContent) { $separator = ', ' }
    return $Text.Substring(0, $close) + $separator + $Member + $Text.Substring($close)
}

function Merge-OpencodeJsonc {
    [Diagnostics.CodeAnalysis.SuppressMessageAttribute('PSUseApprovedVerbs', '', Justification = 'Function name frozen by CONTRACTS.md (WS-I).')]
    [Diagnostics.CodeAnalysis.SuppressMessageAttribute('PSUseShouldProcessForStateChangingFunctions', '', Justification = 'Contract-mandated helper; -DryRun provides WhatIf semantics.')]
    param(
        [Parameter(Mandatory)]
        [string]$Path,
        [hashtable]$Fragment,
        [switch]$DryRun
    )
    $result = @{ Ok = $true; Changed = $false; Backup = $null; Mode = 'jsonc' }
    if ([string]::IsNullOrWhiteSpace($Path)) {
        $result.Ok = $false
        return $result
    }
    if ($null -eq $Fragment) { $Fragment = @{} }

    if (-not (Test-Path -LiteralPath $Path)) {
        $result.Changed = $true
        $result.Mode = 'json'
        if (-not $DryRun) {
            $parent = Split-Path -Parent $Path
            if ($parent -and -not (Test-Path -LiteralPath $parent)) {
                New-Item -ItemType Directory -Path $parent -Force | Out-Null
            }
            $json = ConvertTo-Json -InputObject $Fragment -Depth 12
            Set-Content -LiteralPath $Path -Value $json -Encoding UTF8
        }
        return $result
    }

    $original = Get-Content -LiteralPath $Path -Raw
    if ($null -eq $original) { $original = '' }
    $text = $original
    if ($text -match '//' -or $text -match '/\*') { $result.Mode = 'jsonc' } else { $result.Mode = 'json' }

    $changed = $false
    foreach ($key in @($Fragment.Keys)) {
        $value = $Fragment[$key]
        if ($null -eq $value) { continue }

        $isObject = ($value -is [System.Collections.IDictionary]) -or ($value -is [pscustomobject])
        if (-not $isObject) {
            $bounds = Get-JsoncMemberRange -Text $text -Key $key
            if (-not $bounds.Found) {
                $member = '"' + $key + '": ' + (ConvertTo-Json -InputObject $value -Depth 12 -Compress)
                $text = Add-JsoncTopLevel -Text $text -Member $member
                $changed = $true
            }
            continue
        }

        $entries = @(Get-JsoncEntry -Value $value)
        if ($entries.Count -eq 0) { continue }

        $bounds = Get-JsoncMemberRange -Text $text -Key $key
        if ($bounds.Found -and $bounds.ObjectStart -ge 0) {
            foreach ($entry in $entries) {
                $recheck = Get-JsoncMemberRange -Text $text -Key $key
                if (-not ($recheck.Found -and $recheck.ObjectStart -ge 0)) { break }
                $inner = $text.Substring($recheck.ObjectStart + 1, $recheck.ObjectEnd - $recheck.ObjectStart - 1)
                $present = [regex]::IsMatch($inner, '"' + [regex]::Escape($entry.Key) + '"\s*:')
                if ($present) { continue }
                $stripped = [regex]::Replace($inner, '(?s)/\*.*?\*/', '')
                $stripped = [regex]::Replace($stripped, '(?m)//[^\r\n]*', '')
                $hasContent = $stripped.Trim().Length -gt 0
                $member = '"' + $entry.Key + '": ' + (ConvertTo-Json -InputObject $entry.Value -Depth 12 -Compress)
                $text = Add-JsoncMember -Text $text -CloseIndex $recheck.ObjectEnd -Member $member -HasContent $hasContent
                $changed = $true
            }
        } else {
            $parts = @()
            foreach ($entry in $entries) {
                $parts += '"' + $entry.Key + '": ' + (ConvertTo-Json -InputObject $entry.Value -Depth 12 -Compress)
            }
            $member = '"' + $key + '": { ' + ($parts -join ', ') + ' }'
            $text = Add-JsoncTopLevel -Text $text -Member $member
            $changed = $true
        }
    }

    if (-not $changed) { return $result }
    $result.Changed = $true
    if (-not $DryRun) {
        $result.Backup = Backup-Path -Path $Path
        Set-Content -LiteralPath $Path -Value $text -Encoding UTF8 -NoNewline
    }
    return $result
}

function Test-WingetPackage {
    # `winget list --id <id> -e` matcht manche Pakete nicht (z. B. Fonts, deren
    # Listen-Id "FONT\\User\\<id>" lautet). Deshalb Volltext-Suche ueber die
    # komplette Liste.
    param(
        [string]$Id,
        [string]$Name
    )
    if ([string]::IsNullOrWhiteSpace($Id) -and [string]::IsNullOrWhiteSpace($Name)) { return $false }
    try {
        $output = (& winget list --accept-source-agreements 2>&1 | Out-String)
    } catch {
        return $false
    }
    if (-not [string]::IsNullOrWhiteSpace($Id) -and $output -match [regex]::Escape($Id)) { return $true }
    if (-not [string]::IsNullOrWhiteSpace($Name) -and $output -match [regex]::Escape($Name)) { return $true }
    return $false
}

function Invoke-Winget {
    param(
        [string]$Id,
        [string]$Name,
        [switch]$DryRun,
        [switch]$SkipDeps
    )
    $result = @{ Ok = $true; Installed = $false; Changed = $false }
    if ($SkipDeps) { return $result }
    if ([string]::IsNullOrWhiteSpace($Id)) {
        $result.Ok = $false
        return $result
    }
    if (-not (Test-Command -Name 'winget')) {
        $result.Ok = $false
        return $result
    }

    if ($DryRun) {
        # Plan only: never invoke the winget executable while dry-running.
        $result.Changed = $true
        return $result
    }

    if (Test-WingetPackage -Id $Id -Name $Name) {
        $result.Installed = $true
        return $result
    }

    try {
        $arguments = @('install', '--id', $Id, '-e', '--silent', '--accept-package-agreements', '--accept-source-agreements')
        & winget @arguments 2>&1 | Out-Null
        if ($LASTEXITCODE -ne 0) {
            # Manche Pakete melden "bereits installiert" mit Fehlercode; ist das
            # Paket danach vorhanden, ist das Ziel dennoch erreicht.
            if (Test-WingetPackage -Id $Id -Name $Name) {
                $result.Installed = $true
                $result.Changed = $true
                return $result
            }
            $result.Ok = $false
            return $result
        }
        $result.Installed = $true
        $result.Changed = $true
    } catch {
        $result.Ok = $false
    }
    return $result
}

function New-AidResult {
    [Diagnostics.CodeAnalysis.SuppressMessageAttribute('PSUseShouldProcessForStateChangingFunctions', '', Justification = 'Factory for the contract result hashtable, not a system change.')]
    param([string]$Component)
    return @{
        Component = $Component
        Status    = 'skipped'
        Changed   = $false
        Messages  = @()
    }
}

# --- Nerd Font (CaskaydiaCove) ---------------------------------------------
# Der Font wird NICHT ueber winget installiert (die 'winget-font'-Quelle ist
# nicht auf jedem System vorhanden, und der E2E wuerde eine externe Quelle
# voraussetzen). Stattdessen laden wir das gepinnte Nerd-Fonts-Release von
# GitHub und registrieren die Schnitte pro Benutzer (kein Admin noetig).

$script:AidFontSpec = @{
    Repo    = 'ryanoasis/nerd-fonts'
    Version = '3.4.0'
    Asset   = 'CascadiaCode.zip'
    Family  = 'CaskaydiaCove Nerd Font'
    Entries = @(
        [pscustomobject]@{ File = 'CaskaydiaCoveNerdFont-Regular.ttf';    Name = 'CaskaydiaCove Nerd Font (TrueType)' }
        [pscustomobject]@{ File = 'CaskaydiaCoveNerdFont-Bold.ttf';       Name = 'CaskaydiaCove Nerd Font Bold (TrueType)' }
        [pscustomobject]@{ File = 'CaskaydiaCoveNerdFont-Italic.ttf';     Name = 'CaskaydiaCove Nerd Font Italic (TrueType)' }
        [pscustomobject]@{ File = 'CaskaydiaCoveNerdFont-BoldItalic.ttf'; Name = 'CaskaydiaCove Nerd Font Bold Italic (TrueType)' }
    )
}

function Get-AidFontDirectory {
    return (Join-Path -Path $env:LOCALAPPDATA -ChildPath 'Microsoft\Windows\Fonts')
}

function Get-AidFontRegistryPath {
    return 'HKCU:\Software\Microsoft\Windows NT\CurrentVersion\Fonts'
}

function Initialize-AidFontNative {
    if (-not ('Aid.FontNative' -as [type])) {
        Add-Type -Namespace 'Aid' -Name 'FontNative' -MemberDefinition @'
[DllImport("gdi32.dll", SetLastError=true, CharSet=CharSet.Unicode)] public static extern int AddFontResourceW(string lpFileName);
[DllImport("gdi32.dll", SetLastError=true, CharSet=CharSet.Unicode)] public static extern bool RemoveFontResourceW(string lpFileName);
[DllImport("user32.dll", SetLastError=true)] public static extern IntPtr SendMessageTimeout(IntPtr hWnd, uint Msg, IntPtr wParam, IntPtr lParam, uint fuFlags, uint uTimeout, out IntPtr lpdwResult);
'@
    }
}

function Update-AidFontChange {
    [Diagnostics.CodeAnalysis.SuppressMessageAttribute('PSUseShouldProcessForStateChangingFunctions', '', Justification = 'Interner Helfer; loest lediglich WM_FONTCHANGE aus.')]
    [CmdletBinding()]
    param()
    Initialize-AidFontNative
    # HWND_BROADCAST + WM_FONTCHANGE, SMTO_ABORTIFHUNG.
    $result = [IntPtr]::Zero
    [void][Aid.FontNative]::SendMessageTimeout([IntPtr]0xffff, 0x001D, [IntPtr]::Zero, [IntPtr]::Zero, 0x0002, 1000, [ref]$result)
}

function Test-AidFontInstalled {
    $dir = Get-AidFontDirectory
    $reg = Get-AidFontRegistryPath
    if (-not (Test-Path -LiteralPath $reg)) { return $false }
    foreach ($entry in $script:AidFontSpec.Entries) {
        if (-not (Test-Path -LiteralPath (Join-Path -Path $dir -ChildPath $entry.File))) { return $false }
        if ($null -eq (Get-ItemProperty -Path $reg -Name $entry.Name -ErrorAction SilentlyContinue)) { return $false }
    }
    return $true
}

function Install-AidFont {
    [Diagnostics.CodeAnalysis.SuppressMessageAttribute('PSUseShouldProcessForStateChangingFunctions', '', Justification = 'Interner Installer-Helfer; Planung erfolgt ueber -DryRun.')]
    [CmdletBinding()]
    param(
        [string]$LogFile,
        [switch]$DryRun
    )
    $result = @{ Ok = $true; Changed = $false; Messages = @() }
    if (Test-AidFontInstalled) { return $result }
    if ($DryRun) {
        $result.Changed = $true
        $result.Messages += "[DryRun] Nerd Font '$($script:AidFontSpec.Family)' wuerde installiert."
        return $result
    }

    $url = 'https://github.com/{0}/releases/download/v{1}/{2}' -f $script:AidFontSpec.Repo, $script:AidFontSpec.Version, $script:AidFontSpec.Asset
    $temp = Join-Path -Path $env:TEMP -ChildPath ('lwai-font-' + [Guid]::NewGuid().ToString('N'))
    try {
        New-Item -ItemType Directory -Path $temp -Force | Out-Null
        $zip = Join-Path -Path $temp -ChildPath 'font.zip'
        Write-Log "font: lade $url" -Level Info -LogFile $LogFile
        Invoke-WebRequest -Uri $url -OutFile $zip -UseBasicParsing -ErrorAction Stop
        $extract = Join-Path -Path $temp -ChildPath 'extract'
        Expand-Archive -LiteralPath $zip -DestinationPath $extract -Force -ErrorAction Stop

        $dir = Get-AidFontDirectory
        New-Item -ItemType Directory -Path $dir -Force | Out-Null
        $reg = Get-AidFontRegistryPath
        if (-not (Test-Path -LiteralPath $reg)) { New-Item -Path $reg -Force | Out-Null }

        Initialize-AidFontNative
        foreach ($entry in $script:AidFontSpec.Entries) {
            $source = Get-ChildItem -LiteralPath $extract -Recurse -Filter $entry.File -ErrorAction SilentlyContinue | Select-Object -First 1
            if ($null -eq $source) { throw "Font-Datei fehlt im Archiv: $($entry.File)" }
            $target = Join-Path -Path $dir -ChildPath $entry.File
            Copy-Item -LiteralPath $source.FullName -Destination $target -Force
            New-ItemProperty -Path $reg -Name $entry.Name -Value $target -PropertyType String -Force | Out-Null
            [void][Aid.FontNative]::AddFontResourceW($target)
        }
        Update-AidFontChange
        $result.Changed = $true
        Write-Log "font: '$($script:AidFontSpec.Family)' installiert ($dir)." -Level Success -LogFile $LogFile
    } catch {
        $result.Ok = $false
        $result.Messages += "Font-Installation fehlgeschlagen: $($_.Exception.Message)"
        Write-Log "font: Installation fehlgeschlagen: $($_.Exception.Message)" -Level Error -LogFile $LogFile
    } finally {
        Remove-Item -LiteralPath $temp -Recurse -Force -ErrorAction SilentlyContinue
    }
    return $result
}

function Uninstall-AidFont {
    [Diagnostics.CodeAnalysis.SuppressMessageAttribute('PSUseShouldProcessForStateChangingFunctions', '', Justification = 'Interner Uninstaller-Helfer; Planung erfolgt ueber -DryRun.')]
    [CmdletBinding()]
    param(
        [switch]$DryRun
    )
    $dir = Get-AidFontDirectory
    $reg = Get-AidFontRegistryPath
    $removed = $false
    Initialize-AidFontNative
    foreach ($entry in $script:AidFontSpec.Entries) {
        $target = Join-Path -Path $dir -ChildPath $entry.File
        if (-not (Test-Path -LiteralPath $target)) { continue }
        $removed = $true
        if ($DryRun) { continue }
        [void][Aid.FontNative]::RemoveFontResourceW($target)
        Remove-ItemProperty -Path $reg -Name $entry.Name -ErrorAction SilentlyContinue
        Remove-Item -LiteralPath $target -Force -ErrorAction SilentlyContinue
    }
    if ($removed -and -not $DryRun) { Update-AidFontChange }
    return $removed
}
