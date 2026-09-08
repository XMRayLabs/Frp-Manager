const loopbackHosts = new Set(['localhost', '127.0.0.1', '::1', '[::1]'])

function shellQuote(value) {
  return `'${String(value).replaceAll("'", "'\\''")}'`
}

function appleScriptQuote(value) {
  return `"${String(value).replaceAll('\\', '\\\\').replaceAll('"', '\\"')}"`
}

function powershellQuote(value) {
  return `'${String(value).replaceAll("'", "''")}'`
}

function buildWindowsServiceScript({
  action,
  sourceBinary = '',
  tempConfig = '',
  installDir,
  targetBinary,
  targetConfig,
  outputPath,
  serviceName = 'frpp',
  verifyService = true,
  managerRpcPort = 0,
}) {
  const allowedActions = new Set(['apply', 'install', 'restart', 'start', 'stop', 'uninstall'])
  if (!allowedActions.has(action)) throw new Error(`Unsupported Windows service action: ${action}`)

  return `
$ErrorActionPreference = 'Stop'
$ProgressPreference = 'SilentlyContinue'
$action = ${powershellQuote(action)}
$sourceBinary = ${powershellQuote(sourceBinary)}
$tempConfig = ${powershellQuote(tempConfig)}
$installDir = ${powershellQuote(installDir)}
$targetBinary = ${powershellQuote(targetBinary)}
$targetConfig = ${powershellQuote(targetConfig)}
$outputPath = ${powershellQuote(outputPath)}
$serviceName = ${powershellQuote(serviceName)}
$verifyService = ${verifyService ? '$true' : '$false'}
$managerRpcPort = ${Number.isInteger(managerRpcPort) && managerRpcPort > 0 ? managerRpcPort : 0}
$serviceLog = [System.IO.Path]::Combine([System.IO.Path]::GetDirectoryName($targetBinary), 'service.log')

function Write-Result {
  param([string]$Message)
  if ($Message) {
    $cleanMessage = $Message -replace "$([char]27)\\[[0-9;]*[A-Za-z]", ''
    [System.IO.File]::AppendAllText(
      $outputPath,
      $cleanMessage + [Environment]::NewLine,
      [System.Text.UTF8Encoding]::new($false)
    )
  }
}

function Invoke-Core {
  param(
    [string[]]$CoreArguments,
    [switch]$AllowFailure
  )

  $captureId = [Guid]::NewGuid().ToString('N')
  $captureDir = [System.IO.Path]::GetDirectoryName($outputPath)
  $stdoutPath = [System.IO.Path]::Combine($captureDir, "core-$captureId.stdout")
  $stderrPath = [System.IO.Path]::Combine($captureDir, "core-$captureId.stderr")
  try {
    $startParams = @{
      FilePath = $targetBinary
      ArgumentList = $CoreArguments
      WorkingDirectory = [System.IO.Path]::GetDirectoryName($targetBinary)
      Wait = $true
      PassThru = $true
      NoNewWindow = $true
      RedirectStandardOutput = $stdoutPath
      RedirectStandardError = $stderrPath
    }
    $process = Start-Process @startParams
    $exitCode = [int]$process.ExitCode
    $stdout = ''
    if (Test-Path -LiteralPath $stdoutPath) {
      $stdout = [System.IO.File]::ReadAllText($stdoutPath, [System.Text.Encoding]::UTF8)
    }
    $stderr = ''
    if (Test-Path -LiteralPath $stderrPath) {
      $stderr = [System.IO.File]::ReadAllText($stderrPath, [System.Text.Encoding]::UTF8)
    }
    $result = @($stdout.Trim(), $stderr.Trim()) -join [Environment]::NewLine
    if ($result.Trim()) { Write-Result $result.Trim() }
  } finally {
    Remove-Item -LiteralPath $stdoutPath -Force -ErrorAction SilentlyContinue
    Remove-Item -LiteralPath $stderrPath -Force -ErrorAction SilentlyContinue
  }
  if ($exitCode -ne 0 -and -not $AllowFailure) {
    throw "frp-manager '$($CoreArguments -join ' ')' failed with exit code $exitCode"
  }
}

function Get-ServiceLogTail {
  if (-not (Test-Path -LiteralPath $serviceLog -PathType Leaf)) { return '' }
  $lastError = ''
  for ($attempt = 1; $attempt -le 5; $attempt++) {
    $stream = $null
    $reader = $null
    try {
      $share = [System.IO.FileShare]::ReadWrite -bor [System.IO.FileShare]::Delete
      $stream = [System.IO.FileStream]::new(
        $serviceLog,
        [System.IO.FileMode]::Open,
        [System.IO.FileAccess]::Read,
        $share
      )
      $reader = [System.IO.StreamReader]::new($stream, [System.Text.Encoding]::UTF8)
      $content = $reader.ReadToEnd()
      $lines = @($content -split "\r?\n")
      if ($lines.Count -gt 120) {
        $lines = @($lines | Select-Object -Last 120)
      }
      return $lines -join [Environment]::NewLine
    } catch {
      $lastError = $_.Exception.Message
      Start-Sleep -Milliseconds 200
    } finally {
      if ($reader) {
        $reader.Dispose()
      } elseif ($stream) {
        $stream.Dispose()
      }
    }
  }
  return "Unable to read service log: $lastError"
}

function Write-ServiceLogTail {
  $logTail = Get-ServiceLogTail
  if (-not [string]::IsNullOrWhiteSpace($logTail)) {
    Write-Result '--- Windows service log ---'
    Write-Result $logTail.Trim()
    Write-Result '--- End service log ---'
  }
}

function Assert-ServiceRunning {
  if (-not $verifyService) { return }
  $deadline = [DateTime]::UtcNow.AddSeconds(20)
  do {
    Start-Sleep -Seconds 1
    $service = Get-CimInstance -ClassName Win32_Service -Filter "Name='$serviceName'" -ErrorAction SilentlyContinue
    if (-not $service) {
      throw "Windows service '$serviceName' was not found after startup"
    }
    if ($service.State -ne 'Running') {
      Write-ServiceLogTail
      throw "Windows service '$serviceName' stopped shortly after startup (state: $($service.State), exit code: $($service.ExitCode))"
    }
    $logTail = Get-ServiceLogTail
    if ($logTail -match 'client get server register envent success') {
      Write-Result "Windows service '$serviceName' is running and registered with the manager."
      return
    }
    if ($logTail -like 'Unable to read service log:*' -and $managerRpcPort -gt 0) {
      $managerConnection = Get-NetTCPConnection -OwningProcess ([uint32]$service.ProcessId) -State Established -ErrorAction SilentlyContinue |
        Where-Object { $_.RemotePort -eq $managerRpcPort } |
        Select-Object -First 1
      if ($managerConnection) {
        Write-Result "Windows service '$serviceName' is running with an established manager RPC connection."
        return
      }
    }
  } while ([DateTime]::UtcNow -lt $deadline)

  Write-ServiceLogTail
  if ($service.State -eq 'Running') {
    throw "Windows service '$serviceName' is running but did not register with the manager within 20 seconds"
  }
}

try {
  Write-Result "Running Windows service action '$action'."
  if ($action -eq 'apply' -or $action -eq 'install') {
    if (-not (Test-Path -LiteralPath $sourceBinary -PathType Leaf)) {
      throw "Client binary not found: $sourceBinary"
    }
    if (-not (Test-Path -LiteralPath $tempConfig -PathType Leaf)) {
      throw "Temporary client configuration not found: $tempConfig"
    }
    if (Test-Path -LiteralPath $targetBinary -PathType Leaf) {
      Invoke-Core -CoreArguments @('stop') -AllowFailure
      Invoke-Core -CoreArguments @('uninstall') -AllowFailure
    }
    New-Item -ItemType Directory -Path $installDir -Force | Out-Null
    Remove-Item -LiteralPath $serviceLog -Force -ErrorAction SilentlyContinue
    if ([System.IO.Path]::GetFullPath($sourceBinary) -ne [System.IO.Path]::GetFullPath($targetBinary)) {
      Copy-Item -LiteralPath $sourceBinary -Destination $targetBinary -Force
    }
    Remove-Item -LiteralPath ($targetBinary + '.new') -Force -ErrorAction SilentlyContinue
    Copy-Item -LiteralPath $tempConfig -Destination $targetConfig -Force

    $aclResult = & icacls.exe $targetConfig /inheritance:r /grant:r '*S-1-5-18:(F)' '*S-1-5-32-544:(F)' 2>&1 | Out-String
    $aclExitCode = $LASTEXITCODE
    if ($aclExitCode -ne 0) {
      throw "Unable to protect the service configuration (icacls exit code $aclExitCode): $($aclResult.Trim())"
    }

    Invoke-Core -CoreArguments @('install', 'client')
    if ($action -eq 'apply') {
      Invoke-Core -CoreArguments @('start')
      Assert-ServiceRunning
    }
  } else {
    if (-not (Test-Path -LiteralPath $targetBinary -PathType Leaf)) {
      throw 'frp-manager service is not installed'
    }
    if ($action -eq 'restart') {
      Invoke-Core -CoreArguments @('stop') -AllowFailure
      Remove-Item -LiteralPath $serviceLog -Force -ErrorAction SilentlyContinue
      Invoke-Core -CoreArguments @('start')
      Assert-ServiceRunning
    } else {
      if ($action -eq 'start') {
        Remove-Item -LiteralPath $serviceLog -Force -ErrorAction SilentlyContinue
      }
      Invoke-Core -CoreArguments @($action)
      if ($action -eq 'start') { Assert-ServiceRunning }
    }
    if ($action -eq 'uninstall') {
      Remove-Item -LiteralPath $targetConfig -Force -ErrorAction SilentlyContinue
      Remove-Item -LiteralPath $targetBinary -Force -ErrorAction SilentlyContinue
    }
  }
  Write-Result "Windows service action '$action' completed."
  exit 0
} catch {
  try {
    Write-Result ("Windows service action '$action' failed: " + $_.Exception.Message)
    if ($_.InvocationInfo.PositionMessage) {
      Write-Result $_.InvocationInfo.PositionMessage
    }
    if ($_.ScriptStackTrace) {
      Write-Result ("PowerShell stack: " + $_.ScriptStackTrace)
    }
  } catch {
    Write-Error $_.Exception.Message
  }
  exit 1
}
`
}

