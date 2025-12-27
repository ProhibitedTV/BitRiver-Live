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
$TemplateFile = Join-Path $RepoRoot 'deploy/.env.example'
$EnvPreexisting = Test-Path $EnvFile

if ($EnvFile -ne $DefaultEnvFile) {
    $envDir = Split-Path -Parent $EnvFile
    if ($envDir) { New-Item -ItemType Directory -Path $envDir -Force | Out-Null }
}

function New-Secret {
    param([int]$Length = 32)

    $chars = 'abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789'
    $buffer = New-Object byte[] $Length
    [System.Security.Cryptography.RandomNumberGenerator]::Fill($buffer)

    $sb = New-Object System.Text.StringBuilder
    foreach ($b in $buffer) { $null = $sb.Append($chars[$b % $chars.Length]) }
    $sb.ToString()
}

function Ensure-KeyValue {
    param(
        [string]$Path,
        [string]$Key,
        [string]$Value,
        [bool]$Force = $false
    )

    $lines = @()
    if (Test-Path $Path) {
        $lines = Get-Content -Path $Path
    }

    $pattern = "^$([regex]::Escape("$Key="))"
    $index = -1
    for ($i = 0; $i -lt $lines.Count; $i++) {
        if ($lines[$i] -match $pattern) {
            $index = $i
            break
        }
    }

    if ($index -ge 0) {
        if ($Force) {
            $lines[$index] = "$Key=$Value"
            Set-Content -Path $Path -Value $lines
        }
    } else {
        Add-Content -Path $Path -Value "$Key=$Value"
    }
}

function Reconcile-EnvFile {
    param(
        [string]$EnvFilePath,
        [string]$TemplatePath,
        [bool]$EnvExisted
    )

    if (-not (Test-Path $TemplatePath)) {
        throw "Template missing at $TemplatePath"
    }

    if (-not (Test-Path $EnvFilePath)) {
        $envDir = Split-Path -Parent $EnvFilePath
        if ($envDir) { New-Item -ItemType Directory -Path $envDir -Force | Out-Null }
        Copy-Item -Path $TemplatePath -Destination $EnvFilePath -Force
        Write-Output "Created environment file at $EnvFilePath from $TemplatePath"
    }

    foreach ($line in Get-Content -Path $TemplatePath) {
        if ([string]::IsNullOrWhiteSpace($line) -or $line.Trim().StartsWith('#')) { continue }
        $parts = $line.Split('=', 2)
        if ($parts.Count -ne 2) { continue }
        $key = $parts[0]
        if (-not (Select-String -Path $EnvFilePath -Pattern "^$([regex]::Escape($key))=" -Quiet)) {
            Add-Content -Path $EnvFilePath -Value $line
        }
    }

    $forceDefaults = -not $EnvExisted
    Ensure-KeyValue -Path $EnvFilePath -Key 'BITRIVER_LIVE_IMAGE_TAG' -Value 'latest' -Force:$forceDefaults
    Ensure-KeyValue -Path $EnvFilePath -Key 'BITRIVER_LIVE_CHAT_QUEUE_REDIS_PASSWORD' -Value 'bitriver' -Force:$forceDefaults

    if (-not $EnvExisted) {
        $redisPassword = New-Secret -Length 24

        Ensure-KeyValue -Path $EnvFilePath -Key 'BITRIVER_POSTGRES_PASSWORD' -Value (New-Secret -Length 24) -Force:$true
        Ensure-KeyValue -Path $EnvFilePath -Key 'BITRIVER_REDIS_PASSWORD' -Value $redisPassword -Force:$true
        Ensure-KeyValue -Path $EnvFilePath -Key 'BITRIVER_LIVE_CHAT_QUEUE_REDIS_PASSWORD' -Value $redisPassword -Force:$true
        Ensure-KeyValue -Path $EnvFilePath -Key 'BITRIVER_LIVE_ADMIN_PASSWORD' -Value (New-Secret -Length 28) -Force:$true
        Ensure-KeyValue -Path $EnvFilePath -Key 'BITRIVER_SRS_TOKEN' -Value (New-Secret -Length 32) -Force:$true

        $omePassword = New-Secret -Length 28
        $omeToken = New-Secret -Length 40
        Ensure-KeyValue -Path $EnvFilePath -Key 'BITRIVER_OME_PASSWORD' -Value $omePassword -Force:$true
        Ensure-KeyValue -Path $EnvFilePath -Key 'BITRIVER_OME_API_TOKEN' -Value $omeToken -Force:$true
        Ensure-KeyValue -Path $EnvFilePath -Key 'BITRIVER_OME_ACCESS_TOKEN' -Value $omeToken -Force:$true

        Ensure-KeyValue -Path $EnvFilePath -Key 'BITRIVER_TRANSCODER_TOKEN' -Value (New-Secret -Length 40) -Force:$true
    }
}

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
Invoke-Cli -Args @('env', 'init', '--env-file', $EnvFile)
Reconcile-EnvFile -EnvFilePath $EnvFile -TemplatePath $TemplateFile -EnvExisted $EnvPreexisting

Write-Output 'Rendering OME configuration ...'
Invoke-Cli -Args @('ome', 'render', '--env-file', $EnvFile)

Write-Output 'Starting Docker Compose ...'
Invoke-Cli -Args @('compose', '--file', $ComposeFile, 'up')
