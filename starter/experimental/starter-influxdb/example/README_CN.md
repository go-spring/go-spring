# starter-influxdb 示例

一次面向真实服务（见 docker-compose.yml）的往返：应用自建所需 schema、
经 starter 写入、读回并断言结果。成功后自退出（`SIGTERM`）；任何失败都
以非零码退出。

## 运行

```bash
docker compose up -d
go run .
./check.sh             # 起环境、跑示例、收环境 —— 冒烟测试
```

手动验证模式会让服务保持运行：

```bash
go run . -manual
```
