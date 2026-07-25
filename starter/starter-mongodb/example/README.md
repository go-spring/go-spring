# starter-mongodb Example

演示 starter-mongodb 的 MongoDB 客户端。

## 功能验证

- **健康检查**：验证 MongoDB 集群连通性
- **CRUD 操作**：插入文档、查询文档
- **连接池**：连接池配置和管理

> 需要 MongoDB 服务运行。`check.sh` 通过 docker compose 启动 MongoDB。

## 手动验证

```bash
cd starter-mongodb/example
go run .
```

预期输出：
```
health check: OK
document inserted
document found
```

需要先启动 MongoDB：
```bash
# 启动 MongoDB
docker compose up -d

# 运行示例
go run .
```

## 冒烟测试

```bash
./check.sh
```

`check.sh` 通过 docker compose 启动 MongoDB，运行示例并验证操作，退出码 0 表示通过。