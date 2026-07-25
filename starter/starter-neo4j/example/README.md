# starter-neo4j Example

演示 starter-neo4j 的 Neo4j 图数据库客户端。

## 功能验证

- **连接健康**：验证 Neo4j 数据库连通性
- **Cypher 查询**：执行 Cypher 查询语句
- **多 Driver 支持**：可切换不同 Neo4j driver

> 需要 Neo4j 服务运行。`check.sh` 通过 docker compose 启动 Neo4j。

## 手动验证

```bash
cd starter-neo4j/example
go run .
```

预期输出：
```
health check: OK
query executed
```

需要先启动 Neo4j：
```bash
# 启动 Neo4j
docker compose up -d

# 运行示例
go run .
```

浏览器打开 `http://127.0.0.1:7474` 查看 Neo4j Browser。

## 冒烟测试

```bash
./check.sh
```

`check.sh` 通过 docker compose 启动 Neo4j，运行示例并验证查询，退出码 0 表示通过。