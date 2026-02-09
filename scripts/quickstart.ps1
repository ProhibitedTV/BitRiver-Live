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

function Get-EnvValue {
    param(
        [Parameter(Mandatory = $true)][string]$FilePath,
        [Parameter(Mandatory = $true)][string]$Key
    )

    if (-not (Test-Path -LiteralPath $FilePath -PathType Leaf)) {
        return ''
    }

    $result = ''
    foreach ($line in Get-Content -LiteralPath $FilePath) {
        $trimmed = $line.Trim()
        if (-not $trimmed -or $trimmed.StartsWith('#')) {
            continue
        }

        $working = $trimmed
        if ($working -match '^export\s+') {
            $working = $working -replace '^export\s+', ''
        }

        $eqIndex = $working.IndexOf('=')
        if ($eqIndex -lt 0) {
            continue
        }

        $name = $working.Substring(0, $eqIndex).Trim()
        if ($name -ne $Key) {
            continue
        }

        $value = $working.Substring($eqIndex + 1).Trim()
        if (
            ($value.Length -ge 2) -and
            (($value.StartsWith('"') -and $value.EndsWith('"')) -or ($value.StartsWith("'") -and $value.EndsWith("'")))
        ) {
            $value = $value.Substring(1, $value.Length - 2)
        }

        if ($value) {
            $result = $value
        }
    }

    return $result
}

function Get-RenderedAccessToken {
    param([Parameter(Mandatory = $true)][string]$ConfigPath)

    if (-not (Test-Path -LiteralPath $ConfigPath -PathType Leaf)) {
        return ''
    }

    $content = Get-Content -LiteralPath $ConfigPath -Raw
    $match = [regex]::Match($content, '<Managers>.*?<API>.*?<AccessToken>\s*(.*?)\s*</AccessToken>.*?</API>.*?</Managers>', [System.Text.RegularExpressions.RegexOptions]::Singleline)
    if (-not $match.Success) {
        return ''
    }

    return $match.Groups[1].Value.Trim()
}

