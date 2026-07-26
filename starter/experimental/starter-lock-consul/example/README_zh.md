# starter-lock-consul Example

演示 starter-lock-consul 的 Consul Session 分布式锁。

## 功能验证

- **锁获取**：通过 Consul Session 获取分布式锁
- **锁竞争**：多个实例竞争同一把锁，只有一个获取成功
- **锁释放**：任务完成后释放锁

> 需要 Consul 服务运行。`check.sh` 通过 docker compose 启动 Consul。

## 手动验证

```bash
cd starter-lock-consul/example
go run . -manual
```

预期输出：
```
lock acquired
task completed
lock released
```

需要先启动 Consul：
```bash
# 启动 Consul
docker compose up -d

# 运行示例（manual 模式，保持运行）
go run . -manual
```

服务保持运行，可以用对应 CLI 工具验证。`Ctrl+C` 退出服务。

## 冒烟测试

```bash
./check.sh
```

`check.sh` 通过 docker compose 启动 Consul，运行示例并验证锁，退出码 0 表示通过。
