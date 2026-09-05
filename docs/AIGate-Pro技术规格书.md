# AIGate-Pro 企业级 AI API 网关 · 技术规格书

| 项 | 内容 |
|---|---|
| 版本 | **V4.0（严格评审修订版）** + **[V4.1 架构补丁](./架构补丁-V4.1-Go与Python插件.md)** |
| 替代 | V3.0（2026-09-03 DeepSeek 初稿） |
| 最后更新 | 2026-09-05（V4.1：Go 主栈 + Python 插件 + 本地模型渠道） |
| 目标读者 | 技术负责人、架构师、产品经理、AI 编程助手（Cursor） |
| 文档性质 | **可执行的立项与施工规格**：产品边界 + 分期范围 + 技术约束 + 数据模型 + API 契约 |
| 评审结论 | **V3 不可原样开工**。方向正确，但 MVP 过重、对标失真、护城河错位、技术强制过激、计费存在资金风险。V4 收敛为可私有化交付的企业网关。 |
| V4.1 覆盖 | 技术栈改为 **Go 主功能**；Python 后期插件；**本地大模型 = Channel**；原 Java/WebFlux 条款作废 |

---

## 〇、评审总裁定（必须先读）

### 0.1 一句话产品定义（锁定）

> **AIGate-Pro = 可私有化部署的 OpenAI 兼容企业 AI API 网关**  
> 企业把上游大模型 Key 配进平台，对内发子密钥，统一鉴权、限流、路由、计费、审计。

不是：又一个聊天站、又一个 SaaS Token 倒卖站、又一个“语义缓存降本 80%”故事。

### 0.2 V3 → V4 必须改什么

| 裁定 | 原因 | V4 动作 |
|---|---|---|
| ❌ 语义缓存当护城河 | 未验证；实现重；企业采购不买单“降本 80%”口号 | 降为 **P3 可选增强**；卖点改为私有化 + 渠道稳定 + 企业权限审计 |
| ❌ 对标表“对方没有 XX” | 实地看过智信 TokenHub（New API 系 SaaS）后，多项误判 | 重写对标：对方强在协议广度与运营；我们强在私有化与企业治理 |
| ❌ WebFlux+R2DBC 全局强制 | 管理端/报表/充值强 CRUD，R2DBC 成本高、事务弱 | **网关可选响应式；业务面默认 MVC + JDBC/MyBatis** |
| ❌ 三服务拆分过早 | gateway/admin-api/user-api 首期交付过重 | **Phase 1–2 单体后端 + 两套前端**，跑通后再拆 |
| ❌ 仅 `/v1/chat/completions` + `X-API-Key` | 无法对接 Cursor/LangChain/Cherry Studio 等主流工具链 | **Bearer + `/v1/models` 为 P0**；协议面分期扩展 |
| ❌ 异步 MQ 扣费作唯一真相 | 超扣、欠费、流式失败难回滚 | **预扣/冻结 → 实结多退少补**；MQ 只做日志对账 |
| ❌ MySQL VECTOR 默认 | 私有化环境不可控 | MVP **Redis 精确缓存**；向量方案后置且优先 PG+pgvector |
| ❌ 24 周塞满支付/分销/K8s/语义缓存 | 小团队必延期、质量崩 | **MVP 10–12 周可交付**；增强功能后置 |
| ⚠️ 缺 Channel 概念 | TokenHub/New API 的核心是「上游渠道池」 | **Channel / Model / Group 三层模型为 P0** |
| ⚠️ 企业能力缺口 | ToB 采购看 SSO、组织、审计、License | 组织/审计进 P1；SSO/OIDC 进 P2 |

### 0.3 成功标准（验收用）

1. 客户机房 Docker Compose 一键起来，数据不出域。  
2. 管理员配置 ≥2 个上游渠道（如 DeepSeek + 通义），对外暴露统一模型名。  
3. 业务侧只改 Base URL + API Key，即可用 OpenAI SDK / Cursor 调通流式对话。  
4. 子密钥可限额、限模型、限 IP；余额不足被拒绝；日志可对账。  
5. 压测：单节点稳态 ≥ 目标 QPS（立项时填数），无余额穿透超扣。

---

## 一、项目战略定位

### 1.1 产品定义

AIGate-Pro 是面向企业的 **私有化 AI API 网关与配额治理平台**。

它解决四类真实买单痛点：

1. **多模型接入成本**：各厂商 SDK/鉴权/计费不一致。  
2. **密钥与成本失控**：部门乱配 Key、无法按人/项目计量。  
3. **稳定性**：单渠道故障导致业务中断。  
4. **合规**：数据与日志需留在企业内网，需审计。

### 1.2 目标客户与非目标

