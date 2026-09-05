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

## Go 工具链（本机）

**推荐**：系统安装（winget/MSI）→ `C:\Program Files\Go\bin\go.exe`  
详见：[docs/03-Go环境安装与使用手册.md](docs/03-Go环境安装与使用手册.md)

备用便携目录（可选）：`E:\Projects\.toolchain\go`  
脚本优先用系统 `go`，找不到再回退便携版。


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
cd E:\Projects\AIGate
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
| [商业化对标与阶段门禁](docs/08-商业化对标与阶段门禁.md) | **Mock→商业化强制门禁** |
| [Phase2 控制台说明](docs/01-Phase2控制台说明.md) | 打开网页从这里 |
| [Phase1 验收与本地模型](docs/01-Phase1验收与接本地模型.md) | API / Ollama 联调 |
| [Go 环境安装与使用（白丁版）](docs/03-Go环境安装与使用手册.md) | winget/MSI、验收清单 |
| [Go 避坑与排障速查](docs/04-Go开发避坑与排障速查.md) | 报错解读、语法对照、常犯错误 |
| [AIGate-Pro技术规格书](docs/AIGate-Pro技术规格书.md) | 产品与领域模型 V4 |
| [架构补丁 V4.1](docs/架构补丁-V4.1-Go与Python插件.md) | Go + Python 插件 + 本地模型 |
| [技术选型与部署学习](docs/技术选型决议与部署学习规范.md) | 选型决议、排障树 |
| [无 Docker 部署 · Windows](docs/02-无Docker部署手册-Windows.md) | 安装步骤 |
| [One API 体检结论](docs/07-OneAPI体检结论.md) | 不整仓 Fork，参考实现 |
| [开源参考说明](docs/06-开源协议与合规.md) | One API MIT / 禁止默认 AGPL |

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

本仓库业务代码默认专有（私有化交付）；参考目录 `reference/one-api` 遵循其 MIT 协议与署名要求。详见 `docs/06-开源协议与合规.md`。
