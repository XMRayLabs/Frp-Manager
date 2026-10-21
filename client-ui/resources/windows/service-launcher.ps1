param([Parameter(Mandatory=$true)][string]$RequestPath)
$ErrorActionPreference = 'Stop'
[Console]::OutputEncoding = [System.Text.UTF8Encoding]::new($false)
try {
  $helper = Join-Path $PSScriptRoot 'service-action.ps1'
  if ($RequestPath.Contains('"') -or $RequestPath.Contains([char]10) -or $RequestPath.Contains([char]13)) { throw 'Invalid request path' }
  $arguments = @('-NoProfile', '-NonInteractive', '-ExecutionPolicy', 'Bypass', '-File', ('"' + $helper + '"'), '-RequestPath', ('"' + $RequestPath + '"'))
  $process = Start-Process -FilePath (Join-Path $PSHOME 'powershell.exe') -Verb RunAs -Wait -PassThru -WindowStyle Hidden -ArgumentList $arguments
  $outputPath = Join-Path (Split-Path -Parent $RequestPath) 'service-output.txt'
  if (Test-Path -LiteralPath $outputPath) { Get-Content -LiteralPath $outputPath -Raw -Encoding UTF8 }
  exit [int]$process.ExitCode
} catch {
  Write-Error $_.Exception.Message
  if ($_.Exception.NativeErrorCode -eq 1223) { exit 1223 }
  exit 1
}
