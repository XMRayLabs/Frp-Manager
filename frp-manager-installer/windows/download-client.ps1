$ErrorActionPreference = 'Stop'

$Root = Split-Path -Parent $MyInvocation.MyCommand.Path
$FilesDir = Join-Path $Root 'files'
$OutFile = Join-Path $FilesDir 'frpp.exe'
$Version = if ($env:FRP_MANAGER_VERSION) { $env:FRP_MANAGER_VERSION } else { '1.0.0' }
$Url = "https://github.com/XMRayLabs/Frp-Manager/releases/download/$Version/frp-manager-client-windows-amd64.exe"

New-Item -ItemType Directory -Force -Path $FilesDir | Out-Null

Write-Host "Downloading frp-manager client..."
Write-Host $Url

[Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12
Invoke-WebRequest -Uri $Url -OutFile $OutFile -UseBasicParsing

$hash = Get-FileHash -Path $OutFile -Algorithm SHA256
Write-Host "Downloaded to: $OutFile"
Write-Host "SHA256: $($hash.Hash)"
