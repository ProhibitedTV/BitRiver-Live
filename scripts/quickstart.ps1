#!/usr/bin/env pwsh
<#!
  Thin quickstart wrapper for PowerShell.
  Delegates directly to the Go CLI quickstart entrypoint.
#>

param(
    [switch]$h,
    [switch]$help,
    [switch]$ValidateOnly,
    [Parameter(ValueFromRemainingArguments = $true)]
    [string[]]$QuickstartArgs
)

$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest

function Show-Usage {
    @'
Usage: scripts/quickstart.ps1 [quickstart flags...]

Thin wrapper around the Go BitRiver CLI quickstart flow.
It performs minimal local checks, then forwards all arguments to:
  go run ./cmd/bitriver quickstart ...

Options:
  -h, --help      Show this help message.
  -ValidateOnly   Validate script entrypoint wiring by invoking quickstart help.

Examples:
  scripts/quickstart.ps1
  scripts/quickstart.ps1 --env-file .env.prod --compose-file deploy/docker-compose.yml
'@
}

function Ensure-Go {
    if (-not (Get-Command go -ErrorAction SilentlyContinue)) {
        Write-Error "Go 1.26 or newer is required to run the BitRiver Live CLI. Install it from https://go.dev/dl/ and ensure it is in your PATH."
    }
    $version = (& go env GOVERSION).Trim()
    if ($version -notmatch 'go(?<major>\d+)\.(?<minor>\d+)') {
        Write-Error "Unable to determine Go version from: $version"
    }
    $major = [int]$Matches.major
    $minor = [int]$Matches.minor
    if ($major -lt 1 -or ($major -eq 1 -and $minor -lt 26)) {
        Write-Error "Go 1.26 or newer is required to run the BitRiver Live CLI (found $version)."
    }
}

$ScriptDir = Split-Path -Parent $MyInvocation.MyCommand.Definition
$RepoRoot = Resolve-Path "$ScriptDir/.."
$CodeRoot = if ($Env:BITRIVER_QUICKSTART_REPO_ROOT) { Resolve-Path $Env:BITRIVER_QUICKSTART_REPO_ROOT } else { $RepoRoot }

function Invoke-Cli {
    param(
        [string[]]$CliArgs,
        [switch]$AllowHelpExitCode
    )
    Push-Location $CodeRoot | Out-Null
    try {
        $env:GOTOOLCHAIN = 'local'
        $env:GOPROXY = 'off'
        $env:GOSUMDB = 'off'
        $stdoutPath = [System.IO.Path]::GetTempFileName()
        $stderrPath = [System.IO.Path]::GetTempFileName()
        $goPath = (Get-Command go -ErrorAction Stop).Source
        $processPath = [System.Environment]::GetEnvironmentVariable('Path', 'Process')
        $processPATH = [System.Environment]::GetEnvironmentVariable('PATH', 'Process')
        $processGOCACHE = [System.Environment]::GetEnvironmentVariable('GOCACHE', 'Process')
        $goCacheRoot = if (-not [string]::IsNullOrWhiteSpace($processGOCACHE)) {
            $processGOCACHE
        } else {
            Join-Path ([System.IO.Path]::GetTempPath()) 'bitriver-live-go-build-cache'
        }
        $normalizedPath = if (-not [string]::IsNullOrEmpty($processPath)) { $processPath } else { $processPATH }
        try {
            New-Item -ItemType Directory -Force -Path $goCacheRoot | Out-Null
            if ($null -ne $normalizedPath) {
                [System.Environment]::SetEnvironmentVariable('Path', $normalizedPath, 'Process')
            }
            if ($null -ne $processPATH) {
                [System.Environment]::SetEnvironmentVariable('PATH', $null, 'Process')
            }
            [System.Environment]::SetEnvironmentVariable('GOCACHE', $goCacheRoot, 'Process')
            $goArgs = @('run', './cmd/bitriver') + $CliArgs
            $process = Start-Process -FilePath $goPath `
                -ArgumentList $goArgs `
                -WorkingDirectory $CodeRoot `
                -NoNewWindow `
                -PassThru `
                -Wait `
                -RedirectStandardOutput $stdoutPath `
                -RedirectStandardError $stderrPath
            $stdoutLines = if ((Get-Item $stdoutPath).Length -gt 0) { Get-Content $stdoutPath } else { @() }
            $stderrLines = if ((Get-Item $stderrPath).Length -gt 0) { Get-Content $stderrPath } else { @() }
        } finally {
            [System.Environment]::SetEnvironmentVariable('Path', $processPath, 'Process')
            [System.Environment]::SetEnvironmentVariable('PATH', $processPATH, 'Process')
            [System.Environment]::SetEnvironmentVariable('GOCACHE', $processGOCACHE, 'Process')
            Remove-Item $stdoutPath, $stderrPath -ErrorAction SilentlyContinue
        }
        $exitCode = $process.ExitCode
        $cliOutput = @($stdoutLines) + @($stderrLines)
        if ($cliOutput.Count -gt 0) {
            $cliOutput | ForEach-Object { Write-Host $_ }
        }

        $helpRequested = $false
        if ($AllowHelpExitCode) {
            $helpRequested = ($cliOutput | Out-String) -match 'flag: help requested'
        }
        if ($exitCode -ne 0 -and -not $helpRequested) {
            return $exitCode
        }
        return 0
    } finally {
        Pop-Location | Out-Null
        Remove-Item Env:GOTOOLCHAIN -ErrorAction SilentlyContinue
        Remove-Item Env:GOPROXY -ErrorAction SilentlyContinue
        Remove-Item Env:GOSUMDB -ErrorAction SilentlyContinue
    }
}

if ($h -or $help) {
    Show-Usage
    exit 0
}

Ensure-Go

if ($ValidateOnly) {
    Write-Output 'Running BitRiver Live quickstart entrypoint validation (no Docker orchestration) ...'
    $exitCode = Invoke-Cli -CliArgs @('quickstart', '--help') -AllowHelpExitCode
    if ($exitCode -ne 0) {
        exit $exitCode
    }
    exit 0
}

Write-Output 'Running BitRiver Live quickstart ...'
$argsToForward = @('quickstart') + $QuickstartArgs
$exitCode = Invoke-Cli -CliArgs $argsToForward
if ($exitCode -ne 0) {
    exit $exitCode
}
