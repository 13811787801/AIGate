# AIGate-Pro

可私有化部署的企业级 AI API 网关（OpenAI 兼容）。

> **技术路线（已锁定）**：Go 主网关 + Python 后期插件 + 本地/云模型统一渠道。  
> 规格见 `docs/`。

## 当前进度

| 阶段 | 状态 |
|---|---|
| Phase 0 建仓 / 骨架 / 健康检查 | ✅ |
| One API 体检 | ✅ 不整仓 Fork |
| **Phase 1 鉴权·渠道·计费·Chat** | ✅ |
| **Phase 2 控制台 UI** | **✅ 已完成（内嵌网页控制台）** |
| Phase 4+ Python 插件 | 未开始 |

打开控制台：**http://127.0.0.1:3000/**（先 `.\scripts\start-background.ps1`）  
说明：[docs/01-Phase2控制台说明.md](docs/01-Phase2控制台说明.md)

## Go 工具链

安装 **Go 1.22+**，并确认终端里 `go version` 可用。  
Windows 脚本优先使用系统里的 `go`。


## 仓库结构

```text
AIGate/
├── cmd/aigate/          # 主程序入口
├── internal/            # 业务包（auth/billing/channel/...）
├── pkg/                 # 可复用小库
├── configs/             # 配置样例
├── deploy/              # Windows / Linux / Compose 部署
├── docs/                # 规格、选型、排障、部署手册
├── reference/one-api/   # MIT 参考实现（只读参考，不直接当生产依赖）
├── scripts/             # 开发脚本
└── web/                 # 前端（后续）
```

## 快速开始（Windows，无 Docker）

### 1. 安装 Go

已用 winget 安装 **Go 1.22+**（本机若尚未生效，重开终端后执行 `go version`）。

### 2. 配置

```powershell
cd AIGate
Copy-Item configs\config.example.yaml configs\config.yaml
```

按需改 `configs\config.yaml` 里的 `auth.jwt_secret` / 管理员密码。

### 3. 运行

```powershell
.\scripts\dev-run.ps1
```

或：

```powershell
go run .\cmd\aigate -c configs\config.yaml
```

### 4. 验证

浏览器或 PowerShell：

```powershell
Invoke-RestMethod http://127.0.0.1:3000/healthz
```

## 文档索引

| 文档 | 说明 |
|---|---|
| [控制台说明](docs/01-Phase2控制台说明.md) | 打开网页从这里 |
| [验收与本地模型](docs/01-Phase1验收与接本地模型.md) | API / Ollama 联调 |
| [技术规格书](docs/AIGate-Pro技术规格书.md) | 产品与领域模型 |
| [架构补丁 V4.1](docs/架构补丁-V4.1-Go与Python插件.md) | Go + Python 插件 + 本地模型 |
| [无 Docker 部署 · Windows](docs/02-无Docker部署手册-Windows.md) | 安装步骤 |
| [数据库备份与恢复](docs/15-数据库备份与恢复手册.md) | SQLite / MySQL |
| [开源协议与合规](docs/06-开源协议与合规.md) | MIT；禁止把 AGPL 代码混入本仓库 |

拉取参考代码（可选）：

```powershell
.\scripts\fetch-reference.ps1
```

## 设计原则（短）

1. 主链路 **不依赖** Python 插件（fail-open）。  
2. **本地大模型** = `openai_compatible` 渠道。  
3. 计费：**预扣冻结 → 实结**（Phase 1）。  
4. 排查顺序：日志 traceId → Key → 限流 → 渠道（本地/云）→ 上游。

## License

本仓库的服务端与网页控制台以 [MIT](LICENSE) 发布。  
后续的本地部署客户端不在本仓库发布。  
参考目录 `reference/one-api` 若单独拉取，仍遵循其上游 MIT 与署名要求。详见 `docs/06-开源协议与合规.md`。
