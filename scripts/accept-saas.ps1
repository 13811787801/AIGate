# saas 模式冒烟（临时实例端口 3001，不干扰默认 3000）
# 用法: .\scripts\accept-saas.ps1

$ErrorActionPreference = "Stop"
$root = Split-Path -Parent (Split-Path -Parent $MyInvocation.MyCommand.Path)
Set-Location $root

$go = "C:\Program Files\Go\bin\go.exe"
if (-not (Test-Path $go)) { $go = "go" }

New-Item -ItemType Directory -Force -Path "$root\data\saas-accept" | Out-Null
$cfg = @"
server:
  host: "127.0.0.1"
  port: 3001
  mode: "debug"
database:
  driver: "sqlite"
  sqlite_path: "./data/saas-accept/aigate.db"
auth:
  jwt_secret: "saas-accept-jwt-secret-32chars!!"
  admin_username: "admin"
  admin_password: "admin123"
  bootstrap_token: ""
security:
  data_key: "saas-accept-data-key-32chars!!"
  allow_insecure: true
billing:
  freeze_margin: 1.2
deploy:
  mode: "saas"
  product_name: "AIGate-SaaS-Accept"
modules:
  marketplace: true
  wallet: true
  playground: true
  admin_channels: true
  billing_online: false
  sso: false
  audit: false
  plugin_runtime: false
license:
  file: ""
  public_key_b64: ""
  require: false
plugin:
  enabled: false
log:
  level: "info"
  file: "./logs/saas-accept.log"
"@
$cfgPath = "$root\configs\config.saas.accept.yaml"
Set-Content -Path $cfgPath -Value $cfg -Encoding UTF8

# clean db for deterministic seed
Remove-Item "$root\data\saas-accept\*" -Force -ErrorAction SilentlyContinue

& $go build -o "$root\dist\aigate-saas-accept.exe" "$root\cmd\aigate"
$proc = Start-Process -FilePath "$root\dist\aigate-saas-accept.exe" -ArgumentList @("-c", $cfgPath) -PassThru -WindowStyle Hidden
Start-Sleep -Seconds 2

try {
  $base = "http://127.0.0.1:3001"
  $hz = Invoke-RestMethod "$base/healthz"
  if ($hz.deploy -ne "saas") { throw "deploy=$($hz.deploy)" }

  $login = Invoke-RestMethod -Uri "$base/api/auth/login" -Method POST -ContentType "application/json" -Body '{"username":"admin","password":"admin123"}'
  $h = @{ Authorization = "Bearer $($login.token)" }
  $s = Invoke-RestMethod -Uri "$base/api/session" -Headers $h
  if ($s.admin_surface -ne "vendor") { throw "surface=$($s.admin_surface)" }
  "OK TC-MODE-05 admin_surface=vendor"

  $plans = Invoke-RestMethod -Uri "$base/api/admin/plans" -Headers $h
  $free = ($plans.items | Where-Object { $_.code -eq "free" }).id
  $uname = "accept_free_" + [int](Get-Random -Maximum 99999)
  Invoke-RestMethod -Uri "$base/api/admin/users" -Method POST -Headers $h -ContentType "application/json" -Body (@{
    username=$uname; password="demo123"; plan_id=[int]$free; role="user"; balance=10
  } | ConvertTo-Json) | Out-Null

  $lf = Invoke-RestMethod -Uri "$base/api/auth/login" -Method POST -ContentType "application/json" -Body (@{username=$uname;password="demo123"} | ConvertTo-Json)
  if ($lf.modules.wallet -eq $true) { throw "free wallet should be false" }
  "OK TC-MODE-06 free wallet module=false"

  $hf = @{ Authorization = "Bearer $($lf.token)" }
  $denied = $false
  try { Invoke-RestMethod -Uri "$base/api/wallet" -Headers $hf | Out-Null } catch { $denied = $true }
  if (-not $denied) { throw "wallet should 403" }
  "OK TC-MODE-07 wallet 403"

  "ACCEPT-SAAS PASS"
}
finally {
  if ($proc -and -not $proc.HasExited) { Stop-Process -Id $proc.Id -Force }
}
