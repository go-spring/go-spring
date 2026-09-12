# starter-config-consul Example

演示 starter-config-consul 的 Consul KV 配置管理。

## 功能验证

- **配置加载**：从 Consul KV 读取配置项
- **配置热更新**：通过 Consul API 修改 KV，应用实时感知变化
- **Dync 动态绑定**：通过 `Dync[T]` 绑定配置，自动刷新

> 需要 Consul 服务运行。`check.sh` 通过 docker compose 启动 Consul。

## 手动验证

```bash
cd starter-config-consul/example
go run . -manual
```

预期输出：
```
initial value
updated value
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

`check.sh` 通过 docker compose 启动 Consul，运行示例并验证配置刷新，退出码 0 表示通过。