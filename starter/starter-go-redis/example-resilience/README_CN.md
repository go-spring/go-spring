# starter-go-redis 韧性(resilience)示例

演示 starter-go-redis 的**韧性保护**:开启 `resilience.enabled` 后,每条 Redis 命令都会通过所选驱动的 `ClientExecutor`,超过限流的突发会在到达 Redis 之前被以 `ErrRateLimited` 拒绝。

## 特性

- **后端无关配置**:同一个 `${resilience.*}` 键驱动所有客户端 starter(rate-limit、burst、error-threshold、max-concurrent……)。
- **热更新**:包装类通过 `gs.Dync` 字段注入 `Resilience`,刷新即可在线替换 executor。
- **驱动可插拔**:`driver=default`(内置,零依赖)或 `sentinel`。
- **可观测**:executor 被 `observe/resilience` 包裹,熔断与拒绝会产出 span + 计数器 + 直方图。

## 手动验证

需要一个本地 Redis(`docker compose up -d`):

```bash
cd starter/starter-go-redis/example-resilience
docker compose up -d
go run . -manual
```

## 冒烟测试

```bash
./check.sh
```

`check.sh` 通过 docker compose 拉起 Redis,触发一批 `Set` 并断言非空头部被放行、非空尾部被以 `ErrRateLimited` 拒绝;退出码 0 表示通过。缺少 docker 时优雅跳过。
