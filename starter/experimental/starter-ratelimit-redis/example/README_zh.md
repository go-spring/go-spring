# starter-ratelimit-redis 示例

两个 HTTP handler（`/a/`、`/b/`）模拟一个服务的两个副本：进程间不共享任何状态，两者都跑在
service label `ratelimit-redis:api` 下，预算（burst 5，2/s）花的是本 starter 贡献的同一个
Redis 计数器存储——整个进程每个 scope 一个预算，配置相同的每个副本也共享它。

预算本身是治理规则（`conf/govern.yaml`），不是 starter 配置：`spring.ratelimit.redis.client=cache`
只说明计数器通过哪个 Redis 客户端计数。

## 运行方式

```bash
docker compose up -d     # 或把 spring.go-redis.instances.cache.addr 指向任意 redis
go run .
```

## 冒烟测试

```bash
./check.sh
```

断言：10 个交替请求恰好 5×200 + 5×429（"副本"共享一个预算）；约 2 秒后补充又放出新的令牌。
手动模式：`go run . -manual`，然后 `curl http://127.0.0.1:9090/a/`、
`curl http://127.0.0.1:9090/b/` 观察两者共享预算。
