#!/usr/bin/env pwsh

[CmdletBinding()]
param(
    [string]$EnvFile = '.env',
    [string]$ComposeFile = 'deploy/docker-compose.yml',
    [switch]$Start,
    [ValidateRange(30, 900)]
    [int]$TimeoutSeconds = 240
)

$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest

$ScriptDir = Split-Path -Parent $MyInvocation.MyCommand.Definition
$RepoRoot = (Resolve-Path (Join-Path $ScriptDir '..')).Path

function Resolve-RepoPath {
    param([Parameter(Mandatory = $true)][string]$Path)

    if ([System.IO.Path]::IsPathRooted($Path)) {
        return $Path
    }
    return Join-Path $RepoRoot $Path
}

function Invoke-CheckedCommand {
    param(
        [Parameter(Mandatory = $true)][string]$FilePath,
        [Parameter(Mandatory = $true)][string[]]$Arguments,
        [Parameter(Mandatory = $true)][string]$FailureMessage
    )

    $output = @(& $FilePath @Arguments 2>&1)
    if ($LASTEXITCODE -ne 0) {
        $detail = ($output | Out-String).Trim()
        if ($detail) {
            throw "$FailureMessage`n$detail"
        }
        throw $FailureMessage
    }
    return $output
}

function Get-EnvValue {
    param(
        [Parameter(Mandatory = $true)][string]$Path,
        [Parameter(Mandatory = $true)][string]$Key
    )

    $pattern = "^\s*$([regex]::Escape($Key))\s*=\s*(.*)$"
    foreach ($line in Get-Content -LiteralPath $Path) {
        if ($line -match '^\s*#') {
            continue
        }
        if ($line -match $pattern) {
            return $Matches[1].Trim().Trim('"').Trim("'")
        }
    }
    return $null
}

function Set-EnvValue {
    param(
        [Parameter(Mandatory = $true)][string]$Path,
        [Parameter(Mandatory = $true)][string]$Key,
        [Parameter(Mandatory = $true)][AllowEmptyString()][string]$Value
    )

    $lines = [System.Collections.Generic.List[string]]::new()
    $found = $false
    $pattern = "^\s*$([regex]::Escape($Key))\s*="
    foreach ($line in Get-Content -LiteralPath $Path) {
        if ($line -match $pattern) {
            $lines.Add("$Key=$Value")
            $found = $true
        } else {
            $lines.Add($line)
        }
    }
    if (-not $found) {
        $lines.Add("$Key=$Value")
    }
    [System.IO.File]::WriteAllLines($Path, $lines, [System.Text.UTF8Encoding]::new($false))
}

function New-EvaluationEnvFile {
    param([Parameter(Mandatory = $true)][string]$SourcePath)

    $tempPath = Join-Path ([System.IO.Path]::GetTempPath()) "bitriver-windows-proof-$([guid]::NewGuid().ToString('N')).env"
    Copy-Item -LiteralPath $SourcePath -Destination $tempPath
    Set-EnvValue -Path $tempPath -Key 'BITRIVER_DEPLOY_IMAGE_SOURCE' -Value 'build'
    foreach ($key in @(
        'BITRIVER_LIVE_IMAGE_DIGEST',
        'BITRIVER_VIEWER_IMAGE_DIGEST',
        'BITRIVER_SRS_CONTROLLER_IMAGE_DIGEST',
        'BITRIVER_TRANSCODER_IMAGE_DIGEST'
    )) {
        Set-EnvValue -Path $tempPath -Key $key -Value ''
    }
    Set-EnvValue -Path $tempPath -Key 'BITRIVER_SRS_PUBLIC_RTMP_BASE_URL' -Value 'rtmp://localhost:1935/live'
    Set-EnvValue -Path $tempPath -Key 'BITRIVER_OME_PUBLIC_LLHLS_BASE_URL' -Value 'http://localhost:8080/live'
    return $tempPath
}

