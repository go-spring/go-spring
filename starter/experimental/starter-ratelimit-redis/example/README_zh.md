# starter-ratelimit-redis 示例

两个 HTTP handler（`/a/`、`/b/`）模拟一个服务的两个副本：进程间不共享
任何状态，只共用同一个 Redis 令牌桶（burst 5，2/s 补充），注册在 driver
名 `redis` 下。

## 运行方式

```bash
docker compose up -d     # 或把 spring.go-redis.cache.addr 指向任意 redis
go run .
```

## 冒烟测试

```bash
./check.sh
```

断言：10 个交替请求恰好 5×200 + 5×429（"副本"共享一个预算）；约 2 秒后
补充又放出新的令牌。手动模式：`go run . -manual`，然后
`curl http://127.0.0.1:9090/a/`、`curl http://127.0.0.1:9090/b/` 观察两者
共享预算。