| 类型 | 说明 |
|---|---|
| ✅ 目标 | 中大型企业信息化/AI 中台；有内网部署要求的政企与金融；ISV 给客户做私有化交付 |
| ✅ 买单人 | IT/信息安全、数字化负责人、AI 平台负责人 |
| ❌ 非目标（首期） | C 端 Token 倒卖、分销拉新、公有云低价聚合站 |
| ❌ 不做（首期） | 完整 Agent 编排 IDE、知识库 RAG 全家桶、视频生成素材中台 |

### 1.3 真实差异化（V4）

| 维度 | AIGate-Pro 定位 |
|---|---|
| 部署 | **私有化优先**（Compose 必交；Helm 后置） |
| 模型 | 国产优先（DeepSeek / 通义 / 豆包 / 文心）+ OpenAI 兼容上游 |
| 产品形态 | 管理后台 + 用户控制台 + OpenAI 兼容网关 |
| 计费 | Token/按次 + 分组倍率 + 额度钱包（兑换码先于在线支付） |
| **护城河（可讲、可验）** | **数据不出域 + 上游渠道池高可用 + 企业配额/审计/权限** |

禁止对外承诺：「语义缓存降本 80%」——无基准测试不得写进销售材料。

### 1.4 市场对标（修订，基于智信 TokenHub 实地）

**事实**：`tk.wxdatas.cn` 为 New API 系产品的 SaaS 封装，强项是「换 Base URL + 多协议兼容 + 分组倍率运营」。

| 能力 | 智信 TokenHub（SaaS/New API 系） | AIGate-Pro（我们） |
|---|---|---|
| 协议广度 | 很强（Chat/Responses/Claude/Gemini/图/音/视频等） | MVP 只保证 Chat + Models；按商机扩展 |
| 渠道/分组/倍率 | 成熟 | **必须对齐其核心逻辑** |
| 令牌治理 | 成熟（额度/模型白名单/IP/过期） | **对齐** |
| 用户台体验 | 完整（Playground/日志/钱包/分销） | 对齐主路径；砍分销/素材 |
| 私有化 | 对方是 SaaS；New API 可自建但协议需合规评估 | **我们主打交付与企业治理** |
| 企业 SSO/组织/审计 | SaaS 场景弱/不可见 | **我们补强** |
| 语义缓存 | 未见产品级「向量相似度缓存」；有的是厂商 Prompt Cache 计费 | 可选增强，非卖点 |

**抄什么**：使用逻辑与信息架构（落地页一句话、令牌中心、模型广场、日志对账）。  
**不抄什么**：视觉皮肤、文案、对方专有接口实现、违规二开 AGPL 闭源商用。

---

## 二、领域模型（P0 概念，写进所有设计）

必须严格区分三层，禁止把「模型配置」和「上游 Key」揉成一张表糊弄过去。

```
User / Org ──拥有──► ApiKey(子密钥)
    │                    │
    │                    ├─ group（计费倍率组）
    │                    ├─ allowed_models
    │                    ├─ rpm/tpm / quota / balance
    │                    └─ ip_whitelist / expire
    │
    └── Group(倍率) ──决定──► 可见模型集合 & 计价系数

对外 Model（alias，如 deepseek-chat）
    └── 路由到一个或多个 Channel（上游实例）
            ├── provider / endpoint / upstream_key(加密)
            ├── weight / priority / health
            └── 故障时切换下一 Channel
```

| 概念 | 含义 | 例子 |
|---|---|---|
| **Channel** | 一条上游通道（一个 BaseURL + 一把上游 Key） | 通义正式 Key、DeepSeek 备用 Key |
| **Model** | 对外模型名（客户请求里的 `model`） | `deepseek-chat`、`qwen-plus` |
| **Group** | 用户/令牌计费与可见性分组 | `default=1.0x`、`premium=1.2x` |
| **ApiKey** | 发给内部用户/系统的子密钥 | `sk-...` |
| **Wallet** | 账户额度（人民币或内部积分，需统一） | 余额、冻结额、已消费 |

---

## 三、功能需求矩阵（按优先级裁剪）

### 3.1 优先级定义

| 级 | 含义 |
|---|---|
| **P0** | MVP 不上线就不收钱 |
| **P1** | MVP 同期或紧随，私有化交付竞争力 |
| **P2** | 商业版/第二阶段 |
| **P3** | 探索项，禁止挤占 P0 排期 |

### 3.2 网关核心（最重要）

