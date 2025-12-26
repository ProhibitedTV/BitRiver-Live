#!/usr/bin/env pwsh
<#!
  Quickstart helper for Docker Desktop on Windows.
  Mirrors scripts/quickstart.sh with PowerShell-friendly checks.
!>

$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest

function Show-Usage {
    @'
Usage: scripts/quickstart.ps1 [-h|--help]

Options:
  -h, --help    Show this help message.
'@
}

function Require-Command {
    param(
        [Parameter(Mandatory = $true)][string]$Name
    )
    if (-not (Get-Command $Name -ErrorAction SilentlyContinue)) {
        Write-Error "Error: $Name is required but was not found in PATH."
    }
}

function Generate-StrongPassword {
    $length = 48
    $alphabet = ('a'..'z') + ('A'..'Z') + ('0'..'9')
    while ($true) {
        $bytes = New-Object byte[] $length
        [System.Security.Cryptography.RandomNumberGenerator]::Create().GetBytes($bytes)
        $chars = for ($i = 0; $i -lt $length; $i++) { $alphabet[$bytes[$i] % $alphabet.Count] }
        $candidate = -join $chars
        if ($candidate -match '[a-z]' -and $candidate -match '[A-Z]' -and $candidate -match '[0-9]') {
            return $candidate
        }
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

$ScriptDir = Split-Path -Parent $MyInvocation.MyCommand.Definition
$RepoRoot = Resolve-Path "$ScriptDir/.."
$EnvFile = if ($Env:ENV_FILE) { $Env:ENV_FILE } else { Join-Path $RepoRoot '.env' }
$ComposeFile = if ($Env:COMPOSE_FILE) { $Env:COMPOSE_FILE } else { Join-Path $RepoRoot 'deploy/docker-compose.yml' }
$MinDockerDiskGB = 15
$MinDockerDiskKB = $MinDockerDiskGB * 1024 * 1024

Require-Command docker
Require-Command python

try {
    docker compose version | Out-Null
} catch {
    Write-Error 'Docker Compose V2 is required. Install Docker Desktop with the compose plugin enabled and try again.'
}

try {
    $dockerInfo = docker info 2>&1
} catch {
    Write-Error "Failed to contact Docker. If Docker Desktop requires elevation, rerun this script from an elevated PowerShell session. Output:`n$($_.Exception.Message)"
}

function Get-DockerRoot {
    try {
        $root = docker info -f '{{ .DockerRootDir }}' 2>$null
        if (-not $root) {
            $root = ($dockerInfo -split "`n" | Where-Object { $_ -match 'Docker Root Dir' } | Select-Object -First 1) -replace '.*:',''
        }
    } catch {
        $root = ''
    }
    if (-not $root) {
        $root = 'C:/ProgramData/Docker'
    }
    return $root.Trim()
}

function Get-AvailableKB([string]$Path) {
    try {
        $resolved = Resolve-Path $Path -ErrorAction SilentlyContinue
        if (-not $resolved) { return $null }
        $driveRoot = [System.IO.Path]::GetPathRoot($resolved)
        $drive = Get-PSDrive -Name $driveRoot.TrimEnd(':','\') -ErrorAction SilentlyContinue
        if (-not $drive) { return $null }
        return [math]::Floor($drive.Free/1KB)
    } catch {
        return $null
    }
}

function Check-DockerDiskSpace {
    $dockerRoot = Get-DockerRoot
    $availableKB = Get-AvailableKB $dockerRoot
    if (-not $availableKB) {
        Write-Warning "Unable to determine free space for Docker storage at $dockerRoot; continuing without a preflight disk check."
        return
    }
    $availableGB = [math]::Floor($availableKB / 1024 / 1024)
    Write-Output "Docker storage path: $dockerRoot (free: ${availableGB}GB; minimum recommended: ${MinDockerDiskGB}GB)"
    if ($availableKB -lt $MinDockerDiskKB) {
        throw "Insufficient disk space detected for Docker builds. Free at least ${MinDockerDiskGB}GB on $dockerRoot and retry."
    }
}

$envDefaults = [ordered]@{
    BITRIVER_LIVE_PORT = '8080'
    BITRIVER_LIVE_STORAGE_DRIVER = 'postgres'
    BITRIVER_LIVE_MODE = 'development'
    BITRIVER_LIVE_ADDR = ':8080'
    BITRIVER_LIVE_POSTGRES_DSN = 'postgres://bitriver:bitriver@postgres:5432/bitriver?sslmode=disable'
    BITRIVER_LIVE_SESSION_TTL = '168h'
    BITRIVER_LIVE_SESSION_IDLE_TIMEOUT = '0'
    BITRIVER_LIVE_ALLOW_SELF_SIGNUP = 'false'
    BITRIVER_POSTGRES_USER = 'bitriver'
    BITRIVER_POSTGRES_PASSWORD = 'bitriver'
    BITRIVER_LIVE_POSTGRES_MAX_CONNS = '15'
    BITRIVER_LIVE_POSTGRES_MIN_CONNS = '5'
    BITRIVER_LIVE_POSTGRES_ACQUIRE_TIMEOUT = '5s'
    BITRIVER_LIVE_POSTGRES_MAX_CONN_LIFETIME = '30m'
    BITRIVER_LIVE_IMAGE_TAG = 'latest'
    BITRIVER_POSTGRES_HOST_PORT = '5432'
    BITRIVER_REDIS_PASSWORD = 'bitriver'
    BITRIVER_VIEWER_ORIGIN = 'http://viewer:3000'
    BITRIVER_REDIS_PORT = '6379'
    BITRIVER_SRS_API = 'http://srs-controller:1985'
    BITRIVER_SRS_TOKEN = 'local-dev-token'
    BITRIVER_SRS_API_PORT = '1985'
    BITRIVER_SRS_CONTROLLER_PORT = '1986'
    BITRIVER_SRS_RTMP_PORT = '1935'
    BITRIVER_VIEWER_IMAGE_TAG = 'latest'
    BITRIVER_SRS_CONTROLLER_IMAGE_TAG = 'latest'
    SRS_CONTROLLER_UPSTREAM = 'http://srs:1985/api/'
    BITRIVER_OME_IMAGE_TAG = '0.16.0'
    BITRIVER_OME_API = 'http://ome:8081'
    BITRIVER_OME_BIND = '0.0.0.0'
    BITRIVER_OME_IP = '0.0.0.0'
    BITRIVER_OME_SERVER_PORT = '9000'
    BITRIVER_OME_SERVER_TLS_PORT = '9443'
    BITRIVER_OME_USERNAME = 'admin'
    BITRIVER_OME_PASSWORD = 'local-dev-password'
    BITRIVER_OME_API_TOKEN = 'local-dev-access-token'
    BITRIVER_OME_ACCESS_TOKEN = 'local-dev-access-token'
    BITRIVER_OME_HTTP_PORT = '8081'
    BITRIVER_OME_SIGNALLING_PORT = '9000'
    BITRIVER_TRANSCODER_API = 'http://transcoder:9000'
    BITRIVER_TRANSCODER_TOKEN = 'local-dev-token'
    BITRIVER_TRANSCODER_HOST_PORT = '9001'
    BITRIVER_TRANSCODER_PUBLIC_BASE_URL = 'http://localhost:9080'
    BITRIVER_TRANSCODER_IMAGE_TAG = 'latest'
    BITRIVER_INGEST_HEALTH = '/healthz'
    NEXT_PUBLIC_API_BASE_URL = ''
    NEXT_VIEWER_BASE_PATH = '/viewer'
    NEXT_PUBLIC_VIEWER_URL = 'http://localhost:8080/viewer'
    BITRIVER_LIVE_ADMIN_EMAIL = 'admin@example.com'
    BITRIVER_LIVE_ADMIN_PASSWORD = 'local-dev-password'
    BITRIVER_LIVE_CHAT_QUEUE_REDIS_PASSWORD = 'bitriver'
}

$requiredKeys = @($envDefaults.Keys)

function Get-EnvValue {
    param(
        [string]$Key,
        [string]$DefaultValue
    )
    if (Test-Path $EnvFile) {
        $match = Select-String -Path $EnvFile -Pattern "^$Key=" | Select-Object -Last 1
        if ($match) {
            return $match.Line.Substring($Key.Length + 1)
        }
    }
    if ($envDefaults.Contains($Key)) { return $envDefaults[$Key] }
    return $DefaultValue
}

$generatedAdminPassword = ''
if (Test-Path $EnvFile) {
    Write-Output "Existing .env file detected at $EnvFile. Reconciling missing keys."
    if (-not (Select-String -Path $EnvFile -Pattern '^BITRIVER_LIVE_ADMIN_PASSWORD=' -Quiet)) {
        $generatedAdminPassword = Generate-StrongPassword
        $envDefaults['BITRIVER_LIVE_ADMIN_PASSWORD'] = $generatedAdminPassword
        Write-Output 'Missing BITRIVER_LIVE_ADMIN_PASSWORD in existing .env; generated a new one for reconciliation.'
    }
} else {
    $generatedAdminPassword = Generate-StrongPassword
    $envDefaults['BITRIVER_LIVE_ADMIN_PASSWORD'] = $generatedAdminPassword
    $lines = @()
    $lines += "# Generated by scripts/quickstart.ps1 on $(Get-Date -AsUTC -Format 'yyyy-MM-ddTHH:mm:ssZ')"
    $lines += '# Update the admin email and viewer URL before inviting real users.'
    foreach ($key in $envDefaults.Keys) {
        $lines += "$key=$($envDefaults[$key])"
    }
    $lines | Set-Content -Path $EnvFile -Encoding UTF8
    Write-Output "Wrote default environment configuration to $EnvFile with a freshly generated administrator password."
}

function Reconcile-EnvFile {
    if (-not (Test-Path $EnvFile)) { return }
    $content = Get-Content -Path $EnvFile -Encoding UTF8
    $needsNewline = if ($content.Count -gt 0) { -not ($content[-1] -eq '') } else { $false }
    foreach ($key in $requiredKeys) {
        if (-not ($content -match "^$key=")) {
            $value = if ($envDefaults.Contains($key)) { $envDefaults[$key] } else { '' }
            if ($needsNewline) {
                Add-Content -Path $EnvFile -Value '' -Encoding UTF8
                $needsNewline = $false
            }
            Add-Content -Path $EnvFile -Value "$key=$value" -Encoding UTF8
        }
    }
}

function Validate-OmeImageTag {
    $tag = Get-EnvValue -Key 'BITRIVER_OME_IMAGE_TAG' -DefaultValue $envDefaults['BITRIVER_OME_IMAGE_TAG']
    if ($tag -notmatch '^v?([0-9]+)\.([0-9]+)\.([0-9]+)$') {
        throw "BITRIVER_OME_IMAGE_TAG=$tag is not a MAJOR.MINOR.PATCH value. Pin it to a specific tag like 0.16.0."
    }
    $major = [int]$Matches[1]
    $minor = [int]$Matches[2]
    if ($major -eq 0 -and $minor -lt 16) {
        throw "BITRIVER_OME_IMAGE_TAG=$tag is unsupported. Use 0.16.0 or newer."
    }
}

function Render-OmeConfig {
    $template = Join-Path $RepoRoot 'deploy/ome/Server.xml'
    $output = Join-Path $RepoRoot 'deploy/ome/Server.generated.xml'
    if (-not (Test-Path $template)) { throw "OME template missing at $template" }
    if (-not (Test-Path $EnvFile)) { throw "Environment file not found at $EnvFile." }

    $omeBind = Get-EnvValue -Key 'BITRIVER_OME_BIND' -DefaultValue $envDefaults['BITRIVER_OME_BIND']
    $omePort = Get-EnvValue -Key 'BITRIVER_OME_SERVER_PORT' -DefaultValue $envDefaults['BITRIVER_OME_SERVER_PORT']
    $omeTlsPort = Get-EnvValue -Key 'BITRIVER_OME_SERVER_TLS_PORT' -DefaultValue $envDefaults['BITRIVER_OME_SERVER_TLS_PORT']
    $omeIp = Get-EnvValue -Key 'BITRIVER_OME_IP' -DefaultValue $envDefaults['BITRIVER_OME_IP']
    $omeTag = Get-EnvValue -Key 'BITRIVER_OME_IMAGE_TAG' -DefaultValue $envDefaults['BITRIVER_OME_IMAGE_TAG']
    $omeIcePortRange = Get-EnvValue -Key 'BITRIVER_OME_ICE_PORT_RANGE' -DefaultValue '10000-10009'
    $omeTcpRelay = Get-EnvValue -Key 'BITRIVER_OME_TCP_RELAY' -DefaultValue (Get-EnvValue -Key 'BITRIVER_OME_RELAY_PORT' -DefaultValue '3478')
    if ($omeTcpRelay -notmatch ':') { $omeTcpRelay = "*:$omeTcpRelay" }
    $omeIceCandidate = Get-EnvValue -Key 'BITRIVER_OME_ICE_CANDIDATE' -DefaultValue "*:${omeIcePortRange}/udp"
    $omeUsername = Get-EnvValue -Key 'BITRIVER_OME_USERNAME' -DefaultValue ''
    $omePassword = Get-EnvValue -Key 'BITRIVER_OME_PASSWORD' -DefaultValue ''
    $omeApiToken = Get-EnvValue -Key 'BITRIVER_OME_API_TOKEN' -DefaultValue ''
    $omeAccessToken = Get-EnvValue -Key 'BITRIVER_OME_ACCESS_TOKEN' -DefaultValue $omeApiToken

    if (-not $omeUsername -or -not $omePassword -or -not $omeApiToken) {
        throw 'BITRIVER_OME_USERNAME, BITRIVER_OME_PASSWORD, and BITRIVER_OME_API_TOKEN must be set in .env.'
    }

    Write-Output 'Rendering OME config from template...'
    $args = @(
        'python', (Join-Path $ScriptDir 'render_ome_config.py'),
        '--template', $template,
        '--output', $output,
        '--bind', $omeBind,
        '--server-ip', $omeIp,
        '--port', $omePort,
        '--tls-port', $omeTlsPort,
        '--username', $omeUsername,
        '--password', $omePassword,
        '--api-token', $omeApiToken,
        '--access-token', $omeAccessToken,
        '--image-tag', $omeTag,
        '--tcp-relay', $omeTcpRelay,
        '--ice-candidate', $omeIceCandidate
    )
    $proc = Start-Process -FilePath $args[0] -ArgumentList $args[1..($args.Count - 1)] -NoNewWindow -PassThru -Wait -RedirectStandardOutput $null -RedirectStandardError ([System.IO.Path]::GetTempFileName())
    if ($proc.ExitCode -ne 0) {
        $err = Get-Content -Path $proc.RedirectStandardError -Raw
        throw "OME configuration rendering failed: $err"
    }
    Write-Output "Rendered OME configuration to $output"
}

function Wait-ForApi {
    param(
        [string]$Url,
        [int]$Attempts = 60,
        [int]$SleepSeconds = 2
    )
    Write-Output "Waiting for BitRiver Live API at $Url ..."
    for ($i = 1; $i -le $Attempts; $i++) {
        try {
            Invoke-WebRequest -Uri $Url -UseBasicParsing -TimeoutSec 5 | Out-Null
            Write-Output 'API is reachable.'
            return $true
        } catch {
            Start-Sleep -Seconds $SleepSeconds
        }
    }
    Write-Warning "Timed out waiting for API readiness after $($Attempts * $SleepSeconds) seconds."
    return $false
}

Reconcile-EnvFile
Validate-OmeImageTag
Check-DockerDiskSpace
Render-OmeConfig

Write-Output 'Starting BitRiver Live stack...'
docker compose -f $ComposeFile up --build -d

$apiPort = Get-EnvValue -Key 'BITRIVER_LIVE_PORT' -DefaultValue $envDefaults['BITRIVER_LIVE_PORT']
$readyUrl = "http://localhost:$apiPort/readyz"
if (-not (Wait-ForApi -Url $readyUrl)) {
    throw 'API did not become ready in time; check docker compose logs.'
}

$adminPassword = if ($generatedAdminPassword) { $generatedAdminPassword } else { Get-EnvValue -Key 'BITRIVER_LIVE_ADMIN_PASSWORD' -DefaultValue $envDefaults['BITRIVER_LIVE_ADMIN_PASSWORD'] }
$adminEmail = Get-EnvValue -Key 'BITRIVER_LIVE_ADMIN_EMAIL' -DefaultValue $envDefaults['BITRIVER_LIVE_ADMIN_EMAIL']
Write-Output ''
Write-Output 'Administrator credentials:'
Write-Output "  Email:    $adminEmail"
Write-Output "  Password: $adminPassword"
Write-Output 'Log in through the control center and change the password immediately.'
