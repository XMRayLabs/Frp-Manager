$ErrorActionPreference = 'Stop'

Add-Type -AssemblyName System.Windows.Forms
Add-Type -AssemblyName System.Drawing

$InstallDir = Join-Path $env:ProgramFiles 'frp-manager'
$ExePath = Join-Path $InstallDir 'frpp.exe'
$DataDir = Join-Path $env:ProgramData 'frp-manager'
$ConfigPath = Join-Path $DataDir 'client-manager.json'
$ServiceName = 'frpp'
$DefaultClientId = 'admin.c.01'
$DefaultApiUrl = 'http://frp.xmray.de:9000'
$DefaultRpcUrl = 'grpc://frp.xmray.de:9001'
$DefaultCommand = "frp-manager client -s <secret> -i $DefaultClientId --api-url $DefaultApiUrl --rpc-url $DefaultRpcUrl"

function Test-Admin {
  $identity = [Security.Principal.WindowsIdentity]::GetCurrent()
  $principal = New-Object Security.Principal.WindowsPrincipal($identity)
  return $principal.IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)
}

if (-not (Test-Admin)) {
  Start-Process -FilePath 'powershell.exe' -ArgumentList @(
    '-NoProfile',
    '-ExecutionPolicy', 'Bypass',
    '-File', "`"$PSCommandPath`""
  ) -Verb RunAs
  exit
}

function Load-Config {
  if (Test-Path $ConfigPath) {
    try {
      return Get-Content $ConfigPath -Raw | ConvertFrom-Json
    } catch {
      return $null
    }
  }
  return $null
}

function Save-Config($command, $secret, $clientId, $apiUrl, $rpcUrl) {
  New-Item -ItemType Directory -Force -Path $DataDir | Out-Null
  [pscustomobject]@{
    command = $command
    secret = $secret
    clientId = $clientId
    apiUrl = $apiUrl
    rpcUrl = $rpcUrl
  } | ConvertTo-Json | Set-Content -Path $ConfigPath -Encoding UTF8
}

function Get-ServiceText {
  $svc = Get-Service -Name $ServiceName -ErrorAction SilentlyContinue
  if ($null -eq $svc) {
    return 'Service: not installed'
  }
  return "Service: $($svc.Status)"
}

function Run-Frpp($arguments) {
  if (-not (Test-Path $ExePath)) {
    throw "frpp.exe not found: $ExePath"
  }

  $psi = New-Object System.Diagnostics.ProcessStartInfo
  $psi.FileName = $ExePath
  $psi.Arguments = $arguments
  $psi.WorkingDirectory = $InstallDir
  $psi.UseShellExecute = $false
  $psi.RedirectStandardOutput = $true
  $psi.RedirectStandardError = $true
  $psi.CreateNoWindow = $true

  $p = [System.Diagnostics.Process]::Start($psi)
  $stdout = $p.StandardOutput.ReadToEnd()
  $stderr = $p.StandardError.ReadToEnd()
  $p.WaitForExit()

  $text = @()
  if ($stdout) { $text += $stdout.Trim() }
  if ($stderr) { $text += $stderr.Trim() }
  if ($p.ExitCode -ne 0) {
    throw "Command failed ($($p.ExitCode)): frpp.exe $arguments`r`n$($text -join "`r`n")"
  }
  return ($text -join "`r`n")
}

function Quote-Arg($value) {
  if ($null -eq $value) { return '""' }
  return '"' + ($value -replace '"', '\"') + '"'
}

function Get-CommandOption($command, $pattern) {
  $match = [regex]::Match($command, $pattern, [Text.RegularExpressions.RegexOptions]::IgnoreCase)
  if (-not $match.Success) {
    return ''
  }
  foreach ($name in @('dq', 'sq', 'bare')) {
    if ($match.Groups[$name].Success) {
      return $match.Groups[$name].Value.Trim()
    }
  }
  return ''
}

function Parse-ClientCommand($command) {
  $cmd = ($command -replace "`r", ' ' -replace "`n", ' ').Trim()
  if ([string]::IsNullOrWhiteSpace($cmd)) {
    throw 'Please paste the full client start command.'
  }
  if ($cmd -notmatch '(^|\s)client(\s|$)') {
    throw 'The command must include the client subcommand.'
  }

  $secret = Get-CommandOption $cmd '(?:^|\s)-s\s+(?:"(?<dq>[^"]+)"|''(?<sq>[^'']+)''|(?<bare>\S+))'
  $clientId = Get-CommandOption $cmd '(?:^|\s)-i\s+(?:"(?<dq>[^"]+)"|''(?<sq>[^'']+)''|(?<bare>\S+))'
  $apiUrl = Get-CommandOption $cmd '(?:^|\s)--api-url\s+(?:"(?<dq>[^"]+)"|''(?<sq>[^'']+)''|(?<bare>\S+))'
  $rpcUrl = Get-CommandOption $cmd '(?:^|\s)--rpc-url\s+(?:"(?<dq>[^"]+)"|''(?<sq>[^'']+)''|(?<bare>\S+))'

  if ([string]::IsNullOrWhiteSpace($secret)) { throw 'Missing -s secret in command.' }
  if ([string]::IsNullOrWhiteSpace($clientId)) { throw 'Missing -i client id in command.' }
  if ([string]::IsNullOrWhiteSpace($apiUrl)) { throw 'Missing --api-url in command.' }
  if ([string]::IsNullOrWhiteSpace($rpcUrl)) { throw 'Missing --rpc-url in command.' }

  return [pscustomobject]@{
    command = $cmd
    secret = $secret
    clientId = $clientId
    apiUrl = $apiUrl
    rpcUrl = $rpcUrl
  }
}

$cfg = Load-Config

$form = New-Object Windows.Forms.Form
$form.Text = 'frp-manager Client Manager'
$form.StartPosition = 'CenterScreen'
$form.Size = New-Object Drawing.Size(720, 560)
$form.MinimumSize = New-Object Drawing.Size(720, 560)

$font = New-Object Drawing.Font('Microsoft YaHei UI', 9)
$form.Font = $font

$title = New-Object Windows.Forms.Label
$title.Text = 'frp-manager client setup'
$title.Font = New-Object Drawing.Font('Microsoft YaHei UI', 14, [Drawing.FontStyle]::Bold)
$title.Location = New-Object Drawing.Point(20, 18)
$title.Size = New-Object Drawing.Size(660, 32)
$form.Controls.Add($title)

$statusLabel = New-Object Windows.Forms.Label
$statusLabel.Location = New-Object Drawing.Point(20, 55)
$statusLabel.Size = New-Object Drawing.Size(660, 24)
$form.Controls.Add($statusLabel)

function Add-Label($text, $x, $y) {
  $label = New-Object Windows.Forms.Label
  $label.Text = $text
  $label.Location = New-Object Drawing.Point($x, $y)
  $label.Size = New-Object Drawing.Size(120, 24)
  $form.Controls.Add($label)
  return $label
}

function Add-TextBox($x, $y, $width, $text) {
  $box = New-Object Windows.Forms.TextBox
  $box.Location = New-Object Drawing.Point($x, $y)
  $box.Size = New-Object Drawing.Size($width, 24)
  $box.Text = $text
  $form.Controls.Add($box)
  return $box
}

Add-Label 'Start command' 20 95 | Out-Null
$commandBox = New-Object Windows.Forms.TextBox
$commandBox.Location = New-Object Drawing.Point(150, 92)
$commandBox.Size = New-Object Drawing.Size(520, 92)
$commandBox.Multiline = $true
$commandBox.ScrollBars = 'Vertical'
$commandBox.Text = $(if ($cfg -and $cfg.command) { $cfg.command } else { $DefaultCommand })
$form.Controls.Add($commandBox)

$hintLabel = New-Object Windows.Forms.Label
$hintLabel.Text = 'Paste the full command copied from frp-manager, for example: frp-manager client -s ... -i ... --api-url ... --rpc-url ...'
$hintLabel.Location = New-Object Drawing.Point(150, 190)
$hintLabel.Size = New-Object Drawing.Size(520, 44)
$hintLabel.ForeColor = [Drawing.Color]::DimGray
$form.Controls.Add($hintLabel)

$logBox = New-Object Windows.Forms.TextBox
$logBox.Location = New-Object Drawing.Point(20, 330)
$logBox.Size = New-Object Drawing.Size(650, 170)
$logBox.Multiline = $true
$logBox.ScrollBars = 'Vertical'
$logBox.ReadOnly = $true
$form.Controls.Add($logBox)

function Add-Log($message) {
  $time = Get-Date -Format 'HH:mm:ss'
  $logBox.AppendText("[$time] $message`r`n")
}

function Refresh-Status {
  $statusLabel.Text = "$(Get-ServiceText)    Binary: $ExePath"
}

function Install-Client {
  try {
    $parsed = Parse-ClientCommand $commandBox.Text
    Save-Config $parsed.command $parsed.secret $parsed.clientId $parsed.apiUrl $parsed.rpcUrl

    Add-Log 'Stopping existing service...'
    try { Run-Frpp 'stop' | Out-Null } catch { Add-Log $_.Exception.Message }
    try { Run-Frpp 'uninstall' | Out-Null } catch { Add-Log $_.Exception.Message }

    $args = @(
      'install',
      'client',
      '-s', (Quote-Arg $parsed.secret),
      '-i', (Quote-Arg $parsed.clientId),
      '--api-url', (Quote-Arg $parsed.apiUrl),
      '--rpc-url', (Quote-Arg $parsed.rpcUrl)
    ) -join ' '

    Add-Log "Installing service..."
    Add-Log "Client ID: $($parsed.clientId)"
    Add-Log "API URL: $($parsed.apiUrl)"
    Add-Log "RPC URL: $($parsed.rpcUrl)"
    Add-Log "frpp.exe $args"
    $out = Run-Frpp $args
    if ($out) { Add-Log $out }

    Add-Log 'Starting service...'
    $out = Run-Frpp 'start'
    if ($out) { Add-Log $out }

    Add-Log 'Done.'
  } catch {
    Add-Log $_.Exception.Message
    [Windows.Forms.MessageBox]::Show($_.Exception.Message, 'frp-manager error') | Out-Null
  } finally {
    Refresh-Status
  }
}

function Uninstall-Client($deleteConfig) {
  try {
    Add-Log 'Stopping service...'
    try { Run-Frpp 'stop' | Out-Null } catch { Add-Log $_.Exception.Message }
    Add-Log 'Uninstalling service...'
    try { Run-Frpp 'uninstall' | Out-Null } catch { Add-Log $_.Exception.Message }
    if ($deleteConfig -and (Test-Path $ConfigPath)) {
      Remove-Item -Path $ConfigPath -Force
      Add-Log 'Configuration deleted.'
    }
  } catch {
    Add-Log $_.Exception.Message
  } finally {
    Refresh-Status
  }
}

function Add-Button($text, $x, $y, $width, $handler) {
  $button = New-Object Windows.Forms.Button
  $button.Text = $text
  $button.Location = New-Object Drawing.Point($x, $y)
  $button.Size = New-Object Drawing.Size($width, 34)
  $button.Add_Click($handler)
  $form.Controls.Add($button)
  return $button
}

Add-Button 'Install / Apply' 20 250 125 { Install-Client } | Out-Null
Add-Button 'Start' 155 250 95 {
  try { Add-Log (Run-Frpp 'start') } catch { Add-Log $_.Exception.Message }
  Refresh-Status
} | Out-Null
Add-Button 'Stop' 260 250 95 {
  try { Add-Log (Run-Frpp 'stop') } catch { Add-Log $_.Exception.Message }
  Refresh-Status
} | Out-Null
Add-Button 'Restart' 365 250 95 {
  try { Add-Log (Run-Frpp 'restart') } catch { Add-Log $_.Exception.Message }
  Refresh-Status
} | Out-Null
Add-Button 'Uninstall' 470 250 95 { Uninstall-Client $false } | Out-Null
Add-Button 'Delete config' 575 250 95 { Uninstall-Client $true } | Out-Null

Add-Button 'Refresh status' 20 290 125 { Refresh-Status } | Out-Null
Add-Button 'Open install folder' 155 290 150 {
  if (Test-Path $InstallDir) {
    Start-Process explorer.exe $InstallDir
  }
} | Out-Null

Refresh-Status
Add-Log 'Ready.'

[void]$form.ShowDialog()
