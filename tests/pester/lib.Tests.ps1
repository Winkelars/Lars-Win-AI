$ErrorActionPreference = 'Stop'

# Pester 5 runs test discovery and execution in separate scopes, so the lib
# dot-source and the fixture helpers must live in a BeforeAll so they are
# available inside the It blocks. Requires Pester >= 5 (see docs/TESTING.md).
BeforeAll {
    $LibPath = Join-Path -Path (Split-Path -Parent (Split-Path -Parent $PSScriptRoot)) -ChildPath 'scripts\lib.ps1'
    . $LibPath

    function New-TestSandbox {
        [Diagnostics.CodeAnalysis.SuppressMessageAttribute('PSUseShouldProcessForStateChangingFunctions', '', Justification = 'Test fixture helper.')]
        param()
        $path = Join-Path -Path $env:TEMP -ChildPath ('lwai-pester-' + [guid]::NewGuid().ToString('N'))
        New-Item -ItemType Directory -Path $path -Force | Out-Null
        return $path
    }

    function Remove-TestSandbox {
        [Diagnostics.CodeAnalysis.SuppressMessageAttribute('PSUseShouldProcessForStateChangingFunctions', '', Justification = 'Test fixture helper.')]
        param([string]$Path)
        if (Test-Path -LiteralPath $Path) { Remove-Item -LiteralPath $Path -Recurse -Force -ErrorAction SilentlyContinue }
    }
}

