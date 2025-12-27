#!/usr/bin/env pwsh
<#!
  Quickstart helper for Docker Desktop on Windows.
  Delegates to the Go CLI for environment setup and stack startup.
!>

$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest

function Show-Usage {
    @'
Usage: scripts/quickstart.ps1 [-h|--help]

Runs the Go-based BitRiver Live CLI to initialize the environment, render OME
configuration, and start Docker Compose. Override ENV_FILE or COMPOSE_FILE to
point at custom locations.

Options:
  -h, --help    Show this help message.
'@
}

function Ensure-Go {
    if (-not (Get-Command go -ErrorAction SilentlyContinue)) {
        Write-Error "Go is required to run the BitRiver Live CLI. Install Go 1.21+ from https://go.dev/dl/ and ensure it is in your PATH."
    }
}

param(
    [switch]$h,
    [switch]$help
)

if ($h -or $help) {
    Show-Usage
    exit 0
}

Ensure-Go

$ScriptDir = Split-Path -Parent $MyInvocation.MyCommand.Definition
$RepoRoot = Resolve-Path "$ScriptDir/.."
$DefaultEnvFile = Join-Path $RepoRoot '.env'
$EnvFile = if ($Env:ENV_FILE) { $Env:ENV_FILE } else { $DefaultEnvFile }
$ComposeFile = if ($Env:COMPOSE_FILE) { $Env:COMPOSE_FILE } else { Join-Path $RepoRoot 'deploy/docker-compose.yml' }

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

Write-Output 'Initializing environment file via Go CLI ...'
Invoke-Cli -Args @('env', 'init')

if ($EnvFile -ne $DefaultEnvFile) {
    $envDir = Split-Path -Parent $EnvFile
    if ($envDir) { New-Item -ItemType Directory -Path $envDir -Force | Out-Null }
    Copy-Item -Path $DefaultEnvFile -Destination $EnvFile -Force
    Write-Output "Copied generated .env to $EnvFile"
}

Write-Output 'Rendering OME configuration ...'
Invoke-Cli -Args @('ome', 'render')

Write-Output 'Starting Docker Compose ...'
Invoke-Cli -Args @('compose', '--file', $ComposeFile, 'up')
