param(
  [string]$LauncherRoot,
  [string]$EnvFile,
  [string]$BinaryPath,
  [string]$Command = 'start'
)

$ErrorActionPreference = 'Stop'

function Resolve-LauncherPath {
  param([string]$Path)
  $resolved = Resolve-Path -LiteralPath $Path -ErrorAction SilentlyContinue
  if ($resolved) { return $resolved.ToString() }
  return $Path
}

$scriptDir = Split-Path -Parent $MyInvocation.MyCommand.Definition
$assetsDefault = Join-Path (Split-Path -Parent $scriptDir) 'share/bitriver-live'
$assetsDir = if ($LauncherRoot) { $LauncherRoot } elseif ($env:BITRIVER_LAUNCHER_ROOT) { $env:BITRIVER_LAUNCHER_ROOT } else { $assetsDefault }
$composeFile = Join-Path $assetsDir 'deploy/docker-compose.yml'
$exampleEnv = Join-Path $assetsDir 'deploy/.env.example'
$envFilePath = if ($EnvFile) { $EnvFile } elseif ($env:BITRIVER_ENV_FILE) { $env:BITRIVER_ENV_FILE } else { Join-Path $assetsDir 'deploy/.env' }
$binary = if ($BinaryPath) { $BinaryPath } elseif ($env:BITRIVER_BINARY) { $env:BITRIVER_BINARY } else { Join-Path $scriptDir 'bitriver.exe' }

function Require-Command {
  param([string]$Command)
  $cmd = Get-Command $Command -ErrorAction SilentlyContinue
  if (-not $cmd) {
    Write-Error "$Command is required but missing from PATH"
  }
}

function Ensure-Assets {
  if (-not (Test-Path -LiteralPath $composeFile)) {
    throw "docker-compose.yml not found at $composeFile. Reinstall BitRiver Live installer."
  }

  if (-not (Test-Path -LiteralPath $envFilePath)) {
    if (Test-Path -LiteralPath $exampleEnv) {
      New-Item -ItemType Directory -Path (Split-Path -Parent $envFilePath) -Force | Out-Null
      Copy-Item -LiteralPath $exampleEnv -Destination $envFilePath -Force
      Write-Host "Created default env file at $envFilePath. Please review secrets before continuing."
    } else {
      throw "No env file found and no example to copy from."
    }
  }
}

function Check-Prereqs {
  Write-Host 'Checking Docker and Compose prerequisites...'
  Require-Command 'docker'
  & docker version | Out-Null
  & docker compose version | Out-Null
}

function Invoke-Compose {
  param([Parameter(ValueFromRemainingArguments = $true)][string[]]$Args)
  & docker compose -f $composeFile --env-file $envFilePath @Args
}

function Pull-Images {
  Write-Host 'Pulling BitRiver Live images...'
  Invoke-Compose pull
}

function Bring-Up {
  Write-Host 'Starting BitRiver Live stack...'
  Invoke-Compose up -d
}

function Stop-Stack {
  Write-Host 'Stopping BitRiver Live stack...'
  Invoke-Compose stop
}

function Restart-Stack {
  Write-Host 'Restarting BitRiver Live stack...'
  Invoke-Compose restart
}

function Follow-Logs {
  Write-Host 'Tailing BitRiver Live logs...'
  Invoke-Compose logs -f
}

function Open-Desktop {
  if (-not (Test-Path -LiteralPath $binary)) {
    throw "bitriver binary not found at $binary. Reinstall the launcher or set BITRIVER_BINARY."
  }

  Write-Host 'Launching BitRiver Live control panel...'
  & $binary desktop --compose-file $composeFile --env-file $envFilePath
}

function Show-Usage {
  Write-Host "Usage: bitriver-live-wrapper.ps1 [-LauncherRoot path] [-EnvFile path] [-BinaryPath path] [-Command start|stop|restart|logs|ui]"
}

switch ($Command.ToLowerInvariant()) {
  'start' {
    Check-Prereqs
    Ensure-Assets
    if (Test-Path -LiteralPath $binary) {
      Write-Host 'Running bitriver doctor for sanity checks...'
      try {
        & $binary doctor | Write-Output
      } catch {
        Write-Warning "bitriver doctor reported issues; continuing because Docker is available. $_"
      }
    }
    Pull-Images
    Bring-Up
    Write-Host "BitRiver Live is starting. Use '.\\bitriver-live-wrapper.ps1 -Command logs' to follow logs or '.\\bitriver-live-wrapper.ps1 -Command ui' to keep the control panel in your tray."
  }
  'stop' {
    Check-Prereqs
    Ensure-Assets
    Stop-Stack
  }
  'restart' {
    Check-Prereqs
    Ensure-Assets
    Restart-Stack
  }
  'logs' {
    Check-Prereqs
    Ensure-Assets
    Follow-Logs
  }
  'ui' {
    Check-Prereqs
    Ensure-Assets
    Open-Desktop
  }
  Default { Show-Usage }
}