| 模块 | 功能点 | 优先级 |
|---|---|---|
| OpenAI 兼容 | `POST /v1/chat/completions`（流式 SSE / 非流式） | P0 |
| 模型列表 | `GET /v1/models` | P0 |
| 鉴权 | `Authorization: Bearer sk-...`（兼容可选 `X-API-Key`） | P0 |
| 鉴权校验 | Key 状态/过期、模型白名单、IP 白名单、余额/配额 | P0 |
| 渠道路由 | 按 model → channels；权重；失败切换 | P0 |
| 限流 | 按 Key 的 RPM/TPM；Redis + Lua | P0 |
| 计费 | 预扣冻结 + 实结；分组倍率；按 Token/按次 | P0 |
| 日志 | 请求元数据日志（traceId、token、费用、耗时、状态）；默认不落全文 | P0 |
| 适配器 | OpenAI 兼容上游、DeepSeek、通义、豆包（文心可 P1） | P0 |
| 健康检查 | 渠道主动探测 + 自动摘除/恢复 | P1 |
| 熔断 | 错误率熔断 + 备用渠道 | P1 |
| 精确缓存 | 同 model + 同 messages hash 的响应缓存（可关） | P2 |
| 内容安全 | 敏感词 / 基础注入特征拦截 | P2 |
| 语义缓存 | Embedding 相似度命中 | P3 |
| 协议扩展 | embeddings / images / anthropic messages / responses | P2（按客户买单排期） |

### 3.3 管理后台

| 模块 | 功能点 | 优先级 |
|---|---|---|
| 认证 | 管理员登录 JWT；操作审计 | P0 |
| 渠道管理 | CRUD Channel；上游 Key AES 加密；启用/禁用；权重 | P0 |
| 模型管理 | 对外模型名、映射渠道、定价（input/output/按次）、启用 | P0 |
| 用户管理 | 列表、启用禁用、重置密码、分配分组 | P0 |
| 令牌监管 | 查看/禁用任意用户子密钥 | P0 |
| 分组管理 | 分组名、倍率、可访问模型集 | P0 |
| 仪表盘 | 调用量、费用、错误率、Top 模型/用户 | P1 |
| 兑换码 | 生成/作废/面额/次数/有效期 | P1 |
| 系统设置 | 站点名、注册开关、默认限额、日志保留天数 | P1 |
| 组织/项目 | 部门或项目空间、成员归属 | P1 |
| 报表导出 | 日/月/用户/模型 CSV | P1 |
| SSO | OIDC/LDAP 登录 | P2 |
| 在线支付配置 | 微信/支付宝商户参数 | P2 |
| 分销 | 返利体系 | P3（默认不做） |

### 3.4 用户控制台

| 模块 | 功能点 | 优先级 |
|---|---|---|
| 登录 | 账密登录（注册是否开放由管理端开关） | P0 |
| 令牌管理 | 创建/改名/删除/禁用；额度、模型、IP、过期 | P0 |
| 使用日志 | 时间/模型/Token/费用/耗时/状态/IP；筛选导出 | P0 |
| 钱包 | 余额、冻结、累计消费、请求数 | P0 |
| 模型广场 | 可见模型、计价说明、分组提示 | P0 |
| Playground | 选模型、调参、流式对话、查看原始请求/响应 | P1 |
| 兑换码充值 | 输入兑换码入账 | P1 |
| 个人设置 | 改密、通知阈值（余额预警） | P1 |
| 在线支付 | 微信/支付宝 | P2 |
| 任务日志 | 异步任务（图/视频类） | P2（无协议扩展则不做） |
| 素材库/分销 | — | P3（砍） |

### 3.5 明确砍掉或后置（防止范围蔓延）

- 分销中心、素材库、视频任务中台  
- 语义缓存、完整 Prompt 注入攻防平台  
- 首期 K8s Helm（先 Compose）  
- 首期三服务拆分、强制全站 R2DBC  
- 首期对齐 TokenHub 全协议面

---

## 四、技术栈规范（V4）

### 4.1 推荐默认栈（稳健交付）

| 组件 | 选型 | 说明 |
|---|---|---|
| JDK | **Java 21** | 虚拟线程可用；LTS |
| 应用框架 | **Spring Boot 3.3/3.4.x + Spring MVC** | 业务面默认阻塞模型，简单可靠 |
| 高并发可选 | 网关模块可用 WebClient 调上游；极端场景再评估 WebFlux | **禁止**在未评估团队能力时全局强制 WebFlux |
| DB 访问 | **MyBatis-Plus 或 Spring Data JDBC** | 禁止首期上 R2DBC |
| DB | **MySQL 8.0** | 不依赖 VECTOR；需要向量时另选 PG |
| 缓存/限流 | Redis 7 + Lua / Redisson | Key 会话、限流、冻结额度 |
| MQ | **可选** RabbitMQ/Redis Stream | 仅日志异步、报表；**不负责余额唯一扣减** |
| 前端 | Vue 3 + Vite + Pinia + Vue Router + Axios + Ant Design Vue | admin-ui / user-ui |
| 文档 | springdoc-openapi | |
| 部署 | **Docker Compose 必交**；Helm = P2 | |
| 观测 | Micrometer + Prometheus + Grafana | `/actuator/prometheus` |