function buildWindowsElevationLauncher(helper, outputPath) {
  const encodedHelper = Buffer.from(helper, 'utf16le').toString('base64')
  return `
$ErrorActionPreference = 'Stop'
try {
  $process = Start-Process -FilePath 'powershell.exe' -Verb RunAs -Wait -PassThru -ArgumentList @(
    '-NoProfile',
    '-NonInteractive',
    '-ExecutionPolicy',
    'Bypass',
    '-EncodedCommand',
    '${encodedHelper}'
  )
  if (Test-Path -LiteralPath ${powershellQuote(outputPath)}) {
    [Console]::OutputEncoding = [System.Text.UTF8Encoding]::new($false)
    Get-Content -LiteralPath ${powershellQuote(outputPath)} -Raw -Encoding UTF8
  }
  exit [int]$process.ExitCode
} catch {
  Write-Error $_.Exception.Message
  if ($_.Exception.NativeErrorCode -eq 1223) { exit 1223 }
  exit 1
}
`
}

function dotenvValue(value) {
  return `"${String(value)
    .replaceAll('\\', '\\\\')
    .replaceAll('\r', '\\r')
    .replaceAll('\n', '\\n')
    .replaceAll('"', '\\"')}"`
}

function buildManagedServiceConfig(profile, globalSecret) {
  return [
    `APP_GLOBAL_SECRET=${dotenvValue(globalSecret)}`,
    'APP_AUTO_UPDATE=true',
    `CLIENT_ID=${dotenvValue(profile.clientId)}`,
    `CLIENT_SECRET=${dotenvValue(profile.secret)}`,
    `CLIENT_JOIN_TOKEN=${dotenvValue(profile.joinToken || '')}`,
    `CLIENT_API_URL=${dotenvValue(profile.apiUrl)}`,
    `CLIENT_RPC_URL=${dotenvValue(profile.rpcUrl)}`,
    '',
  ].join('\n')
}

