$ErrorActionPreference = "Stop"
Set-Location $PSScriptRoot\..

# 若已有进程占用 3000，先提示
$existing = Get-NetTCPConnection -LocalPort 3000 -State Listen -ErrorAction SilentlyContinue
if ($existing) {
  Write-Host "Port 3000 already in use by PID(s): $($existing.OwningProcess -join ', ')" -ForegroundColor Yellow
  Write-Host "Open http://127.0.0.1:3000/healthz in browser/preview." -ForegroundColor Green
  exit 0
}

function Resolve-Go {
  $cmd = Get-Command go -ErrorAction SilentlyContinue
  if ($cmd -and $cmd.Source) { return $cmd.Source }
  $machine = [System.Environment]::GetEnvironmentVariable("Path", "Machine")
  $user = [System.Environment]::GetEnvironmentVariable("Path", "User")
  $env:Path = "$machine;$user"
  Remove-Item Env:GOROOT -ErrorAction SilentlyContinue
  $cmd = Get-Command go -ErrorAction SilentlyContinue
  if ($cmd -and $cmd.Source) { return $cmd.Source }
  foreach ($p in @("C:\Program Files\Go\bin\go.exe", "E:\Projects\.toolchain\go\bin\go.exe")) {
    if (Test-Path $p) { return $p }
  }
  return $null
}

$goBin = Resolve-Go
if (-not $goBin) {
  Write-Host "未找到 go。请先安装 Go 1.22+ 并加入 PATH。" -ForegroundColor Red
  exit 1
}
Remove-Item Env:GOROOT -ErrorAction SilentlyContinue

if (-not (Test-Path "configs\config.yaml")) {
  Copy-Item "configs\config.example.yaml" "configs\config.yaml"
}

New-Item -ItemType Directory -Force -Path "data","logs","dist" | Out-Null
& $goBin build -o dist\aigate.exe .\cmd\aigate
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }

Write-Host "Starting AIGate in background..." -ForegroundColor Cyan
$p = Start-Process -FilePath "$PWD\dist\aigate.exe" -ArgumentList "-c","configs\config.yaml" -WorkingDirectory $PWD `
  -RedirectStandardOutput "logs\runtime.out.log" -RedirectStandardError "logs\runtime.err.log" -PassThru -WindowStyle Hidden

Start-Sleep -Seconds 2
if ($p.HasExited) {
  Write-Host "Start failed. See logs\runtime.err.log" -ForegroundColor Red
  Get-Content logs\runtime.err.log -ErrorAction SilentlyContinue
  exit 1
}

Write-Host "OK PID=$($p.Id)" -ForegroundColor Green
Write-Host "Preview / browser: http://127.0.0.1:3000/healthz" -ForegroundColor Green
Write-Host "Stop later: Stop-Process -Id $($p.Id)" -ForegroundColor DarkGray
