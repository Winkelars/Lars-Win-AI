function Install-Deps {
    [Diagnostics.CodeAnalysis.SuppressMessageAttribute('PSUseSingularNouns', '', Justification = 'Function name frozen by CONTRACTS.md (WS-I).')]
    [CmdletBinding()]
    param(
        [Parameter(Mandatory)]
        [hashtable]$Context
    )
    if (-not (Get-Command -Name Write-Log -ErrorAction SilentlyContinue)) { . $Context.LibPath }

    $result = New-AidResult -Component 'deps'

    if ($Context.SkipDeps) {
        Write-Log 'deps: -SkipDeps aktiv, winget wird übersprungen.' -Level Warn -LogFile $Context.LogFile
        $result.Status = 'skipped'
        $result.Messages += 'winget übersprungen (-SkipDeps).'
        return $result
    }

    $messages = @()
    $changed = $false

    # Der Nerd Font wird nicht hier installiert, sondern im eigenen
    # 'font'-Component (GitHub-Release statt winget-font-Quelle).
    $packages = @(
        @{ Id = 'wez.wezterm';             Name = 'WezTerm' },
        @{ Id = 'Herdr.Herdr.Preview';     Name = 'Herdr (Preview)' },
        @{ Id = 'Neovim.Neovim';           Name = 'Neovim' }
    )

    $failed = @()
    foreach ($package in $packages) {
        $outcome = Invoke-Winget -Id $package.Id -Name $package.Name -Source $package.Source -DryRun:$Context.DryRun -SkipDeps:$Context.SkipDeps
        if (-not $outcome.Ok) {
            $failed += $package.Id
            $messages += "winget-Installation fehlgeschlagen: $($package.Id)"
            Write-Log "deps: $($package.Id) fehlgeschlagen." -Level Error -LogFile $Context.LogFile
            continue
        }
        if ($outcome.Changed) { $changed = $true }

        if ($Context.DryRun) {
            if ($outcome.Changed) {
                Write-Log "deps: [DryRun] $($package.Id) wuerde installiert." -Level Info -LogFile $Context.LogFile
            } else {
                Write-Log "deps: [DryRun] $($package.Id) bereits vorhanden." -Level Info -LogFile $Context.LogFile
            }
        } elseif ($outcome.Changed) {
            Write-Log "deps: $($package.Id) installiert." -Level Success -LogFile $Context.LogFile
        } else {
            Write-Log "deps: $($package.Id) bereits vorhanden." -Level Info -LogFile $Context.LogFile
        }
    }

    $result.Changed = $changed
    $result.Messages = $messages
    if ($failed.Count -gt 0) {
        $result.Status = 'failed'
        return $result
    }
    $result.Status = 'installed'
    return $result
}
