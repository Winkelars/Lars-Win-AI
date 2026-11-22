$ErrorActionPreference = 'Stop'

# Pester 5 runs discovery and execution in separate scopes, so the lib/component
# dot-sources and the fixture helpers must live in a BeforeAll (see lib.Tests.ps1).
BeforeAll {
    $script:RepoRoot = Split-Path -Parent (Split-Path -Parent $PSScriptRoot)
    $script:LibPath = Join-Path -Path $script:RepoRoot -ChildPath 'scripts\lib.ps1'
    . $script:LibPath
    . (Join-Path -Path $script:RepoRoot -ChildPath 'scripts\components\crush.ps1')

    function New-CrushSandbox {
        [Diagnostics.CodeAnalysis.SuppressMessageAttribute('PSUseShouldProcessForStateChangingFunctions', '', Justification = 'Test fixture helper.')]
        param()
        $path = Join-Path -Path $env:TEMP -ChildPath ('lwai-crush-' + [guid]::NewGuid().ToString('N'))
        New-Item -ItemType Directory -Path $path -Force | Out-Null
        return $path
    }

    function Remove-CrushSandbox {
        [Diagnostics.CodeAnalysis.SuppressMessageAttribute('PSUseShouldProcessForStateChangingFunctions', '', Justification = 'Test fixture helper.')]
        param([string]$Path)
        if (Test-Path -LiteralPath $Path) { Remove-Item -LiteralPath $Path -Recurse -Force -ErrorAction SilentlyContinue }
    }

    function New-CrushRepo {
        param([string]$RepoRoot, [switch]$WithSource)
        if ($WithSource) {
            $skillDir = Join-Path -Path $RepoRoot -ChildPath 'config\crush\skill'
            New-Item -ItemType Directory -Path $skillDir -Force | Out-Null
            Set-Content -LiteralPath (Join-Path $skillDir 'SKILL.md') -Value "---`nname: herdr`n---`n# Herdr`n" -Encoding UTF8
        }
        return $RepoRoot
    }

    function New-CrushContext {
        param([string]$RepoRoot, [string]$Target, [switch]$DryRun)
        return @{
            RepoRoot = $RepoRoot
            Manifest = [pscustomobject]@{
                symlinks = @([pscustomobject]@{
                    id        = 'crush-skill'
                    kind      = 'junction'
                    source    = 'config/crush/skill'
                    target    = $Target
                    fallback  = 'error'
                    backup    = $true
                    component = 'crush'
                })
            }
            DryRun   = [bool]$DryRun
            LogFile  = (Join-Path -Path $RepoRoot -ChildPath 'install.log')
            LibPath  = $script:LibPath
        }
    }
}

Describe 'Install-Crush' {

    It 'links the herdr skill into the Crush skills directory' {
        $sandbox = New-CrushSandbox
        try {
            $repo = New-CrushRepo -RepoRoot (Join-Path $sandbox 'repo') -WithSource
            $target = Join-Path $sandbox 'skills\herdr'
            $context = New-CrushContext -RepoRoot $repo -Target $target

            $result = Install-Crush -Context $context

            $result.Component | Should -Be 'crush'
            $result.Status | Should -Be 'installed'
            $result.Changed | Should -Be $true
            (Get-Item -LiteralPath $target -Force).LinkType | Should -Be 'Junction'
            ([bool]((Get-Item -LiteralPath $target -Force).Target -match 'crush\\skill')) | Should -Be $true
            (Test-Path -LiteralPath (Join-Path $target 'SKILL.md')) | Should -Be $true
        } finally { Remove-CrushSandbox $sandbox }
    }

    It 'is idempotent when the junction already points at the source' {
        $sandbox = New-CrushSandbox
        try {
            $repo = New-CrushRepo -RepoRoot (Join-Path $sandbox 'repo') -WithSource
            $target = Join-Path $sandbox 'skills\herdr'
            $context = New-CrushContext -RepoRoot $repo -Target $target

            Install-Crush -Context $context | Out-Null
            $second = Install-Crush -Context $context

            $second.Status | Should -Be 'installed'
            $second.Changed | Should -Be $false
        } finally { Remove-CrushSandbox $sandbox }
    }

    It 'writes nothing on DryRun' {
        $sandbox = New-CrushSandbox
        try {
            $repo = New-CrushRepo -RepoRoot (Join-Path $sandbox 'repo') -WithSource
            $target = Join-Path $sandbox 'skills\herdr'
            $context = New-CrushContext -RepoRoot $repo -Target $target -DryRun

            $result = Install-Crush -Context $context

            $result.Status | Should -Be 'installed'
            $result.Changed | Should -Be $true
            (Test-Path -LiteralPath $target) | Should -Be $false
        } finally { Remove-CrushSandbox $sandbox }
    }

    It 'skips when the skill source is missing' {
        $sandbox = New-CrushSandbox
        try {
            $repo = New-CrushRepo -RepoRoot (Join-Path $sandbox 'repo')
            $target = Join-Path $sandbox 'skills\herdr'
            $context = New-CrushContext -RepoRoot $repo -Target $target

            $result = Install-Crush -Context $context

            $result.Status | Should -Be 'skipped'
            (Test-Path -LiteralPath $target) | Should -Be $false
        } finally { Remove-CrushSandbox $sandbox }
    }
}
