$ErrorActionPreference = "Stop"
Set-Location $PSScriptRoot\..

function Resolve-Go {
  $cmd = Get-Command go -ErrorAction SilentlyContinue
  if ($cmd -and $cmd.Source) { return $cmd.Source }
  $machine = [System.Environment]::GetEnvironmentVariable("Path", "Machine")
  $user = [System.Environment]::GetEnvironmentVariable("Path", "User")
  $env:Path = "$machine;$user"
  $cmd = Get-Command go -ErrorAction SilentlyContinue
  if ($cmd -and $cmd.Source) { return $cmd.Source }
  foreach ($p in @(
    "C:\Program Files\Go\bin\go.exe",
    "E:\Projects\.toolchain\go\bin\go.exe"
  )) {
    if (Test-Path $p) { return $p }
  }
  return $null
}

$goBin = Resolve-Go
if (-not $goBin) {
  Write-Host "未找到 go。请先安装 Go 1.22+ 并加入 PATH。" -ForegroundColor Red
  exit 1
}

Write-Host "Using Go: $goBin" -ForegroundColor DarkGray
New-Item -ItemType Directory -Force -Path "dist","data","logs" | Out-Null
& $goBin build -o dist\aigate.exe .\cmd\aigate
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
Write-Host "Built dist\aigate.exe" -ForegroundColor Green
