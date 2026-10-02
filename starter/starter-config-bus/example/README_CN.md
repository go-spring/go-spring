# starter-config-bus Example

演示 starter-config-bus 的配置总线动态刷新。

## 功能验证

- **配置热更新**：通过配置总线发布新值，应用实时感知变化
- **Dync 动态绑定**：通过 `Dync[T]` 绑定配置项，值自动刷新

> 需要 Redis 服务运行（配置总线依赖 Redis 作为消息通道）。`check.sh` 通过 docker compose 启动 Redis。

## 手动验证

```bash
cd starter-config-bus/example
go run . -manual
```

预期输出：
```
initial value
updated value
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

`check.sh` 通过 docker compose 启动 Redis，运行示例并验证配置刷新，退出码 0 表示通过。