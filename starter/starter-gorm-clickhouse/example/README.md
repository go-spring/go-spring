# starter-gorm-clickhouse Example

演示 starter-gorm-clickhouse 的 ClickHouse 数据库连接。

## 功能验证

- **数据库连接**：通过 GORM 连接 ClickHouse
- **版本查询**：执行 `SELECT version()` 验证连通性

> 需要 ClickHouse 服务运行。`check.sh` 通过 docker compose 启动 ClickHouse。

## 手动验证

```bash
cd starter-gorm-clickhouse/example
go run . -manual
```

预期输出：
```
ClickHouse version: ...
```

需要先启动 ClickHouse：
```bash
# 启动 ClickHouse
docker compose up -d

# 运行示例（manual 模式，保持运行）
go run . -manual
```

服务保持运行，可以用对应 CLI 工具验证。`Ctrl+C` 退出服务。

## 冒烟测试

```bash
./check.sh
```

`check.sh` 通过 docker compose 启动 ClickHouse，运行示例并验证连接，退出码 0 表示通过。