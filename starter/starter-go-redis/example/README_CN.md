# starter-go-redis Example

演示 starter-go-redis 的 go-redis Redis 客户端。

## 功能验证

- **String SET/GET**：写入和读取字符串键值
- **Hash 操作**：HSET/HGET 操作
- **List 操作**：LPUSH/LRANGE 操作
- **多 Driver 支持**：可切换 go-redis/redigo driver

> 需要 Redis 服务运行。`check.sh` 通过 docker compose 启动 Redis。

## 手动验证

```bash
cd starter-go-redis/example
go run . -manual
```

预期输出：
```
SET foo bar: OK
GET foo: bar
```

需要先启动 Redis：
```bash
# 启动 Redis
docker compose up -d

# 运行示例（manual 模式，保持运行）
go run . -manual
```

服务保持运行，可以用对应 CLI 工具验证。`Ctrl+C` 退出服务。

## 冒烟测试

```bash
./check.sh
```

`check.sh` 通过 docker compose 启动 Redis，运行示例并验证操作，退出码 0 表示通过。