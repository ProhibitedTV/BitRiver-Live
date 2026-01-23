#!/usr/bin/env pwsh
<#
  Quickstart helper for Docker Desktop on Windows.
  Delegates to the Go CLI for environment setup and stack startup.
#>

param(
    [switch]$h,
    [switch]$help
)

$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest

function Show-Usage {
    @'
Usage: scripts/quickstart.ps1 [-h|--help]

Runs the Go-based BitRiver Live CLI to run doctor, initialize the environment,
render OME configuration, and start Docker Compose. Override ENV_FILE or
COMPOSE_FILE to point at custom locations.

Options:
  -h, --help    Show this help message.
'@
}

function Ensure-Go {
    if (-not (Get-Command go -ErrorAction SilentlyContinue)) {
        Write-Error "Go is required to run the BitRiver Live CLI. Install Go 1.21+ from https://go.dev/dl/ and ensure it is in your PATH."
    }
}

if ($h -or $help) {
    Show-Usage
    exit 0
}

Ensure-Go

$ScriptDir = Split-Path -Parent $MyInvocation.MyCommand.Definition
$RepoRoot = Resolve-Path "$ScriptDir/.."
function Invoke-Cli {
    param([string[]]$Args)
    pushd $RepoRoot | Out-Null
    try {
        $env:GOTOOLCHAIN = 'local'
        $env:GOPROXY = 'off'
        $env:GOSUMDB = 'off'
        go run ./cmd/bitriver @Args
    } finally {
        popd | Out-Null
        Remove-Item Env:GOTOOLCHAIN -ErrorAction SilentlyContinue
        Remove-Item Env:GOPROXY -ErrorAction SilentlyContinue
        Remove-Item Env:GOSUMDB -ErrorAction SilentlyContinue
    }
}

$DefaultEnvFile = Join-Path $RepoRoot '.env'
$DefaultComposeFile = Join-Path $RepoRoot 'deploy/docker-compose.yml'
$EnvFile = if ($Env:ENV_FILE) { $Env:ENV_FILE } else { $DefaultEnvFile }
$ComposeFile = if ($Env:COMPOSE_FILE) { $Env:COMPOSE_FILE } else { $DefaultComposeFile }

$envArgs = @('--env-file', $EnvFile)
$composeArgs = if ($ComposeFile -ne $DefaultComposeFile) { @('--file', $ComposeFile, 'up') } else { @('up') }

Write-Output 'Running environment doctor ...'
Invoke-Cli -Args @('doctor')

Write-Output 'Initializing environment file via Go CLI ...'
Invoke-Cli -Args @('env', 'init') + $envArgs

Write-Output 'Rendering OME configuration ...'
Invoke-Cli -Args @('ome', 'render') + $envArgs

Write-Output 'Starting Docker Compose ...'
Invoke-Cli -Args @('compose') + $composeArgs