### 4.2 架构形态（分期）

**Phase 1–2（强制）**：

```
aigate-pro/                 # 单体后端一个 Boot 进程
├── aigate-common/
├── aigate-domain/          # 实体、仓储、领域服务
├── aigate-gateway/         # 对外 /v1/** 网关处理（可同进程）
├── aigate-admin/           # /api/admin/**
├── aigate-user/            # /api/user/**
├── aigate-admin-ui/
├── aigate-user-ui/
└── deploy/docker-compose.yml
```

**Phase 3+（有流量再拆）**：网关进程独立水平扩展；管理/用户 API 可仍同进程。

### 4.3 技术禁令

1. 禁止在请求关键路径用「先返回成功再异步扣费」且无冻结。  
2. 禁止明文存储上游 API Key；必须 AES-GCM，密钥来自环境变量。  
3. 禁止把 Prompt/Response 全文默认入库（隐私与磁盘）；调试需显式开关且有保留期。  
4. 禁止引入协议不清的二开底座做商业私有化交付。  
5. 禁止尚未签单就实现对方全量多模态协议。

### 4.4 若坚持 WebFlux 的唯一合法条件

仅当：**专职网关进程**、团队具备 Reactor 排障能力、业务 API 仍用 MVC。  
即便如此，**计费与余额必须用同步原子操作或 Lua**，不得交给 MQ 碰运气。

---

## 五、计费与额度（资金安全规格）

### 5.1 账户模型

- 用户级 `balance`（可用）+ `frozen`（冻结）。  
- 令牌可设 `quota_total/quota_used`；也可设令牌级余额（若为空则走用户余额）。  
- `Group.multiplier` 作用于结算价。  
- 计价：`TOKEN`（input/output 单价）或 `REQUEST`（按次）。

### 5.2 扣费状态机（P0）

```
请求进入
  → 鉴权通过
  → 估算上限费用（max_tokens * 单价 * 倍率 + 安全边际）
  → 原子冻结 frozen += estimate（余额不足则 402/429 业务码拒绝）
  → 调上游
  → 成功：按真实 usage 结算；多退少补；写 request_log
  → 失败/超时：解冻；写失败日志（可计少量失败费，默认不计）
```

流式：在流结束（或客户端中断）时结算；中断按下发 token 实结。

### 5.3 对账

- Redis 当日用量可作展示加速。  
- MySQL `request_log` + `ledger`（可选流水表）为财务真相。  
- 每日任务：余额 vs 流水重放校验。

---

## 六、数据库设计（核心表）

> 下列为逻辑模型。实现时用 Flyway；字段可微调，**概念不可丢**。

### 6.1 用户与权限

```sql
CREATE TABLE admin_user (
  id            BIGINT PRIMARY KEY AUTO_INCREMENT,
  username      VARCHAR(50) NOT NULL UNIQUE,
  password_hash VARCHAR(255) NOT NULL,
  role          VARCHAR(32) NOT NULL DEFAULT 'ADMIN', -- ADMIN / SUPER_ADMIN
  status        TINYINT NOT NULL DEFAULT 1,
  created_at    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
);

CREATE TABLE org_unit (
  id         BIGINT PRIMARY KEY AUTO_INCREMENT,
  name       VARCHAR(100) NOT NULL,
  parent_id  BIGINT NULL,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE user_group (
  id          BIGINT PRIMARY KEY AUTO_INCREMENT,
  name        VARCHAR(50) NOT NULL UNIQUE,
  multiplier  DECIMAL(8,4) NOT NULL DEFAULT 1.0000,
  description VARCHAR(255) NULL,
  created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE `user` (
  id             BIGINT PRIMARY KEY AUTO_INCREMENT,
  username       VARCHAR(50) NOT NULL UNIQUE,
  password_hash  VARCHAR(255) NOT NULL,
  display_name   VARCHAR(100) NULL,
  email          VARCHAR(100) NULL,
  phone          VARCHAR(20) NULL,
  org_id         BIGINT NULL,
  group_id       BIGINT NOT NULL DEFAULT 1,
  status         TINYINT NOT NULL DEFAULT 1,
  balance        DECIMAL(14,6) NOT NULL DEFAULT 0,
  frozen         DECIMAL(14,6) NOT NULL DEFAULT 0,
  total_consumed DECIMAL(14,6) NOT NULL DEFAULT 0,
  created_at     DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at     DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  INDEX idx_user_group (group_id),
  INDEX idx_user_org (org_id)
);
```

