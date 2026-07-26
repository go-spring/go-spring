# starter-session-redis Example

演示 starter-session-redis 的 Redis Session 管理。

## 功能验证

- **跨副本共享**：副本 A 写入 session，副本 B 读取（共享 Redis 存储）
- **Session 持久化**：Session 数据存储在 Redis，进程重启不丢失
- **Cookie 传递**：通过 Cookie 传递 session ID

> 需要 Redis 服务运行。`check.sh` 通过 docker compose 启动 Redis。

## 手动验证

```bash
cd starter-session-redis/example
go run . -manual
```

预期输出：
```
session set: ok
session get: ok
cross-replica sharing: OK
```

需要先启动 Redis：
```bash
# 启动 Redis
docker compose up -d

# 运行示例
go run . -manual
```

也可以手动 curl 验证：
```bash
# 终端1：启动服务
go run . -manual

# 终端2：A 写入 session
curl -c /tmp/cookie http://127.0.0.1:9090/a/set?user=alice
# -> ok

# 终端2：B 读取 session（共享 Redis）
curl -b /tmp/cookie http://127.0.0.1:9090/b/get
# -> alice
```

## 冒烟测试

```bash
./check.sh
```

`check.sh` 通过 docker compose 启动 Redis，运行示例并验证 session，退出码 0 表示通过。
