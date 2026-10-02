# starter-rocketmq 示例

一个通过协议无关的 `messaging.Driver` 完成的最小发布/订阅往返：应用以
`hello-group` 消费组订阅 `hello` 主题，发布一条带 key 和自定义 header 的消息，
并断言消费端收到相同的消息体和 header。往返成功后自退出（`SIGTERM`）；
任何失败都会以非零码退出。

## 运行

```bash
docker compose up -d   # 名字服务 :9876 + broker :10911（自动建主题）
go run .
./check.sh             # 起环境、跑示例、收环境 —— 冒烟测试
```

手动验证模式会让服务保持运行：

```bash
go run . -manual
```
