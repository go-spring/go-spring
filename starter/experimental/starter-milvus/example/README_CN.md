# starter-milvus 示例

一次面向 Milvus standalone 的往返：建集合（dim 8）、插入向量、加载、检索、
断言 top-1 命中。成功后自退出。

## 可观测性

starter 只**声明**每个 RPC 是什么（`observe.go`：span 名、`db.client` 指标前缀、
`db.system`/`db.operation` 标签、完整方法路径作为 span/日志 detail）；**发射**由 resilience
层完成——executor 内唯一能看到整次调用（含重试）的位置。因此每次调用都产出调用级
`db.client.operation.duration` 直方图、尝试级 `db.client.attempt.duration` 直方图，以及
打上 `_app_milvus_access` 标签的访问日志。治理关闭时只停保护、不停观测；指标还需 `starter-otel`
安装 provider。

## 运行

```bash
docker compose up -d   # etcd + minio + milvus standalone
go run .
./check.sh             # 起环境、跑示例、收环境 —— 冒烟测试
```