function Invoke-NativeOmeTokenVerification {
    param(
        [Parameter(Mandatory = $true)][string]$EnvFile,
        [Parameter(Mandatory = $true)][string]$ConfigFile
    )

    if (-not (Test-Path -LiteralPath $EnvFile -PathType Leaf)) {
        Write-Error "OME token verification failed: env file not found at $EnvFile"
    }

    if (-not (Test-Path -LiteralPath $ConfigFile -PathType Leaf)) {
        Write-Error "OME token verification failed: generated config not found at $ConfigFile"
    }

    $renderedToken = Get-RenderedAccessToken -ConfigPath $ConfigFile
    $healthcheckToken = Get-EnvValue -FilePath $EnvFile -Key 'BITRIVER_OME_HEALTHCHECK_TOKEN'
    $accessToken = Get-EnvValue -FilePath $EnvFile -Key 'BITRIVER_OME_ACCESS_TOKEN'
    $apiToken = Get-EnvValue -FilePath $EnvFile -Key 'BITRIVER_OME_API_TOKEN'

    $expectedToken = $healthcheckToken
    if (-not $expectedToken) {
        $expectedToken = $accessToken
    }
    if (-not $expectedToken) {
        $expectedToken = $apiToken
    }

    if (-not $renderedToken) {
        Write-Error "OME token verification failed: <Managers><API><AccessToken> is empty in $ConfigFile"
    }

    if (-not $expectedToken) {
        Write-Error "OME token verification failed: resolved runtime token from canonical precedence BITRIVER_OME_HEALTHCHECK_TOKEN -> BITRIVER_OME_ACCESS_TOKEN -> BITRIVER_OME_API_TOKEN is empty in $EnvFile"
    }

    if ($renderedToken -ne $expectedToken) {
        $message = @"
OME token verification failed: rendered and runtime tokens differ.
  rendered (<Managers><API><AccessToken>): $renderedToken
  expected (BITRIVER_OME_HEALTHCHECK_TOKEN -> BITRIVER_OME_ACCESS_TOKEN -> BITRIVER_OME_API_TOKEN): $expectedToken
Fix by updating $EnvFile and re-rendering with:
  go run ./cmd/bitriver ome render --force --env-file $EnvFile
"@
        Write-Error $message
    }

    Write-Output 'OME token verification passed: rendered AccessToken matches compose runtime health token source.'
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

    $rawAuthMode = Get-EnvValue -FilePath $EnvFilePath -Key 'BITRIVER_OME_HEALTHCHECK_AUTH_MODE'
    $authMode = if ($rawAuthMode) { $rawAuthMode.ToLowerInvariant() } else { 'accesstoken' }

    if ($authMode -ne 'accesstoken' -and $authMode -ne 'basic') {
        $message = @"
OME auth preflight failed: BITRIVER_OME_HEALTHCHECK_AUTH_MODE must be accesstoken or basic (current: $(if ($rawAuthMode) { $rawAuthMode } else { '<empty>' })).
Set BITRIVER_OME_HEALTHCHECK_AUTH_MODE=accesstoken for token probes, or:
  BITRIVER_OME_HEALTHCHECK_AUTH_MODE=basic
  BITRIVER_OME_USERNAME=ome-operator
  BITRIVER_OME_PASSWORD=replace-with-strong-password
in $EnvFilePath before running scripts/quickstart.sh.
"@
        Write-Error $message
    }

    $shellAuthMode = $Env:BITRIVER_OME_HEALTHCHECK_AUTH_MODE
    if ($shellAuthMode -and ($shellAuthMode.ToLowerInvariant() -ne $authMode)) {
        Write-Warning "OME auth preflight notice: shell BITRIVER_OME_HEALTHCHECK_AUTH_MODE=$shellAuthMode differs from $EnvFilePath ($rawAuthMode); using env-file value for validation."
    }

    $apiToken = Get-EnvValue -FilePath $EnvFilePath -Key 'BITRIVER_OME_API_TOKEN'
    $accessToken = Get-EnvValue -FilePath $EnvFilePath -Key 'BITRIVER_OME_ACCESS_TOKEN'
    $healthcheckToken = Get-EnvValue -FilePath $EnvFilePath -Key 'BITRIVER_OME_HEALTHCHECK_TOKEN'

    if (-not $apiToken) {
        $message = @"
OME auth preflight failed: BITRIVER_OME_API_TOKEN is empty in $EnvFilePath.
Expected BITRIVER_OME_API_TOKEN=<non-empty token> so OME render can populate <Managers><API><AccessToken>.
"@
        Write-Error $message
    }

    if ($authMode -eq 'basic') {
        $omeUsername = Get-EnvValue -FilePath $EnvFilePath -Key 'BITRIVER_OME_USERNAME'
        $omePassword = Get-EnvValue -FilePath $EnvFilePath -Key 'BITRIVER_OME_PASSWORD'
        if (-not $omeUsername -or -not $omePassword) {
            $message = @"
OME auth preflight failed: BITRIVER_OME_HEALTHCHECK_AUTH_MODE=basic requires BITRIVER_OME_USERNAME and BITRIVER_OME_PASSWORD in $EnvFilePath.
Example:
  BITRIVER_OME_HEALTHCHECK_AUTH_MODE=basic
  BITRIVER_OME_USERNAME=ome-operator
  BITRIVER_OME_PASSWORD=replace-with-strong-password
"@
            Write-Error $message
        }
    } else {
        if (-not $healthcheckToken -and -not $accessToken -and -not $apiToken) {
            $message = @"
OME auth preflight failed: BITRIVER_OME_HEALTHCHECK_AUTH_MODE=accesstoken requires a non-empty token in canonical order:
  BITRIVER_OME_HEALTHCHECK_TOKEN -> BITRIVER_OME_ACCESS_TOKEN -> BITRIVER_OME_API_TOKEN
Example:
  BITRIVER_OME_HEALTHCHECK_AUTH_MODE=accesstoken
  BITRIVER_OME_API_TOKEN=replace-with-non-empty-token
  # Optional overrides:
  # BITRIVER_OME_ACCESS_TOKEN=replace-with-probe-token
  # BITRIVER_OME_HEALTHCHECK_TOKEN=replace-with-healthcheck-token
"@
            Write-Error $message
        }
    }

    Write-Output 'Running OME auth preflight: rendering config and validating token consistency ...'
    Invoke-Cli -CliArgs @('ome', 'render', '--force', '--env-file', $EnvFilePath)

    $configPath = Join-Path $CodeRootPath 'deploy/ome/Server.generated.xml'
    $bash = Get-Command bash -ErrorAction SilentlyContinue
    if ($bash -and (Test-Path -LiteralPath $VerifyScriptPath -PathType Leaf)) {
        & $bash.Source $VerifyScriptPath '--env-file' $EnvFilePath '--config' $configPath
        return
    }

    Invoke-NativeOmeTokenVerification -EnvFile $EnvFilePath -ConfigFile $configPath
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
$VerifyOmeTokenScript = Join-Path $CodeRoot 'scripts/verify-ome-health-token.sh'

$quickstartArgs = @('quickstart', '--env-file', $EnvFile, '--compose-file', $ComposeFile)

Write-Output 'Running BitRiver Live quickstart ...'
Invoke-OmeAuthPreflight -EnvFilePath $EnvFile -CodeRootPath $CodeRoot -VerifyScriptPath $VerifyOmeTokenScript
Invoke-Cli -CliArgs $quickstartArgs