function Wait-HttpEndpoint {
    param(
        [Parameter(Mandatory = $true)][string]$Url,
        [Parameter(Mandatory = $true)][int]$Timeout
    )

    $deadline = [DateTime]::UtcNow.AddSeconds($Timeout)
    $lastError = 'no response'
    while ([DateTime]::UtcNow -lt $deadline) {
        try {
            $response = Invoke-WebRequest -Uri $Url -UseBasicParsing -TimeoutSec 10
            if ($response.StatusCode -ge 200 -and $response.StatusCode -lt 400) {
                Write-Host "  PASS $Url ($($response.StatusCode))"
                return
            }
            $lastError = "HTTP $($response.StatusCode)"
        } catch {
            $lastError = $_.Exception.Message
        }
        Start-Sleep -Seconds 2
    }
    throw "Timed out waiting for $Url after ${Timeout}s (last result: $lastError)."
}

if ($env:OS -ne 'Windows_NT') {
    throw 'This proof script is for Windows hosts. Use ./scripts/verify.sh on Linux or macOS.'
}

if (-not (Get-Command docker -ErrorAction SilentlyContinue)) {
    throw 'Docker CLI was not found. Install Docker Desktop for Windows and reopen PowerShell.'
}

$envPath = Resolve-RepoPath $EnvFile
$composePath = Resolve-RepoPath $ComposeFile
if (-not (Test-Path -LiteralPath $envPath -PathType Leaf)) {
    throw "Environment file not found at $envPath. Copy deploy/.env.example to .env and run the env-init command from docs/quickstart.md."
}
if (-not (Test-Path -LiteralPath $composePath -PathType Leaf)) {
    throw "Compose file not found at $composePath."
}

Write-Host 'Checking Docker Desktop for Windows...'
$serverOutput = @(Invoke-CheckedCommand -FilePath 'docker' -Arguments @('version', '--format', '{{.Server.Os}}|{{.Server.Arch}}|{{.Server.Version}}') -FailureMessage 'Docker Desktop is installed but its engine is unreachable. Start Docker Desktop, wait for Engine running, and retry from a shell allowed to access the Docker named pipe.')
$server = $serverOutput[0].ToString().Trim()
$serverParts = $server.Split('|')
if ($serverParts.Count -ne 3 -or $serverParts[0] -ne 'linux') {
    throw "BitRiver Live requires Docker Desktop Linux containers; detected server '$server'. Switch Docker Desktop to Linux containers and retry."
}

$engineOutput = @(Invoke-CheckedCommand -FilePath 'docker' -Arguments @('info', '--format', '{{.OperatingSystem}}|{{.OSType}}|{{.Architecture}}') -FailureMessage 'Unable to inspect the Docker engine.')
$engine = $engineOutput[0].ToString().Trim()
$engineParts = $engine.Split('|')
if ($engineParts.Count -ne 3 -or $engineParts[1] -ne 'linux') {
    throw "Expected a Linux Docker engine, detected '$engine'."
}
if ($engineParts[0] -notmatch 'Docker Desktop') {
    throw "Expected Docker Desktop, detected '$($engineParts[0])'."
}

$contextOutput = @(Invoke-CheckedCommand -FilePath 'docker' -Arguments @('context', 'show') -FailureMessage 'Unable to read the active Docker context.')
$context = $contextOutput[0].ToString().Trim()
$composeVersionOutput = @(Invoke-CheckedCommand -FilePath 'docker' -Arguments @('compose', 'version', '--short') -FailureMessage 'Docker Compose V2 is unavailable. Enable the Docker Desktop Compose plugin and retry.')
$composeVersion = $composeVersionOutput[0].ToString().Trim()

Write-Host 'Validating the canonical Compose contract...'
$previousPublicRTMP = [System.Environment]::GetEnvironmentVariable('BITRIVER_SRS_PUBLIC_RTMP_BASE_URL', 'Process')
$previousPublicLLHLS = [System.Environment]::GetEnvironmentVariable('BITRIVER_OME_PUBLIC_LLHLS_BASE_URL', 'Process')
try {
    if ($Start) {
        [System.Environment]::SetEnvironmentVariable('BITRIVER_SRS_PUBLIC_RTMP_BASE_URL', 'rtmp://localhost:1935/live', 'Process')
        [System.Environment]::SetEnvironmentVariable('BITRIVER_OME_PUBLIC_LLHLS_BASE_URL', 'http://localhost:8080/live', 'Process')
    }
    Invoke-CheckedCommand -FilePath 'docker' -Arguments @('compose', '--env-file', $envPath, '-f', $composePath, 'config', '--quiet') -FailureMessage 'The canonical Docker Compose contract did not render with the selected .env file.' | Out-Null
} finally {
    [System.Environment]::SetEnvironmentVariable('BITRIVER_SRS_PUBLIC_RTMP_BASE_URL', $previousPublicRTMP, 'Process')
    [System.Environment]::SetEnvironmentVariable('BITRIVER_OME_PUBLIC_LLHLS_BASE_URL', $previousPublicLLHLS, 'Process')
}

