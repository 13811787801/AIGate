$ErrorActionPreference = "Stop"
Set-Location $PSScriptRoot\..
$env:GOROOT = "E:\Projects\.toolchain\go"
$env:PATH = "E:\Projects\.toolchain\go\bin;" + $env:PATH
New-Item -ItemType Directory -Force -Path "dist","data","logs" | Out-Null
go build -o dist\aigate.exe .\cmd\aigate
Write-Host "Built dist\aigate.exe" -ForegroundColor Green
