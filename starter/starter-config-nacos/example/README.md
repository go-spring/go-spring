# starter-config-nacos Example

演示 starter-config-nacos 的 Nacos 配置管理。

## 功能验证

- **配置加载**：从 Nacos 读取 data-id 对应的配置
- **配置热更新**：通过 Nacos API 发布新配置，应用实时感知变化
- **Dync 动态绑定**：通过 `Dync[T]` 绑定配置，自动刷新

> 需要 Nacos 服务运行。`check.sh` 通过 docker compose 启动 Nacos。

## 手动验证

```bash
cd starter-config-nacos/example
go run .
```

预期输出：
```
initial value
updated value
```

需要先启动 Nacos：
```bash
# 启动 Nacos
docker compose up -d

# 运行示例
go run .
```

## 冒烟测试

```bash
./check.sh
```

`check.sh` 通过 docker compose 启动 Nacos，运行示例并验证配置刷新，退出码 0 表示通过。