Write-Host 'Docker Desktop proof: PASS'
Write-Host "  Context: $context"
Write-Host "  Engine: $($engineParts[0]) ($($serverParts[0])/$($serverParts[1]), $($serverParts[2]))"
Write-Host "  Compose: $composeVersion"
Write-Host "  Contract: $composePath"

if (-not $Start) {
    Write-Host 'Run this command again with -Start to build the source checkout and verify the live HTTP routes.'
    exit 0
}

if (-not (Get-Command go -ErrorAction SilentlyContinue)) {
    throw 'Go 1.26 or newer is required for source-checkout startup. Install it from https://go.dev/dl/ and retry.'
}
$previousToolchain = [System.Environment]::GetEnvironmentVariable('GOTOOLCHAIN', 'Process')
try {
    [System.Environment]::SetEnvironmentVariable('GOTOOLCHAIN', 'local', 'Process')
    $goVersion = (& go env GOVERSION).Trim()
} finally {
    [System.Environment]::SetEnvironmentVariable('GOTOOLCHAIN', $previousToolchain, 'Process')
}
if ($LASTEXITCODE -ne 0 -or $goVersion -notmatch '^go(?<major>\d+)\.(?<minor>\d+)') {
    throw "Unable to determine the installed Go version (reported '$goVersion')."
}
$goMajor = [int]$Matches.major
$goMinor = [int]$Matches.minor
if ($goMajor -lt 1 -or ($goMajor -eq 1 -and $goMinor -lt 26)) {
    throw "Go 1.26 or newer is required for source-checkout startup; found $goVersion."
}

$quickstartPath = Join-Path $ScriptDir 'quickstart.ps1'
$runtimeEnvPath = New-EvaluationEnvFile -SourcePath $envPath
$previousMode = [System.Environment]::GetEnvironmentVariable('BITRIVER_LIVE_MODE', 'Process')
try {
    [System.Environment]::SetEnvironmentVariable('BITRIVER_LIVE_MODE', 'development', 'Process')
    Write-Host 'Starting the canonical source-build stack...'
    & $quickstartPath --env-file $runtimeEnvPath --compose-file $composePath --image-source build
    if ($LASTEXITCODE -ne 0) {
        throw "BitRiver quickstart failed with exit code $LASTEXITCODE. Inspect Docker Desktop containers and Compose logs before retrying."
    }

    $port = Get-EnvValue -Path $runtimeEnvPath -Key 'BITRIVER_LIVE_PORT'
    if ([string]::IsNullOrWhiteSpace($port)) {
        $port = '8080'
    }
    $baseUrl = "http://127.0.0.1:$port"

    Write-Host 'Checking live application routes...'
    foreach ($path in @('/healthz', '/readyz', '/viewer', '/admin')) {
        Wait-HttpEndpoint -Url "$baseUrl$path" -Timeout $TimeoutSeconds
    }

    Write-Host 'Windows source-checkout proof: PASS'
    Write-Host "  Viewer: $baseUrl/viewer"
    Write-Host "  Admin:  $baseUrl/admin"
    Write-Host "  Cleanup (PowerShell): `$env:BITRIVER_SRS_PUBLIC_RTMP_BASE_URL='rtmp://localhost:1935/live'; `$env:BITRIVER_OME_PUBLIC_LLHLS_BASE_URL='http://localhost:8080/live'; docker compose --env-file `"$envPath`" -f `"$composePath`" down"
} finally {
    [System.Environment]::SetEnvironmentVariable('BITRIVER_LIVE_MODE', $previousMode, 'Process')
    Remove-Item -LiteralPath $runtimeEnvPath -Force -ErrorAction SilentlyContinue
}
