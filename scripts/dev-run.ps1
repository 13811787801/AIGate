$ErrorActionPreference = "Stop"
Set-Location $PSScriptRoot\..

function Resolve-Go {
  # 1) 系统 PATH 中的 go（正规安装）
  $cmd = Get-Command go -ErrorAction SilentlyContinue
  if ($cmd -and $cmd.Source) {
    return $cmd.Source
  }
  # 2) 刷新 Machine/User PATH 后再找一次（当前会话可能还是旧 PATH）
  $machine = [System.Environment]::GetEnvironmentVariable("Path", "Machine")
  $user = [System.Environment]::GetEnvironmentVariable("Path", "User")
  $env:Path = "$machine;$user"
  $cmd = Get-Command go -ErrorAction SilentlyContinue
  if ($cmd -and $cmd.Source) {
    return $cmd.Source
  }
  # 3) 常见安装路径
  $candidates = @(
    "C:\Program Files\Go\bin\go.exe",
    "$env:USERPROFILE\sdk\go\bin\go.exe",
    "E:\Projects\.toolchain\go\bin\go.exe"
  )
  foreach ($p in $candidates) {
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
& $goBin version

if (-not (Test-Path "configs\config.yaml")) {
  Copy-Item "configs\config.example.yaml" "configs\config.yaml"
  Write-Host "Created configs\config.yaml from example." -ForegroundColor Yellow
}

New-Item -ItemType Directory -Force -Path "data","logs" | Out-Null
Write-Host "Starting AIGate (dev)..." -ForegroundColor Cyan
& $goBin run .\cmd\aigate -c configs\config.yaml
