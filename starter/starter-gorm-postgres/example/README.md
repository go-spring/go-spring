# starter-gorm-postgres Example

演示 starter-gorm-postgres 的 PostgreSQL 数据库连接。

## 功能验证

- **数据库连接**：通过 GORM 连接 PostgreSQL
- **版本查询**：执行 `SELECT version()` 验证连通性

> 需要 PostgreSQL 服务运行。`check.sh` 通过 docker compose 启动 PostgreSQL。

## 手动验证

```bash
cd starter-gorm-postgres/example
go run .
```

预期输出：
```
PostgreSQL version: ...
```

需要先启动 PostgreSQL：
```bash
# 启动 PostgreSQL
docker compose up -d

# 运行示例
go run .
```

## 冒烟测试

```bash
./check.sh
```

`check.sh` 通过 docker compose 启动 PostgreSQL，运行示例并验证连接，退出码 0 表示通过。