Describe 'lib.ps1 helpers' {

    Context 'Resolve-ManifestPath' {
        It 'expands environment variables and normalizes slashes' {
            $resolved = Resolve-ManifestPath -Path '%APPDATA%/alacritty'
            ([string]::IsNullOrEmpty($resolved)) | Should -Be $false
            $resolved | Should -Be ($resolved -replace '/', '\')
            ($resolved.StartsWith($env:APPDATA)) | Should -Be $true
        }
    }

    Context 'Backup-Path' {
        It 'returns null when the path does not exist' {
            $sandbox = New-TestSandbox
            try {
                (Backup-Path -Path (Join-Path $sandbox 'nope.txt')) | Should -BeNullOrEmpty
            } finally { Remove-TestSandbox $sandbox }
        }

        It 'copies an existing file to .bak and returns the backup path' {
            $sandbox = New-TestSandbox
            try {
                $file = Join-Path $sandbox 'file.txt'
                Set-Content -LiteralPath $file -Value 'original' -Encoding UTF8
                $backup = Backup-Path -Path $file
                $backup | Should -Be ($file + '.bak')
                (Test-Path -LiteralPath $backup) | Should -Be $true
                (Get-Content -LiteralPath $backup -Raw).Trim() | Should -Be 'original'
                (Test-Path -LiteralPath $file) | Should -Be $true
            } finally { Remove-TestSandbox $sandbox }
        }

        It 'keeps the first backup on repeated calls' {
            $sandbox = New-TestSandbox
            try {
                $file = Join-Path $sandbox 'file.txt'
                Set-Content -LiteralPath $file -Value 'v1' -Encoding UTF8
                $first = Backup-Path -Path $file
                Set-Content -LiteralPath $file -Value 'v2' -Encoding UTF8
                $second = Backup-Path -Path $file
                $second | Should -Be $first
                (Get-Content -LiteralPath $first -Raw).Trim() | Should -Be 'v1'
            } finally { Remove-TestSandbox $sandbox }
        }
    }

    Context 'New-JunctionSafe' {
        It 'creates a junction pointing at the target' {
            $sandbox = New-TestSandbox
            try {
                $target = Join-Path $sandbox 'target'
                New-Item -ItemType Directory -Path $target -Force | Out-Null
                $link = Join-Path $sandbox 'link'
                $result = New-JunctionSafe -LinkPath $link -TargetPath $target
                $result.Ok | Should -Be $true
                $result.Changed | Should -Be $true
                $result.Mode | Should -Be 'junction'
                (Get-Item -LiteralPath $link -Force).LinkType | Should -Be 'Junction'
            } finally { Remove-TestSandbox $sandbox }
        }

        It 'is idempotent when the junction already points at the target' {
            $sandbox = New-TestSandbox
            try {
                $target = Join-Path $sandbox 'target'
                New-Item -ItemType Directory -Path $target -Force | Out-Null
                $link = Join-Path $sandbox 'link'
                New-JunctionSafe -LinkPath $link -TargetPath $target | Out-Null
                $result = New-JunctionSafe -LinkPath $link -TargetPath $target
                $result.Ok | Should -Be $true
                $result.Changed | Should -Be $false
            } finally { Remove-TestSandbox $sandbox }
        }

        It 'writes nothing on DryRun' {
            $sandbox = New-TestSandbox
            try {
                $target = Join-Path $sandbox 'target'
                New-Item -ItemType Directory -Path $target -Force | Out-Null
                $link = Join-Path $sandbox 'link'
                $result = New-JunctionSafe -LinkPath $link -TargetPath $target -DryRun
                $result.Ok | Should -Be $true
                $result.Changed | Should -Be $true
                (Test-Path -LiteralPath $link) | Should -Be $false
            } finally { Remove-TestSandbox $sandbox }
        }

        It 'fails when the target does not exist' {
            $sandbox = New-TestSandbox
            try {
                $result = New-JunctionSafe -LinkPath (Join-Path $sandbox 'link') -TargetPath (Join-Path $sandbox 'missing')
                $result.Ok | Should -Be $false
            } finally { Remove-TestSandbox $sandbox }
        }
    }

    Context 'Remove-LinkSafe' {
        It 'removes a junction without deleting the target contents' {
            $sandbox = New-TestSandbox
            try {
                $target = Join-Path $sandbox 'target'
                New-Item -ItemType Directory -Path $target -Force | Out-Null
                Set-Content -LiteralPath (Join-Path $target 'keep.txt') -Value 'important' -Encoding UTF8
                $link = Join-Path $sandbox 'link'
                New-Item -ItemType Junction -Path $link -Target $target | Out-Null
                Remove-LinkSafe -Path $link
                (Test-Path -LiteralPath $link) | Should -Be $false
                (Test-Path -LiteralPath (Join-Path $target 'keep.txt')) | Should -Be $true
            } finally { Remove-TestSandbox $sandbox }
        }
    }

    Context 'New-FileSymlinkOrCopy' {
        It 'links or copies the file so the content matches' {
            $sandbox = New-TestSandbox
            try {
                $target = Join-Path $sandbox 'source.txt'
                Set-Content -LiteralPath $target -Value 'payload' -Encoding UTF8
                $link = Join-Path $sandbox 'link.txt'
                $result = New-FileSymlinkOrCopy -LinkPath $link -TargetPath $target
                $result.Ok | Should -Be $true
                $result.Changed | Should -Be $true
                (@('symlink', 'copy') -contains $result.Mode) | Should -Be $true
                (Get-Content -LiteralPath $link -Raw).Trim() | Should -Be 'payload'
            } finally { Remove-TestSandbox $sandbox }
        }

        It 'is idempotent on a second call' {
            $sandbox = New-TestSandbox
            try {
                $target = Join-Path $sandbox 'source.txt'
                Set-Content -LiteralPath $target -Value 'payload' -Encoding UTF8
                $link = Join-Path $sandbox 'link.txt'
                New-FileSymlinkOrCopy -LinkPath $link -TargetPath $target | Out-Null
                $result = New-FileSymlinkOrCopy -LinkPath $link -TargetPath $target
                $result.Ok | Should -Be $true
                $result.Changed | Should -Be $false
            } finally { Remove-TestSandbox $sandbox }
        }

        It 'writes nothing on DryRun' {
            $sandbox = New-TestSandbox
            try {
                $target = Join-Path $sandbox 'source.txt'
                Set-Content -LiteralPath $target -Value 'payload' -Encoding UTF8
                $link = Join-Path $sandbox 'link.txt'
                $result = New-FileSymlinkOrCopy -LinkPath $link -TargetPath $target -DryRun
                $result.Ok | Should -Be $true
                (Test-Path -LiteralPath $link) | Should -Be $false
            } finally { Remove-TestSandbox $sandbox }
        }
    }

    Context 'Merge-OpencodeJsonc' {
        It 'creates the file when it does not exist' {
            $sandbox = New-TestSandbox
            try {
                $path = Join-Path $sandbox 'opencode.jsonc'
                $fragment = @{ mcp = @{ exa = @{ type = 'remote'; url = 'https://mcp.exa.ai/mcp' } }; theme = 'ai-transparent' }
                $result = Merge-OpencodeJsonc -Path $path -Fragment $fragment
                $result.Ok | Should -Be $true
                $result.Changed | Should -Be $true
                $result.Mode | Should -Be 'json'
                (Test-Path -LiteralPath $path) | Should -Be $true
                (ConvertFrom-Jsonc -Text (Get-Content -LiteralPath $path -Raw)).theme | Should -Be 'ai-transparent'
            } finally { Remove-TestSandbox $sandbox }
        }

        It 'preserves comments and does not clobber existing keys' {
            $sandbox = New-TestSandbox
            try {
                $path = Join-Path $sandbox 'opencode.jsonc'
                $content = @"
{
  // important comment
  "`$schema": "https://opencode.ai/config.json",
  "mcp": {
    "zoho": { "type": "local" },
    "brain": { "type": "remote" }
  }
}
"@
                Set-Content -LiteralPath $path -Value $content -Encoding UTF8
                $fragment = @{ mcp = @{ exa = @{ type = 'remote'; url = 'https://mcp.exa.ai/mcp' }; playwright = @{ type = 'local'; command = @('cmd', '/c', 'playwright-mcp') } }; theme = 'ai-transparent' }
                $result = Merge-OpencodeJsonc -Path $path -Fragment $fragment
                $result.Changed | Should -Be $true
                $raw = Get-Content -LiteralPath $path -Raw
                ([bool]($raw -match '// important comment')) | Should -Be $true
                ([bool]($raw -match '"zoho"')) | Should -Be $true
                ([bool]($raw -match '"brain"')) | Should -Be $true
                ([bool]($raw -match '"exa"')) | Should -Be $true
                ([bool]($raw -match '"playwright"')) | Should -Be $true
                $parsed = ConvertFrom-Jsonc -Text $raw
                $parsed.mcp.exa.url | Should -Be 'https://mcp.exa.ai/mcp'
                $parsed.theme | Should -Be 'ai-transparent'
            } finally { Remove-TestSandbox $sandbox }
        }

        It 'is idempotent' {
            $sandbox = New-TestSandbox
            try {
                $path = Join-Path $sandbox 'opencode.jsonc'
                Set-Content -LiteralPath $path -Value '{ "mcp": {} }' -Encoding UTF8
                $fragment = @{ mcp = @{ exa = @{ type = 'remote'; url = 'https://mcp.exa.ai/mcp' } }; theme = 'ai-transparent' }
                $first = Merge-OpencodeJsonc -Path $path -Fragment $fragment
                $first.Changed | Should -Be $true
                $second = Merge-OpencodeJsonc -Path $path -Fragment $fragment
                $second.Changed | Should -Be $false
            } finally { Remove-TestSandbox $sandbox }
        }

        It 'writes nothing on DryRun' {
            $sandbox = New-TestSandbox
            try {
                $path = Join-Path $sandbox 'opencode.jsonc'
                $content = '{ "mcp": {} }'
                Set-Content -LiteralPath $path -Value $content -Encoding UTF8
                $fragment = @{ mcp = @{ exa = @{ type = 'remote' } }; theme = 'ai-transparent' }
                $result = Merge-OpencodeJsonc -Path $path -Fragment $fragment -DryRun
                $result.Changed | Should -Be $true
                (Get-Content -LiteralPath $path -Raw).Trim() | Should -Be $content
                (Test-Path -LiteralPath ($path + '.bak')) | Should -Be $false
            } finally { Remove-TestSandbox $sandbox }
        }
    }

    Context 'Write-Log' {
        It 'does not write a journal while AidDryRun is set' {
            $sandbox = New-TestSandbox
            $previous = $script:AidDryRun
            try {
                $script:AidDryRun = $true
                $journal = Join-Path $sandbox 'install.log'
                Write-Log -Message 'hello' -Level Info -LogFile $journal
                (Test-Path -LiteralPath $journal) | Should -Be $false
            } finally {
                $script:AidDryRun = $previous
                Remove-TestSandbox $sandbox
            }
        }

        It 'writes a journal when AidDryRun is not set' {
            $sandbox = New-TestSandbox
            $previous = $script:AidDryRun
            try {
                $script:AidDryRun = $false
                $journal = Join-Path $sandbox 'install.log'
                Write-Log -Message 'hello' -Level Info -LogFile $journal
                (Test-Path -LiteralPath $journal) | Should -Be $true
            } finally {
                $script:AidDryRun = $previous
                Remove-TestSandbox $sandbox
            }
        }
    }

    Context 'Invoke-Winget' {
        It 'is a no-op when SkipDeps is set' {
            $result = Invoke-Winget -Id 'ryanoasis.CaskaydiaCove' -SkipDeps
            $result.Ok | Should -Be $true
            $result.Changed | Should -Be $false
            $result.Installed | Should -Be $false
        }

        It 'flags a planned change on DryRun when winget is available' {
            $hasWinget = [bool](Get-Command -Name winget -ErrorAction SilentlyContinue)
            $result = Invoke-Winget -Id 'Lars-Win-AI.Nonexistent.Package' -DryRun
            if ($hasWinget) {
                $result.Ok | Should -Be $true
                $result.Installed | Should -Be $false
                $result.Changed | Should -Be $true
            } else {
                $result.Ok | Should -Be $false
            }
        }
    }

    Context 'New-AidResult' {
        It 'returns the contract-shaped component result' {
            $result = New-AidResult -Component 'deps'
            $result.Component | Should -Be 'deps'
            $result.Status | Should -Be 'skipped'
            $result.Changed | Should -Be $false
            @($result.Messages).Count | Should -Be 0
            $result.ContainsKey('Messages') | Should -Be $true
        }
    }
}
