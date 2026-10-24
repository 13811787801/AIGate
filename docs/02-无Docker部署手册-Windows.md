# AIGate-Pro · 无 Docker 部署手册（Windows）

## 目标

在一台 Windows 机器上跑起 AIGate 主进程，访问健康检查接口。  
**不需要 Docker、不需要 JDK。**

## 前置

1. 已安装 **Git**
2. 已安装 **Go 1.22+**（`go version` 可用）
3. 已克隆本仓库

验证：

```powershell
go version
where.exe go
# 期望包含：C:\Program Files\Go\bin\go.exe
```

## 步骤

```powershell
cd AIGate
Copy-Item configs\config.example.yaml configs\config.yaml
# 编辑 configs\config.yaml：修改 jwt_secret 与 admin_password
.\scripts\dev-run.ps1
```

另开窗口验证：

```powershell
Invoke-RestMethod http://127.0.0.1:3000/healthz
```

期望返回含 `"status": "ok"`。

## 编译为单文件（便于拷贝到客户机）

```powershell
.\scripts\build-windows.ps1
# 得到 dist\aigate.exe
```

客户机只需：`aigate.exe` + `configs\config.yaml`（及后续的 data/logs 目录）。

## 常见问题

| 现象 | 处理 |
|---|---|
| 端口被占用 | 改 `server.port` 或结束占用进程 |
| 找不到 config | 用 `-c` 指定绝对路径 |
| go 不是命令 | 重开终端；检查 Go 安装 |

## Phase 0 说明

当前 `/v1/chat/completions` 返回 `501 not implemented` 是正常的；Phase 1 才会接通鉴权、渠道与上游调用。
