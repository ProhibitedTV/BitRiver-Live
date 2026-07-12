#!/usr/bin/env pwsh
<#
  Thin verify wrapper for PowerShell.
  Delegates to the canonical Bash verify gate after finding a usable Bash.
#>

param(
    [Alias('h')]
    [switch]$Help,
    [Parameter(ValueFromRemainingArguments = $true)]
    [string[]]$VerifyArgs
)

$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest

function Show-Usage {
    @'
Usage: scripts/verify.ps1 [verify flags...]

PowerShell wrapper around the canonical repository verification gate:
  ./scripts/verify.sh

Options are forwarded to verify.sh. Common examples:
  .\scripts\verify.ps1
  .\scripts\verify.ps1 --viewer
  .\scripts\verify.ps1 --ci-viewer
  .\scripts\verify.ps1 --go-packages ./scripts

The wrapper prefers Git for Windows Bash, then falls back to bash on PATH. This
avoids using Windows' WSL bash.exe when WSL is installed but has no default
distro configured.
'@
}

function Add-Candidate {
    param(
        [System.Collections.Generic.List[string]]$Candidates,
        [string]$Path
    )

    if ([string]::IsNullOrWhiteSpace($Path)) {
        return
    }
    if (-not $Candidates.Contains($Path)) {
        $Candidates.Add($Path)
    }
}

function Get-BashCandidates {
    $candidates = [System.Collections.Generic.List[string]]::new()

    Add-Candidate $candidates $env:BITRIVER_VERIFY_BASH

    $programFiles = [System.Environment]::GetEnvironmentVariable('ProgramFiles')
    if (-not [string]::IsNullOrWhiteSpace($programFiles)) {
        Add-Candidate $candidates (Join-Path $programFiles 'Git\usr\bin\bash.exe')
        Add-Candidate $candidates (Join-Path $programFiles 'Git\bin\bash.exe')
    }

    $programFilesX86 = [System.Environment]::GetEnvironmentVariable('ProgramFiles(x86)')
    if (-not [string]::IsNullOrWhiteSpace($programFilesX86)) {
        Add-Candidate $candidates (Join-Path $programFilesX86 'Git\usr\bin\bash.exe')
        Add-Candidate $candidates (Join-Path $programFilesX86 'Git\bin\bash.exe')
    }

    $localAppData = [System.Environment]::GetEnvironmentVariable('LocalAppData')
    if (-not [string]::IsNullOrWhiteSpace($localAppData)) {
        Add-Candidate $candidates (Join-Path $localAppData 'Programs\Git\usr\bin\bash.exe')
        Add-Candidate $candidates (Join-Path $localAppData 'Programs\Git\bin\bash.exe')
    }

    $pathBash = Get-Command bash -ErrorAction SilentlyContinue
    if ($null -ne $pathBash) {
        Add-Candidate $candidates $pathBash.Source
    }

    return $candidates.ToArray()
}

function Test-BashCandidate {
    param([string]$Candidate)

    if ([string]::IsNullOrWhiteSpace($Candidate)) {
        return $false
    }
    if (-not (Test-Path -LiteralPath $Candidate -PathType Leaf)) {
        return $false
    }

    try {
        $output = & $Candidate -lc 'command -v dirname >/dev/null && printf ok' 2>&1
        return ($LASTEXITCODE -eq 0 -and (($output | Out-String).Trim() -eq 'ok'))
    } catch {
        return $false
    }
}

function Find-Bash {
    $checked = [System.Collections.Generic.List[string]]::new()
    foreach ($candidate in Get-BashCandidates) {
        $checked.Add($candidate)
        if (Test-BashCandidate $candidate) {
            return $candidate
        }
    }

    $checkedList = if ($checked.Count -gt 0) {
        ($checked | ForEach-Object { "  - $_" }) -join [System.Environment]::NewLine
    } else {
        '  - no candidates found'
    }

    throw @"
No usable Bash was found for the canonical verify gate.

Install Git for Windows and re-run:
  .\scripts\verify.ps1

If Windows bash.exe reports WSL_E_DEFAULT_DISTRO_NOT_FOUND, install a WSL distro
or run this wrapper after installing Git for Windows so it can use Git Bash.

Checked:
$checkedList
"@
}

if ($Help -or ($VerifyArgs.Count -gt 0 -and ($VerifyArgs[0] -in @('-h', '--help')))) {
    Show-Usage
    exit 0
}

$scriptDir = Split-Path -Parent $MyInvocation.MyCommand.Definition
$repoRoot = Resolve-Path "$scriptDir/.."
$bashPath = Find-Bash

Push-Location $repoRoot | Out-Null
try {
    $bashArgs = @('-lc', 'exec ./scripts/verify.sh "$@"', 'verify.sh') + $VerifyArgs
    & $bashPath @bashArgs
    exit $LASTEXITCODE
} finally {
    Pop-Location | Out-Null
}
