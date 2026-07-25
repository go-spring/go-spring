# starter-gorm-sqlserver Example

演示 starter-gorm-sqlserver 的 SQL Server 数据库连接。

## 功能验证

- **数据库连接**：通过 GORM 连接 SQL Server
- **版本查询**：执行 `SELECT @@VERSION` 验证连通性

> 需要 SQL Server 服务运行。`check.sh` 通过 docker compose 启动 SQL Server。

## 手动验证

```bash
cd starter-gorm-sqlserver/example
go run .
```

预期输出：
```
SQL Server version: ...
```

需要先启动 SQL Server：
```bash
# 启动 SQL Server
docker compose up -d

# 运行示例
go run .
```

## 冒烟测试

```bash
./check.sh
```

`check.sh` 通过 docker compose 启动 SQL Server，运行示例并验证连接，退出码 0 表示通过。