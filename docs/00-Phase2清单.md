# Phase 2 完成清单

> **备案总文档（优先）：** [`12-里程碑M2-基线需求与测试用例.md`](./12-里程碑M2-基线需求与测试用例.md)  
> 含应用场景、角色、边界、REQ、TC；后续变更必须回溯该文档。

## Gate A · Mock（已完成）

- [x] JWT 登录 `/api/auth/login`
- [x] 用户 API：me / wallet / keys / logs / models / chat
- [x] 管理 API：overview / channels / models（JWT）
- [x] 内嵌控制台 UI；单二进制托管静态资源
- [x] 修复 `/` 重定向环；冒烟通过

## Gate B · 商业化（本轮推进）

- [x] 模型广场、渠道厂商下拉、令牌高级字段、兑换码
- [x] 双模式：onprem 组织管理 / saas 套餐强校验（docs/11、docs/12）

### 仍留给 Phase 3

- [ ] 在线支付、渠道 Key AES、完整报表、真实 Key 演示包

## 验收

按 **docs/12** 第 8.9 节冒烟清单执行。
