# starter-lock-redis Example

演示 starter-lock-redis 的 Redis 分布式锁。

## 功能验证

- **TryAcquire**：对空闲 key 获取锁成功
- **锁互斥**：已持有的锁不能被其他实例获取
- **锁释放**：任务完成后释放锁

> 需要 Redis 服务运行。`check.sh` 通过 docker compose 启动 Redis。

## 手动验证

```bash
cd starter-lock-redis/example
go run .
```

预期输出：
```
lock acquired on free key
lock rejected on held key
lock released
```

需要先启动 Redis：
```bash
# 启动 Redis
docker compose up -d

# 运行示例
go run .
```

## 冒烟测试

```bash
./check.sh
```

`check.sh` 通过 docker compose 启动 Redis，运行示例并验证锁，退出码 0 表示通过。