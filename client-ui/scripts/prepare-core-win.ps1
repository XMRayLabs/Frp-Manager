$ErrorActionPreference = 'Stop'

$ScriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$ClientUiRoot = Resolve-Path (Join-Path $ScriptDir '..')
$RepoRoot = Resolve-Path (Join-Path $ClientUiRoot '..')
$OutputDir = Join-Path $ClientUiRoot 'resources\binaries'
$Output = Join-Path $OutputDir 'frpp.exe'

$go = Get-Command go -ErrorAction SilentlyContinue
if ($null -eq $go) {
  $candidate = Join-Path $env:USERPROFILE '.cache\codex-go\go1.26.4\go\bin\go.exe'
  if (Test-Path $candidate) {
    $go = [pscustomobject]@{ Source = $candidate }
  }
}

if ($null -eq $go) {
  throw 'Go toolchain not found. Install Go 1.25+ or set go.exe on PATH.'
}

New-Item -ItemType Directory -Force -Path $OutputDir | Out-Null

Push-Location $RepoRoot
try {
  $buildDate = (Get-Date).ToUniversalTime().ToString('yyyy-MM-ddTHH:mm:ssZ')
  $gitCommit = (& git rev-parse HEAD 2>$null)
  if (-not $gitCommit) { $gitCommit = 'unknown' }
  $gitBranch = (& git rev-parse --abbrev-ref HEAD 2>$null)
  if (-not $gitBranch) { $gitBranch = 'unknown' }
  $version = $env:RELEASE_VERSION
  if (-not $version -and (Test-Path 'VERSION')) {
    $version = (Get-Content 'VERSION' -Raw).Trim()
  }
  if (-not $version) { $version = '1.0.0' }
  if ($version.StartsWith('v')) { $version = $version.Substring(1) }

  $ldflags = "-checklinkname=0 -X github.com/Sakurame1/frp-manager/conf.buildDate=$buildDate -X github.com/Sakurame1/frp-manager/conf.gitCommit=$gitCommit -X github.com/Sakurame1/frp-manager/conf.gitVersion=$version -X github.com/Sakurame1/frp-manager/conf.gitBranch=$gitBranch -X github.com/Sakurame1/frp-manager/conf.binaryType=client"

  $env:CGO_ENABLED = '0'
  $env:GOOS = 'windows'
  $env:GOARCH = 'amd64'
  & $go.Source build -o $Output -ldflags $ldflags ./cmd/frppc
  if ($LASTEXITCODE -ne 0) {
    exit $LASTEXITCODE
  }
  Write-Host "Built core client: $Output"
} finally {
  Pop-Location
}
