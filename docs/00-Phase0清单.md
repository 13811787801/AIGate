# Phase 0 完成清单

- [x] 仓库目录与模块骨架
- [x] 配置样例与健康检查 HTTP 服务
- [x] 文档迁入 docs/ + 部署手册骨架
- [x] One API 浅克隆到 reference（gitignore，脚本可重拉）
- [x] 便携 Go 1.22（`E:\Projects\.toolchain\go`）
- [x] `go build` 成功；`/healthz`、`/v1/models` 冒烟通过
- [x] One API 二开体检结论：**不整仓 Fork**
- [x] Git 初始化与首提交

## 下一步 Phase 1

1. SQLite 表结构（user/api_key/channel/model/group/request_log）
2. Bearer 鉴权 + `/v1/models` 真数据
3. OpenAI compatible 上游调用（含本地 Ollama）
4. 预扣计费最小闭环
