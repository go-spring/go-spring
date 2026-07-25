# starter-redigo Example

演示 starter-redigo 的 Redigo Redis 客户端。

## 功能验证

- **String SET/GET**：写入和读取字符串键值
- **多 Driver 支持**：可切换 redigo/go-redis driver

> 需要 Redis 服务运行。`check.sh` 通过 docker compose 启动 Redis。

## 手动验证

```bash
cd starter-redigo/example
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

# 运行示例
go run . -manual
```

## 冒烟测试

```bash
./check.sh
```

`check.sh` 通过 docker compose 启动 Redis，运行示例并验证操作，退出码 0 表示通过。