function redactSecrets(value, secrets = []) {
  let result = String(value || '')
  for (const secret of secrets) {
    if (secret) result = result.split(secret).join('[redacted]')
  }
  return result
    .replace(/((?:^|\s)(?:-s|--secret|-j|--join-token)(?:=|\s+))([^\s]+)/gi, '$1[redacted]')
    .replace(/(CLIENT_(?:SECRET|JOIN_TOKEN)\s*=\s*)[^\s]+/gi, '$1[redacted]')
}

function validateProfile(profile) {
  for (const key of ['apiUrl', 'rpcUrl']) {
    if (!profile?.[key]) throw new Error(`Missing ${key}.`)
  }

  if (!profile.joinToken && (!profile.secret || !profile.clientId)) throw new Error('Provide an enrollment token or a client ID and secret.')
  const api = parseEndpoint(profile.apiUrl, 'API URL')
  const rpc = parseEndpoint(profile.rpcUrl, 'RPC URL')
  if (api.username || api.password || rpc.username || rpc.password) {
    throw new Error('Endpoint URLs must not contain embedded credentials.')
  }
  if (profile.allowInsecure) return

  if (!loopbackHosts.has(api.hostname) && api.protocol !== 'https:') {
    throw new Error('Remote API URL must use HTTPS. Enable the insecure endpoint override only for an isolated trusted network.')
  }
  if (!loopbackHosts.has(rpc.hostname) && !['grpc:', 'wss:'].includes(rpc.protocol)) {
    throw new Error('Remote RPC URL must use grpc:// (TLS) or wss://. Enable the insecure endpoint override only for an isolated trusted network.')
  }
}

