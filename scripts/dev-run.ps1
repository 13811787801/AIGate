$ErrorActionPreference = "Stop"
Set-Location $PSScriptRoot\..

$goBin = "E:\Projects\.toolchain\go\bin\go.exe"
if (-not (Test-Path $goBin)) {
  if (Get-Command go -ErrorAction SilentlyContinue) {
    $goBin = "go"
  } else {
    Write-Host "Go not found. Expected E:\Projects\.toolchain\go or system PATH." -ForegroundColor Red
    Write-Host "See docs/02-无Docker部署手册-Windows.md" -ForegroundColor Yellow
    exit 1
  }
}

if (-not (Test-Path "configs\config.yaml")) {
  Copy-Item "configs\config.example.yaml" "configs\config.yaml"
  Write-Host "Created configs\config.yaml from example." -ForegroundColor Yellow
}

New-Item -ItemType Directory -Force -Path "data","logs" | Out-Null
$env:GOROOT = "E:\Projects\.toolchain\go"
$env:PATH = "E:\Projects\.toolchain\go\bin;" + $env:PATH

Write-Host "Starting AIGate (dev)..." -ForegroundColor Cyan
& $goBin run .\cmd\aigate -c configs\config.yaml
