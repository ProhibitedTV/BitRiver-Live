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
        Write-Error "Go is required to run the BitRiver Live CLI. Install Go 1.21+ from https://go.dev/dl/ and ensure it is in your PATH."
    }
}

$ScriptDir = Split-Path -Parent $MyInvocation.MyCommand.Definition
$RepoRoot = Resolve-Path "$ScriptDir/.."
$CodeRoot = if ($Env:BITRIVER_QUICKSTART_REPO_ROOT) { Resolve-Path $Env:BITRIVER_QUICKSTART_REPO_ROOT } else { $RepoRoot }

function Invoke-Cli {
    param([string[]]$CliArgs)
    Push-Location $CodeRoot | Out-Null
    try {
        $env:GOTOOLCHAIN = 'local'
        $env:GOPROXY = 'off'
        $env:GOSUMDB = 'off'
        & go run ./cmd/bitriver @CliArgs
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
    Invoke-Cli -CliArgs @('quickstart', '--help')
    exit 0
}

Write-Output 'Running BitRiver Live quickstart ...'
$argsToForward = @('quickstart') + $QuickstartArgs
Invoke-Cli -CliArgs $argsToForward
