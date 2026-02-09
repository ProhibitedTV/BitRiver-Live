#!/usr/bin/env pwsh
<#
  Quickstart helper for Docker Desktop on Windows.
  Delegates to the Go CLI for environment setup and stack startup.
#>

param(
    [switch]$h,
    [switch]$help,
    [switch]$ValidateOnly
)

$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest

function Show-Usage {
    @'
Usage: scripts/quickstart.ps1 [-h|--help] [-ValidateOnly]

Runs the Go-based BitRiver Live CLI quickstart command to run doctor, initialize
the environment, render OME configuration, start Docker Compose, wait for the
API readiness probe, and seed the admin user. Override ENV_FILE or COMPOSE_FILE
to point at custom locations.

Options:
  -h, --help      Show this help message.
  -ValidateOnly   Validate script entrypoint wiring without Docker Desktop orchestration.
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
        'C:\Program Files (x86)\Docker\Docker\Docker Desktop.exe'
    )
    foreach ($candidate in $candidates) {
        if ($candidate -and (Test-Path $candidate)) {
            return $candidate
        }
    }
    return $null
}

function Test-DockerDesktopEnginePipe {
    return (Test-Path '\\.\pipe\dockerDesktopLinuxEngine') -or (Test-Path '\\.\pipe\dockerDesktopWindowsEngine')
}

function Test-DockerCliReady {
    $output = ''
    try {
        $output = docker version 2>&1
        if ($LASTEXITCODE -ne 0) {
            return $false
        }
    } catch {
        return $false
    }

    if (-not $output) {
        return $false
    }

    if ($output -match 'Internal Server Error' -or $output -match '(?i)pipe') {
        return $false
    }

    return $output -match '(?m)^\s*Server:'
}

function Test-DockerDesktopReady {
    return (Test-DockerDesktopEnginePipe) -and (Test-DockerCliReady)
}

function Wait-ForDocker {
    param([int]$TimeoutSeconds = 210)
    $stopwatch = [System.Diagnostics.Stopwatch]::StartNew()
    while ($stopwatch.Elapsed.TotalSeconds -lt $TimeoutSeconds) {
        if (Test-DockerDesktopReady) {
            return $true
        }
        Write-Output 'Waiting for Docker Desktop...'
        Start-Sleep -Seconds 2
    }
    return $false
}

function Ensure-DockerDesktopRunning {
    if (Test-DockerDesktopReady) {
        return
    }

    $desktopPath = Get-DockerDesktopPath
    if (-not $desktopPath) {
        Write-Error "Docker Desktop is not running and the Docker Desktop executable was not found. Install Docker Desktop from https://www.docker.com/products/docker-desktop/ and try again."
    }

    $desktopProcess = Get-Process "Docker Desktop" -ErrorAction SilentlyContinue
    if (-not $desktopProcess) {
        Write-Output "Docker Desktop process not detected. Starting Docker Desktop..."
        Start-Process -FilePath $desktopPath | Out-Null
    } elseif (-not (Test-DockerDesktopEnginePipe)) {
        Write-Output "Docker Desktop engine pipe not detected yet. Waiting for startup..."
    }

    if (-not (Wait-ForDocker -TimeoutSeconds 210)) {
        Write-Error "Docker Desktop did not start; open it manually and retry."
    }
}

function Invoke-OmeAuthPreflight {
    param(
        [Parameter(Mandatory = $true)][string]$EnvFilePath,
        [Parameter(Mandatory = $true)][string]$CodeRootPath,
        [Parameter(Mandatory = $true)][string]$VerifyScriptPath
    )

    if (-not (Test-Path -LiteralPath $EnvFilePath -PathType Leaf)) {
        Write-Error "OME auth preflight failed: env file not found at $EnvFilePath"
    }

    if (-not (Test-Path -LiteralPath $VerifyScriptPath -PathType Leaf)) {
        Write-Error "OME auth preflight failed: helper script is missing at $VerifyScriptPath"
    }

    Write-Output 'Running OME auth preflight: rendering config and validating token consistency ...'
    Invoke-Cli -CliArgs @('ome', 'render', '--force', '--env-file', $EnvFilePath)

    $configPath = Join-Path $CodeRootPath 'deploy/ome/Server.generated.xml'
    $bash = Get-Command bash -ErrorAction SilentlyContinue
    if ($bash) {
        & $bash.Source $VerifyScriptPath '--env-file' $EnvFilePath '--config' $configPath
        return
    }

    Invoke-Cli -CliArgs @('ome', 'verify-health-token', '--env-file', $EnvFilePath, '--config', $configPath)
}

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
$VerifyOmeTokenScript = Join-Path $CodeRoot 'scripts/verify-ome-health-token.sh'


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

Ensure-DockerDesktopRunning

$quickstartArgs = @('quickstart', '--env-file', $EnvFile, '--compose-file', $ComposeFile)

Write-Output 'Running BitRiver Live quickstart ...'
Invoke-OmeAuthPreflight -EnvFilePath $EnvFile -CodeRootPath $CodeRoot -VerifyScriptPath $VerifyOmeTokenScript
Invoke-Cli -CliArgs $quickstartArgs
