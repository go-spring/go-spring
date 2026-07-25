# starter-elasticsearch Example

演示 starter-elasticsearch 的 Elasticsearch 客户端。

## 功能验证

- **集群连通性**：通过 HealthCheck 验证 ES 集群连接
- **索引操作**：创建索引、写入文档、查询文档

> 需要 Elasticsearch 服务运行。`check.sh` 通过 docker compose 启动 Elasticsearch。

## 手动验证

```bash
cd starter-elasticsearch/example
go run .
```

预期输出：
```
cluster health: green
document indexed
document found
```

需要先启动 Elasticsearch：
```bash
# 启动 ES
docker compose up -d

# 运行示例
go run .
```

## 冒烟测试

```bash
./check.sh
```

`check.sh` 通过 docker compose 启动 Elasticsearch，运行示例并验证操作，退出码 0 表示通过。