### 6.2 令牌

```sql
CREATE TABLE api_key (
  id             BIGINT PRIMARY KEY AUTO_INCREMENT,
  user_id        BIGINT NOT NULL,
  name           VARCHAR(100) NOT NULL,
  key_hash       VARCHAR(128) NOT NULL,      -- 只存哈希；明文仅创建时返回一次
  key_prefix     VARCHAR(16) NOT NULL,       -- 展示用 sk-xxxx
  group_id       BIGINT NULL,                -- 空则继承用户分组
  quota_total    BIGINT NULL,                -- Token 配额，空=不限
  quota_used     BIGINT NOT NULL DEFAULT 0,
  balance        DECIMAL(14,6) NULL,         -- 令牌独立余额，空=用用户余额
  rpm_limit      INT NOT NULL DEFAULT 60,
  tpm_limit      INT NOT NULL DEFAULT 100000,
  allowed_models JSON NULL,
  ip_whitelist   JSON NULL,
  status         TINYINT NOT NULL DEFAULT 1,
  expired_at     DATETIME NULL,
  last_used_at   DATETIME NULL,
  created_at     DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  UNIQUE KEY uk_key_hash (key_hash),
  INDEX idx_api_key_user (user_id)
);
```

> 安全要求：数据库 **禁止存 API Key 明文**。V3 的 `key VARCHAR` 明文方案废止。

### 6.3 渠道与模型

```sql
CREATE TABLE channel (
  id              BIGINT PRIMARY KEY AUTO_INCREMENT,
  name            VARCHAR(100) NOT NULL,
  provider        VARCHAR(50) NOT NULL,   -- openai_compatible / deepseek / tongyi / doubao / ...
  base_url        VARCHAR(255) NOT NULL,
  api_key_enc     VARCHAR(1024) NOT NULL, -- AES-GCM
  weight          INT NOT NULL DEFAULT 100,
  priority        INT NOT NULL DEFAULT 0,
  status          TINYINT NOT NULL DEFAULT 1, -- 1启用 0禁用 2熔断
  timeout_ms      INT NOT NULL DEFAULT 120000,
  health_path     VARCHAR(255) NULL,
  last_health_at  DATETIME NULL,
  last_error      VARCHAR(500) NULL,
  created_at      DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at      DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
);

CREATE TABLE model_config (
  id                   BIGINT PRIMARY KEY AUTO_INCREMENT,
  model_name           VARCHAR(100) NOT NULL UNIQUE, -- 对外名
  display_name         VARCHAR(100) NULL,
  billing_mode         VARCHAR(20) NOT NULL DEFAULT 'TOKEN', -- TOKEN / REQUEST
  input_price_per_1k   DECIMAL(14,8) NOT NULL DEFAULT 0,
  output_price_per_1k  DECIMAL(14,8) NOT NULL DEFAULT 0,
  price_per_request    DECIMAL(14,8) NULL,
  max_tokens           INT NOT NULL DEFAULT 8192,
  supports_stream      TINYINT NOT NULL DEFAULT 1,
  enabled              TINYINT NOT NULL DEFAULT 1,
  created_at           DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at           DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
);

-- 模型 ↔ 渠道（多对多，支撑故障转移）
CREATE TABLE model_channel (
  id              BIGINT PRIMARY KEY AUTO_INCREMENT,
  model_config_id BIGINT NOT NULL,
  channel_id      BIGINT NOT NULL,
  upstream_model  VARCHAR(100) NOT NULL, -- 上游真实模型名
  weight          INT NOT NULL DEFAULT 100,
  enabled         TINYINT NOT NULL DEFAULT 1,
  UNIQUE KEY uk_model_channel (model_config_id, channel_id)
);

-- 分组可见模型
CREATE TABLE group_model (
  id              BIGINT PRIMARY KEY AUTO_INCREMENT,
  group_id        BIGINT NOT NULL,
  model_config_id BIGINT NOT NULL,
  UNIQUE KEY uk_group_model (group_id, model_config_id)
);
```

### 6.4 日志、账本、兑换

