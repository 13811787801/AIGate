# Phase 1 验收与接入本地模型

## 已实现能力

| 能力 | 状态 |
|---|---|
| SQLite 表结构（用户/令牌/渠道/模型/日志/账本） | ✅ |
| Bearer / X-API-Key 鉴权 | ✅ |
| `/v1/models` | ✅ |
| `/v1/chat/completions` 非流式 + SSE 流式 | ✅ |
| mock 渠道（无上游可测全链路） | ✅ |
| openai_compatible 渠道（云/Ollama/vLLM） | ✅ |
| 渠道路由 + 多渠道故障尝试 | ✅ |
| 预扣冻结 → 实结 | ✅ |
| RPM 限流（内存滑动窗口） | ✅ |
| 管理引导 API（加渠道/模型/查钱包） | ✅ 简易 |

未做（Phase 2+）：控制台 UI、JWT 登录台、Redis 限流、Key 加密存储强化。

## 快速验收

```powershell
cd E:\Projects\AIGate
# 若要重新种子（会清库）：
Remove-Item data\aigate.db* -ErrorAction SilentlyContinue
.\scripts\build-windows.ps1
.\scripts\dev-run.ps1
```

启动日志里会打印一次：

```text
DEMO API KEY ... sk-........
```

另开终端：

```powershell
$key = "sk-把日志里的贴这里"
$h = @{ Authorization = "Bearer $key" }

Invoke-RestMethod http://127.0.0.1:3000/healthz
Invoke-RestMethod http://127.0.0.1:3000/v1/models -Headers $h

$body = '{"model":"mock-chat","messages":[{"role":"user","content":"你好"}],"stream":false}'
Invoke-RestMethod http://127.0.0.1:3000/v1/chat/completions -Headers $h -Method Post -ContentType "application/json" -Body $body

# 查余额（默认 admin123）
Invoke-RestMethod http://127.0.0.1:3000/api/admin/wallet -Headers @{ "X-Admin-Token"="admin123" }
```

期望：mock 返回 `mock reply: ...`，余额略减。

## 接入本地 Ollama（示例）

假设 Ollama 在 `http://127.0.0.1:11434`，模型名 `qwen2.5:7b`：

```powershell
$admin = @{ "X-Admin-Token"="admin123"; "Content-Type"="application/json" }

# 1) 建渠道
$ch = Invoke-RestMethod http://127.0.0.1:3000/api/admin/channels -Headers $admin -Method Post -Body (@{
  name = "ollama-local"
  provider = "openai_compatible"
  base_url = "http://127.0.0.1:11434/v1"
  api_key = "ollama"
  weight = 100
  priority = 20
} | ConvertTo-Json)

# 2) 建对外模型并绑定渠道
Invoke-RestMethod http://127.0.0.1:3000/api/admin/models -Headers $admin -Method Post -Body (@{
  model_name = "local-qwen"
  display_name = "本地 Qwen"
  billing_mode = "TOKEN"
  input_price_per_1k = 0
  output_price_per_1k = 0
  max_tokens = 4096
  channel_id = $ch.id
  upstream_model = "qwen2.5:7b"
  group_id = 1
} | ConvertTo-Json)

# 3) 调用
$body = '{"model":"local-qwen","messages":[{"role":"user","content":"用一句话介绍你自己"}],"stream":false}'
Invoke-RestMethod http://127.0.0.1:3000/v1/chat/completions -Headers $h -Method Post -ContentType "application/json" -Body $body
```

云厂商同理：`base_url` 填官方兼容地址，`api_key` 填上游 Key，`upstream_model` 填上游模型名。

## 排障

| 现象 | 处理 |
|---|---|
| 401 invalid api key | Key 错了或库被清过；看启动日志重新拿 demo key |
| 402 insufficient | 钱包余额不足；查 `/api/admin/wallet` |
| 502 no channel / upstream | 渠道没绑模型，或 Ollama 没开 |
| 找不到 demo key | 只有**空库首次启动**打印；删 `data\aigate.db*` 再启 |

## 管理 API（Phase1 临时）

鉴权头：`X-Admin-Token: <configs 里 admin_password>`

- `POST /api/admin/channels`
- `POST /api/admin/models`
- `GET /api/admin/wallet`
