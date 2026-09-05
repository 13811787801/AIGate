# Phase 2 控制台说明

## 你现在打开什么？

浏览器 / Cursor 预览请打开：

**http://127.0.0.1:3000/**

这是 **AIGate 控制台网页**（登录后有侧边栏），不再只是 `/healthz` 的 JSON。

| 地址 | 是什么 |
|---|---|
| `/` | 控制台 UI（要登录） |
| `/healthz` | 探活 JSON（给运维/脚本） |
| `/v1/...` | OpenAI 兼容 API（给 SDK/Cursor） |

## 默认登录

- 用户名：`admin`（或 `configs/config.yaml` 里的 `admin_username`）
- 密码：`admin123`（`admin_password`）

## 页面结构

**用户侧**

- 总览：余额与接入说明  
- 对话调试：选模型发消息（先可用 `mock-chat`）  
- 令牌管理：创建/删除 `sk-` 密钥  
- 使用日志 / 钱包  

**管理侧（admin）**

- 渠道配置：接入 DeepSeek / OpenAI 等 `openai_compatible`  
- 模型配置：对外模型名绑定渠道与单价  

## 启动

```powershell
cd E:\Projects\AIGate
.\scripts\start-background.ps1
# 然后浏览器打开 http://127.0.0.1:3000/
```

## 推荐配置顺序（接真实大模型）

1. 登录控制台  
2. **渠道配置** → 填 DeepSeek/OpenAI 的 Base URL + 上游 Key  
3. **模型配置** → 新建对外模型并绑定该渠道  
4. **令牌管理** → 创建业务用 Key  
5. 业务侧 Base URL 指向 `http://服务器:3000/v1`  

Ollama 仍可作为备选渠道，同一套「渠道 + 模型」流程即可。