```sql
CREATE TABLE request_log (
  id              BIGINT PRIMARY KEY AUTO_INCREMENT,
  trace_id        VARCHAR(64) NOT NULL,
  user_id         BIGINT NOT NULL,
  api_key_id      BIGINT NOT NULL,
  model_name      VARCHAR(100) NOT NULL,
  channel_id      BIGINT NULL,
  group_id        BIGINT NULL,
  input_tokens    INT NOT NULL DEFAULT 0,
  output_tokens   INT NOT NULL DEFAULT 0,
  cost            DECIMAL(14,8) NOT NULL DEFAULT 0,
  multiplier      DECIMAL(8,4) NOT NULL DEFAULT 1,
  status_code     INT NULL,
  success         TINYINT NOT NULL DEFAULT 1,
  latency_ms      INT NULL,
  ttft_ms         INT NULL,          -- 首 token 耗时（流式）
  ip_address      VARCHAR(45) NULL,
  error_message   VARCHAR(500) NULL,
  created_at      DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  INDEX idx_user_time (user_id, created_at),
  INDEX idx_trace (trace_id),
  INDEX idx_model_time (model_name, created_at)
) COMMENT '建议按月分表';

CREATE TABLE balance_ledger (
  id          BIGINT PRIMARY KEY AUTO_INCREMENT,
  user_id     BIGINT NOT NULL,
  api_key_id  BIGINT NULL,
  change_amt  DECIMAL(14,6) NOT NULL,  -- 正负
  balance_after DECIMAL(14,6) NOT NULL,
  reason      VARCHAR(32) NOT NULL,    -- FREEZE/SETTLE/UNFREEZE/RECHARGE/REDEEM/ADJUST
  ref_id      VARCHAR(64) NULL,       -- trace_id / order_no
  created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  INDEX idx_ledger_user (user_id, created_at)
);

CREATE TABLE redeem_code (
  id          BIGINT PRIMARY KEY AUTO_INCREMENT,
  code        VARCHAR(32) NOT NULL UNIQUE,
  amount      DECIMAL(14,6) NOT NULL,
  total_quota INT NOT NULL DEFAULT 1,
  used_quota  INT NOT NULL DEFAULT 0,
  expire_at   DATETIME NULL,
  status      TINYINT NOT NULL DEFAULT 1,
  created_by  BIGINT NULL,
  created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE recharge_order (
  id             BIGINT PRIMARY KEY AUTO_INCREMENT,
  order_no       VARCHAR(64) NOT NULL UNIQUE,
  user_id        BIGINT NOT NULL,
  amount         DECIMAL(14,6) NOT NULL,
  payment_method VARCHAR(20) NOT NULL, -- redeem / alipay / wechat
  status         VARCHAR(20) NOT NULL DEFAULT 'PENDING',
  transaction_id VARCHAR(100) NULL,
  paid_at        DATETIME NULL,
  created_at     DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE audit_log (
  id          BIGINT PRIMARY KEY AUTO_INCREMENT,
  actor_type  VARCHAR(16) NOT NULL, -- ADMIN/USER/SYSTEM
  actor_id    BIGINT NULL,
  action      VARCHAR(64) NOT NULL,
  target_type VARCHAR(32) NULL,
  target_id   VARCHAR(64) NULL,
  detail      JSON NULL,
  ip          VARCHAR(45) NULL,
  created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  INDEX idx_audit_time (created_at)
);
```

### 6.5 缓存说明

- **不做** `cache_entry.embedding VECTOR` 默认表。  
- P2 精确缓存：Redis `cache:chat:{sha256(model+messages)}`。  
- P3 语义缓存：独立评估 PostgreSQL+pgvector 或专用向量库。

---

## 七、API 契约

### 7.1 网关（OpenAI 兼容，对外开放）

| 方法 | 路径 | 说明 | 认证 |
|---|---|---|---|
| POST | `/v1/chat/completions` | 对话（`stream=true` 时 SSE） | Bearer |
| GET | `/v1/models` | 当前 Key 可见模型 | Bearer |

认证头（P0）：

```http
Authorization: Bearer sk-xxxxxxxx
```

兼容（可选）：

```http
X-API-Key: sk-xxxxxxxx
```

错误响应：尽量兼容 OpenAI 风格，同时带内部 `trace_id`：

```json
{
  "error": {
    "message": "Insufficient balance",
    "type": "aigate_error",
    "param": null,
    "code": "insufficient_quota"
  },
  "trace_id": "..."
}
```

### 7.2 用户 API（JWT）

| 方法 | 路径 | 功能 | 优先级 |
|---|---|---|---|
| POST | `/api/user/auth/login` | 登录 | P0 |
| POST | `/api/user/auth/register` | 注册（可关） | P1 |
| GET | `/api/user/models` | 模型广场数据 | P0 |
| GET/POST/PATCH/DELETE | `/api/user/keys` | 令牌 CRUD | P0 |
| GET | `/api/user/logs` | 使用日志 | P0 |
| GET | `/api/user/wallet` | 钱包 | P0 |
| POST | `/api/user/recharge/redeem` | 兑换码 | P1 |
| POST | `/api/user/chat` `/stream` | 控制台对话（走同一网关内核） | P1 |
| GET/PUT | `/api/user/settings` | 个人设置 | P1 |
| POST | `/api/user/recharge` | 在线支付下单 | P2 |

