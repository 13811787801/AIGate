# Phase 3 / 里程碑 M3 清单

| 项 | 内容 |
|---|---|
| 里程碑 | **M3 · 生产硬化与经营闭环** |
| 对照 | `docs/13` 条件清单 · `docs/14` · `docs/15` |
| 状态 | **第一刀完成 · 第二刀（MySQL + 订购 Mock）已落地** |

## P0（第一刀）

- [x] 渠道上游 Key：**AES-GCM 加密落库**（`enc:v1:`），兼容旧明文
- [x] 弱密钥治理：启动 WARNING；`server.mode=release` 可拒绝启动
- [x] Bootstrap：`auth.bootstrap_token` 独立，默认空=禁用
- [x] 操练场：无 Key 时服务端自动创建 `playground-auto`
- [x] 厂商下拉：非兼容风险提示
- [x] 单测：crypto / entitlement / billing
- [x] 脚本：`scripts/accept-saas.ps1`（PASS）

## P1（第二刀）

- [x] MySQL 驱动接线 + 方言 upsert / `` `user` `` 表名
- [x] 备份与恢复手册：`docs/15-数据库备份与恢复手册.md`
- [x] 在线订购 Mock：下单 → mock-pay → 开通套餐（仅 `deploy.mode=saas`）
- [x] onprem 组织 RBAC：角色 + 模块权限；运维子账户可管渠道/模型
- [ ] 审计日志只读页（可再下一刀）

## 验收

- SQLite 回归：`go test ./internal/...`；控制台登录正常
- MySQL：配置 `driver=mysql` + DSN 后可启动建表（见 `config.example.yaml`）
- SaaS 订购：侧栏「订购套餐」→ 下单并模拟支付 → `plan_id` 更新
