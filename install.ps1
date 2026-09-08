param(
    [string]$Version = $env:FRP_MANAGER_VERSION
)

# Download a versioned release from GitHub. Use -Version latest only when a floating release is desired.
if($PSVersionTable.PSVersion.Major -lt 5){
    Write-Host "Require PS >= 5,your PSVersion:"$PSVersionTable.PSVersion.Major -BackgroundColor DarkGreen -ForegroundColor White
    exit
}
if ([string]::IsNullOrWhiteSpace($Version)) {
    $Version = "latest"
}
$clientrepo = "XMRayLabs/Frp-Manager"
$architecture = $env:PROCESSOR_ARCHITEW6432
if ([string]::IsNullOrWhiteSpace($architecture)) {
    $architecture = $env:PROCESSOR_ARCHITECTURE
}
switch ($architecture.ToUpperInvariant()) {
    "AMD64" { $file = "frp-manager-windows-amd64.exe" }
    "ARM64" { $file = "frp-manager-windows-arm64.exe" }
    default {
        throw "Unsupported Windows architecture: $architecture. A 64-bit AMD64 or ARM64 system is required."
    }
}

#闁插秴顦叉潻鎰攽閼奉亜濮╅弴瀛樻煀
if (Test-Path "C:\frpp\frpp.exe") {
    Write-Host "frp manager client already exists, delete and reinstall" -BackgroundColor DarkGreen -ForegroundColor White
    C:/frpp/frpp.exe stop
    C:/frpp/frpp.exe uninstall
    Start-Sleep -Seconds 3
    Remove-Item "C:\frpp\frpp.exe" -Force
}

[Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12

$releasePath = "releases/download/$Version"
if ($Version -eq "latest") {
    $releasePath = "releases/latest/download"
}
$download = "https://github.com/$clientrepo/$releasePath/$file"
$checksumUrl = "https://github.com/$clientrepo/$releasePath/SHA256SUMS-core.txt"
$downloadPath = "C:\frpp.exe"
$checksumPath = Join-Path ([System.IO.Path]::GetTempPath()) "frp-manager-SHA256SUMS-core.txt"

Write-Host "Downloading directly from GitHub Releases: $download"
Invoke-WebRequest $download -OutFile $downloadPath
Invoke-WebRequest $checksumUrl -OutFile $checksumPath

$escapedFile = [Regex]::Escape($file)
$checksumLine = Get-Content $checksumPath | Where-Object { $_ -match "^([A-Fa-f0-9]{64})\s+\*?$escapedFile$" } | Select-Object -First 1
if (-not $checksumLine) {
    Remove-Item $downloadPath -Force -ErrorAction SilentlyContinue
    throw "Checksum for $file was not found in SHA256SUMS-core.txt"
}
$expectedHash = ([Regex]::Match($checksumLine, "^[A-Fa-f0-9]{64}")).Value.ToLowerInvariant()
$actualHash = (Get-FileHash $downloadPath -Algorithm SHA256).Hash.ToLowerInvariant()
if ($actualHash -ne $expectedHash) {
    Remove-Item $downloadPath -Force -ErrorAction SilentlyContinue
    throw "SHA256 verification failed for $file"
}
Write-Host "SHA256 verified: $actualHash"

New-Item -Path "C:\frpp" -ItemType Directory -ErrorAction SilentlyContinue
Move-Item -Path $downloadPath -Destination "C:\frpp\frpp.exe"
C:\frpp\frpp.exe install $args
C:\frpp\frpp.exe start
Write-Host "Enjoy It!" -BackgroundColor DarkGreen -ForegroundColor Red
