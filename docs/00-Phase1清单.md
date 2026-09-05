# Phase 1 完成清单

- [x] SQLite schema + store
- [x] Seed：admin / demo key / mock-chat
- [x] Auth Bearer + 模型权限 + IP 白名单字段
- [x] Billing 预扣/实结/解冻 + ledger
- [x] Channel：mock + openai_compatible
- [x] Router：权重选择 + 故障尝试下一渠道
- [x] `/v1/models` `/v1/chat/completions`（流式/非流式）
- [x] RPM 限流（内存）
- [x] 管理引导 API（渠道/模型/钱包）
- [x] 本机冒烟：health / models / mock chat / wallet

## 下一步 Phase 2

1. 用户/管理控制台 UI  
2. 令牌自助 CRUD  
3. 使用日志查询页  
4. 渠道 AES 加密存储  
5. 本地优先路由策略配置化  
