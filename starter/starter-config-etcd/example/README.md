# starter-config-etcd Example

演示 starter-config-etcd 的 etcd KV 配置管理。

## 功能验证

- **配置加载**：从 etcd KV 读取配置项
- **配置热更新**：通过 etcd API 修改 KV，应用实时感知变化
- **Dync 动态绑定**：通过 `Dync[T]` 绑定配置，自动刷新

> 需要 etcd 服务运行。`check.sh` 通过 docker compose 启动 etcd。

## 手动验证

```bash
cd starter-config-etcd/example
go run .
```

预期输出：
```
initial value
updated value
```

需要先启动 etcd：
```bash
# 启动 etcd
docker compose up -d

# 运行示例
go run .
```

## 冒烟测试

```bash
./check.sh
```

`check.sh` 通过 docker compose 启动 etcd，运行示例并验证配置刷新，退出码 0 表示通过。