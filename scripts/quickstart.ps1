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

Runs the Go-based BitRiver Live CLI quickstart command to run doctor, initialize
the environment, render OME configuration, start Docker Compose, wait for the
API readiness probe, and seed the admin user. Override ENV_FILE or COMPOSE_FILE
to point at custom locations.

Options:
  -h, --help    Show this help message.
'@
}

function Ensure-Go {
    if (-not (Get-Command go -ErrorAction SilentlyContinue)) {
        Write-Error "Go is required to run the BitRiver Live CLI. Install Go 1.21+ from https://go.dev/dl/ and ensure it is in your PATH."
    }
}

function Get-DockerDesktopPath {
    $candidates = @(
        'C:\Program Files\Docker\Docker\Docker Desktop.exe',
        'C:\Program Files (x86)\Docker\Docker\Docker Desktop.exe',
        "$Env:ProgramFiles\Docker\Docker\Docker Desktop.exe",
        "$Env:ProgramFiles(x86)\Docker\Docker\Docker Desktop.exe"
    )
    foreach ($candidate in $candidates) {
        if ($candidate -and (Test-Path $candidate)) {
            return $candidate
        }
    }
    return $null
}

function Test-DockerEnginePipe {
    return Test-Path '\\.\pipe\docker_engine'
}

function Wait-ForDocker {
    param([int]$TimeoutSeconds = 120)
    $stopwatch = [System.Diagnostics.Stopwatch]::StartNew()
    while ($stopwatch.Elapsed.TotalSeconds -lt $TimeoutSeconds) {
        try {
            docker version | Out-Null
            return $true
        } catch {
            Write-Output 'Waiting for Docker Desktop to start...'
            Start-Sleep -Seconds 3
        }
    }
    return $false
}

function Ensure-DockerDesktopRunning {
    if (Wait-ForDocker -TimeoutSeconds 5) {
        return
    }

    $desktopPath = Get-DockerDesktopPath
    if (-not $desktopPath) {
        Write-Error "Docker Desktop is not running and the Docker Desktop executable was not found. Install Docker Desktop from https://www.docker.com/products/docker-desktop/ and try again."
    }

    Write-Output "Docker engine pipe not detected. Starting Docker Desktop..."
    Start-Process -FilePath $desktopPath | Out-Null

    if (-not (Wait-ForDocker -TimeoutSeconds 120)) {
        Write-Error "Docker Desktop did not become ready within 120 seconds. Please start Docker Desktop manually, wait for it to finish initializing, and re-run this script."
    }
}

if ($h -or $help) {
    Show-Usage
    exit 0
}

Ensure-Go
Ensure-DockerDesktopRunning

$ScriptDir = Split-Path -Parent $MyInvocation.MyCommand.Definition
$RepoRoot = Resolve-Path "$ScriptDir/.."
$CodeRoot = if ($Env:BITRIVER_QUICKSTART_REPO_ROOT) { Resolve-Path $Env:BITRIVER_QUICKSTART_REPO_ROOT } else { $RepoRoot }
function Invoke-Cli {
    param([string[]]$CliArgs)
    pushd $CodeRoot | Out-Null
    try {
        $env:GOTOOLCHAIN = 'local'
        $env:GOPROXY = 'off'
        $env:GOSUMDB = 'off'
        go run ./cmd/bitriver @CliArgs
    } finally {
        popd | Out-Null
        Remove-Item Env:GOTOOLCHAIN -ErrorAction SilentlyContinue
        Remove-Item Env:GOPROXY -ErrorAction SilentlyContinue
        Remove-Item Env:GOSUMDB -ErrorAction SilentlyContinue
    }
}

$DefaultEnvFile = Join-Path $CodeRoot '.env'
$DefaultComposeFile = Join-Path $CodeRoot 'deploy/docker-compose.yml'
$EnvFile = if ($Env:ENV_FILE) { $Env:ENV_FILE } else { $DefaultEnvFile }
$ComposeFile = if ($Env:COMPOSE_FILE) { $Env:COMPOSE_FILE } else { $DefaultComposeFile }

$quickstartArgs = @('quickstart', '--env-file', $EnvFile, '--compose-file', $ComposeFile)

Write-Output 'Running BitRiver Live quickstart ...'
Invoke-Cli -CliArgs $quickstartArgs
