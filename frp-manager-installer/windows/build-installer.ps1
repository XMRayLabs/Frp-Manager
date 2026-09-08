$ErrorActionPreference = 'Stop'

$Root = Split-Path -Parent $MyInvocation.MyCommand.Path
$Script = Join-Path $Root 'frp-manager-client.iss'
$Client = Join-Path $Root 'files\frpp.exe'

if (-not (Test-Path $Client)) {
  throw "Missing client binary: $Client. Run .\download-client.ps1 first."
}

$candidates = @(
  (Get-Command iscc.exe -ErrorAction SilentlyContinue | Select-Object -ExpandProperty Source -ErrorAction SilentlyContinue),
  'C:\Program Files (x86)\Inno Setup 6\ISCC.exe',
  'C:\Program Files\Inno Setup 6\ISCC.exe',
  'C:\Program Files (x86)\Inno Setup 5\ISCC.exe',
  'C:\Program Files\Inno Setup 5\ISCC.exe'
) | Where-Object { $_ -and (Test-Path $_) }

if (-not $candidates -or $candidates.Count -eq 0) {
  throw 'ISCC.exe not found. Please install Inno Setup or add ISCC.exe to PATH.'
}

$iscc = $candidates[0]
Write-Host "Using Inno Setup compiler: $iscc"
& $iscc $Script

if ($LASTEXITCODE -ne 0) {
  exit $LASTEXITCODE
}

Write-Host 'Installer build completed.'
