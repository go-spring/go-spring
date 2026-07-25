# starter-gorm-mysql Example

演示 starter-gorm-mysql 的 MySQL 数据库连接。

## 功能验证

- **数据库连接**：通过 GORM 连接 MySQL
- **版本查询**：执行 `SELECT VERSION()` 验证连通性

> 需要 MySQL 服务运行。`check.sh` 通过 docker compose 启动 MySQL。

## 手动验证

```bash
cd starter-gorm-mysql/example
go run .
```

预期输出：
```
MySQL version: ...
```

需要先启动 MySQL：
```bash
# 启动 MySQL
docker compose up -d

# 运行示例
go run .
```

## 冒烟测试

```bash
./check.sh
```

`check.sh` 通过 docker compose 启动 MySQL，运行示例并验证连接，退出码 0 表示通过。