### 7.3 管理 API（Admin JWT）

| 方法 | 路径 | 功能 | 优先级 |
|---|---|---|---|
| POST | `/api/admin/auth/login` | 登录 | P0 |
| CRUD | `/api/admin/channels` | 渠道 | P0 |
| CRUD | `/api/admin/models` | 模型与渠道映射 | P0 |
| CRUD | `/api/admin/users` | 用户 | P0 |
| CRUD | `/api/admin/groups` | 分组 | P0 |
| GET/PATCH | `/api/admin/keys` | 令牌监管 | P0 |
| GET | `/api/admin/report/overview` | 概览 | P1 |
| CRUD | `/api/admin/redeem/codes` | 兑换码 | P1 |
| GET/PUT | `/api/admin/settings` | 系统设置 | P1 |
| GET | `/api/admin/audit-logs` | 审计 | P1 |

---

## 八、核心后端设计（给 AI 编程助手）

### 8.1 网关处理管道（顺序固定）

1. `TraceFilter`：生成 `traceId`  
2. `AuthFilter`：解析 Bearer → 查 Key（Redis 缓存）→ 校验状态/IP/过期  
3. `RateLimitFilter`：RPM/TPM  
4. `ModelResolveService`：解析对外 model + 权限  
5. `BillingService.freeze`：冻结额度  
6. `ChannelRouter.select`：按权重选健康渠道  
7. `ProviderAdapter.invoke/stream`  
8. `BillingService.settle`：实结  
9. `LogService`：写日志（异步可，但结算必须先成功）

### 8.2 适配器接口

```java
public interface ProviderAdapter {
    String provider(); // openai_compatible / deepseek / tongyi / doubao ...

    ChatCompletionResult chat(ChatCompletionRequest req, Channel channel, String upstreamModel);

    Flux<String> chatStream(ChatCompletionRequest req, Channel channel, String upstreamModel);

    boolean healthCheck(Channel channel);
}
```

要求：

- 优先实现 **OpenAI Compatible Adapter**（覆盖 DeepSeek 等兼容上游），再做通义/豆包差异适配。  
- 上游超时、4xx/5xx、空流必须可归类，供路由切换。

### 8.3 渠道路由

```text
candidates = model_channel(enabled) ∩ channel(status=UP) 按 weight
try:
  invoke(candidate)
on retryable failure:
  mark soft-fail → next candidate
all failed:
  settle/unfreeze → 返回 502/503
```

### 8.4 前端信息架构（参考对标，不抄皮肤）

**用户台侧栏**：Dashboard · 令牌 · 日志 · 钱包 · 模型广场 · Playground(P1) · 设置  

**管理台侧栏**：概览 · 渠道 · 模型 · 用户 · 分组 · 令牌 · 兑换码 · 报表 · 设置 · 审计  

落地页一句话：**「替换 Base URL，企业内部统一调用大模型」**。

---

## 九、开发路线图（可承诺）

| 阶段 | 周期 | 目标 | 交付物 | 退出标准 |
|---|---|---|---|---|
| **Phase 0 立项基建** | 1–2 周 | 工程骨架、表结构、Compose、规范 | 空工程可启动；Flyway；CI 编译 | 本地一键起 MySQL/Redis/App |
| **Phase 1 网关 MVP** | 3–6 周 | 鉴权/限流/渠道/计费/日志/Chat+Models | 可被 SDK 调用的网关 | OpenAI Python SDK 流式跑通；无超扣 |
| **Phase 2 双端控制台** | 7–10 周 | 管理端渠道模型用户；用户端令牌日志钱包模型广场 | 两套 UI | 管理员不开 SQL 可完成配置 |
| **Phase 3 交付增强** | 11–14 周 | Playground、兑换码、健康检查、审计、报表、文档 | 私有化交付包 | 给试点客户安装成功 |
| **Phase 4 商业增强** | 按商机 | 在线支付、SSO、协议扩展、精确缓存、内容安全、Helm | 商业版 | 有合同再排 |

**总原则**：第 10–12 周必须具备「可卖试点」能力；不要把 Phase 4 写进首版合同。

人员建议（最低）：后端 2 + 前端 1 + 兼产品/测试 1。更少则拉长周期，不砍质量红线。

---

## 十、给 AI 编程助手的执行指令

### 10.1 生成顺序

1. 父 POM + 单体模块 + `docker-compose.yml`（MySQL/Redis/App）  
2. Flyway 脚本（本文表）+ 初始 admin 账号  
3. common：错误码、AES、JWT、traceId  
4. domain：Channel/Model/ApiKey/Billing  
5. gateway：`/v1/chat/completions` + `/v1/models`  
6. admin API + admin-ui（渠道/模型/用户/分组）  
7. user API + user-ui（令牌/日志/钱包/模型广场）  
8. Playground、兑换码、审计、报表  

