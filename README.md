# AIGate

可私有部署的 AI API 网关。对外提供 OpenAI 兼容接口，对内用网页控制台管理渠道、模型、令牌和用量。

服务端与网页控制台以 [MIT](LICENSE) 许可发布。

## 它做什么

- 把各家模型厂商、以及本机的 OpenAI 兼容服务，收成统一的 `/v1` 接口
- 用网页控制台配置渠道、发布模型、签发调用令牌、查看日志和余额
- 操练场支持多轮对话，会话保存在本机数据库
- 按 token 计费：调用前预扣，结束后按实际用量结算
- DeepSeek 渠道可以按请求开启联网搜索

## 文档

| 文档 | 内容 |
|---|---|
| [项目说明书](docs/项目说明书.md) | 功能、使用对象、仓库里有什么 |
| [部署与使用](docs/部署与使用.md) | 安装、配置、控制台、接入客户端、备份 |

## 最快跑起来

需要 Go 1.22 或更高版本。

```powershell
Copy-Item configs\config.example.yaml configs\config.yaml
go run .\cmd\aigate -c configs\config.yaml
```

浏览器打开 http://127.0.0.1:3000/ 。示例配置里的初始管理员是 `admin` / `admin123`，第一次登录后请马上修改。

业务程序使用的地址是 `http://<主机>:3000/v1`，密钥是控制台里创建的令牌，不是厂商的 API Key。

## 不要提交的文件

`configs/config.yaml`、`data/`、`logs/`、`dist/` 已在 `.gitignore` 中。渠道密钥和数据库只留在运行这套程序的机器上。

## 许可

本仓库的服务端和网页控制台使用 MIT 许可，见 [LICENSE](LICENSE)。  
从其他项目拷贝代码时，请保留对方许可要求的署名。不要把 AGPL 项目的源码混进本仓库。
