$ErrorActionPreference = "Stop"
Set-Location $PSScriptRoot\..
$target = "reference\one-api"
if (Test-Path $target) {
  Write-Host "Already exists: $target" -ForegroundColor Yellow
  exit 0
}
New-Item -ItemType Directory -Force -Path "reference" | Out-Null
git clone --depth 1 https://github.com/songquanpeng/one-api.git $target
Write-Host "Cloned One API (MIT) into $target" -ForegroundColor Green