### 10.2 编码约束

- 余额变更必须单事务或 Redis Lua 原子脚本，有单测覆盖并发。  
- 所有对外错误含 `trace_id`。  
- 配置敏感项仅环境变量：`DB_*` `REDIS_*` `JWT_SECRET` `AES_KEY`。  
- 单测：计费冻结/实结、路由切换、鉴权拒绝；适配器用 MockWebServer。  

### 10.3 禁止 AI「自作主张」

- 不得擅自加分销、语义缓存、R2DBC、多服务拆分。  
- 不得生成上游厂商 Key 示例当真密钥。  
- 不得把 V3 的明文 `api_key` 表结构写回。  

---

## 十一、开源与合规

| 策略 | 说明 |
|---|---|
| 首选 | **自研核心网关**（本文规格），控制台可参考开源 UI 思路但不拷代码 |
| 可参考 | One API（MIT）思路；**New API（AGPL）商用私有化需极度谨慎** |
| 交付物 | 《开源依赖 License 清单》必须随版本发布 |
| 行动 | 无法确认协议可商用 → **禁止二开该仓库** |

---

## 十二、交付物清单（首期合同口径）

| 序号 | 交付物 | 首期是否必须 |
|---|---|---|
| 1 | 后端源码（单体多模块） | ✅ |
| 2 | 管理后台前端 | ✅ |
| 3 | 用户控制台前端 | ✅ |
| 4 | Flyway/SQL 初始化 | ✅ |
| 5 | Docker Compose + 部署手册 | ✅ |
| 6 | OpenAPI/Swagger + 客户接入文档（SDK/Cursor 示例） | ✅ |
| 7 | 压测报告与超扣防护说明 | ✅ |
| 8 | License 合规说明 | ✅ |
| 9 | Helm Charts | ❌ 二期 |
| 10 | 在线支付 | ❌ 二期 |
| 11 | 语义缓存 | ❌ 不做承诺 |

---

## 十三、风险清单（项目经理盯）

| 风险 | 等级 | 缓解 |
|---|---|---|
| 范围膨胀对齐 TokenHub 全协议 | 高 | 协议扩展必须客户签字排期 |
| 计费超扣 | 高 | 冻结实结 + 并发单测 + 对账任务 |
| 上游变更/限流 | 高 | 多渠道 + 健康检查 + 明确 SLA 免责 |
| 二开 License | 高 | 自研或只用不感染协议组件 |
| WebFlux 团队不熟 | 中 | 默认 MVC |
| 把聊天 UI 当产品主体 | 中 | Playground 是调试器，不是核心 |

---

## 十四、立项决策（评审签字栏）

- [ ] 产品定义同意：私有化 AI API 网关，而非 SaaS 倒卖站  
- [ ] MVP 范围同意：Chat + Models + Channel + 令牌计费日志 + 双端基础台  
- [ ] 技术栈同意：Java 21 + Spring MVC + MyBatis/JDBC + MySQL + Redis + Compose  
- [ ] 计费方案同意：预扣冻结 + 实结  
- [ ] 不做承诺：语义缓存降本、首期分销、首期 Helm、首期全协议对齐  
- [ ] License 策略同意：优先自研  

**评审人 / 日期：** ________________  

---

## 附录 A · V3 废止条款摘要

1. 「语义缓存降本 80% 为核心护城河」——废止。  
2. 「WebFlux + R2DBC 强制、严禁 JPA/JDBC」——废止。  
3. 「三服务拆分作为默认」——废止为后期选项。  
4. 「仅 X-API-Key」——废止；Bearer 为 P0。  
5. 「MQ 异步扣费」——废止为唯一扣费路径。  
6. 「MySQL VECTOR 默认语义缓存表」——废止。  
7. 对标表中不实对比——废止，以本文 1.4 为准。  

## 附录 B · 对标站观察摘要（智信 TokenHub）

- 用户主路径：控制台 → 令牌 → 模型广场 → Playground → 日志 → 钱包。  
- 令牌字段：名称、过期、批量创建、额度、模型限制、IP 白名单、跨组重试。  
- 定价：分组倍率 + 按量/按次 + 厂商 Prompt Cache 价格展示。  
- 文档强调客户端接入（Codex/Claude Code 等）。  
- 本观察账号为普通用户，管理端渠道细节需管理员账号复核。  

---

**文档结束（V4.0）**

> 执行提示：本文件即为施工依据。AI 助手生成代码时以 **P0 → P1** 为序；凡与附录 A 废止条款冲突的旧描述，一律以 V4 为准。