function hasInsecureRemoteEndpoint(profile) {
  try {
    const api = parseEndpoint(profile.apiUrl, 'API URL')
    const rpc = parseEndpoint(profile.rpcUrl, 'RPC URL')
    return (
      (!loopbackHosts.has(api.hostname) && api.protocol !== 'https:') ||
      (!loopbackHosts.has(rpc.hostname) && !['grpc:', 'wss:'].includes(rpc.protocol))
    )
  } catch {
    return false
  }
}

function rpcPort(value) {
  try {
    const endpoint = new URL(value)
    if (endpoint.port) return Number(endpoint.port)
    if (endpoint.protocol === 'wss:') return 443
    if (endpoint.protocol === 'ws:') return 80
  } catch {
    // Profile validation reports malformed endpoints during apply/install.
  }
  return 0
}

function parseEndpoint(value, label) {
  let parsed
  try {
    parsed = new URL(value)
  } catch {
    throw new Error(`${label} is invalid.`)
  }
  if (!parsed.hostname) throw new Error(`${label} must include a host.`)
  return parsed
}

module.exports = {
  appleScriptQuote,
  buildManagedServiceConfig,
  buildWindowsElevationLauncher,
  buildWindowsServiceScript,
  dotenvValue,
  hasInsecureRemoteEndpoint,
  powershellQuote,
  redactSecrets,
  rpcPort,
  shellQuote,
  validateProfile,
}
