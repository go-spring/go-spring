# starter-config-etcd 示例

演示 starter-config-etcd 的 etcd KV 配置管理。

## 功能验证

- **配置加载**：从 etcd KV 读取配置项
- **配置热更新**：通过 etcd API 修改 KV，应用实时感知变化
- **Dync 动态绑定**：通过 `Dync[T]` 绑定配置，自动刷新

> 需要 etcd 服务运行。`check.sh` 通过 docker compose 启动 etcd。

## 手动验证

```bash
cd starter-config-etcd/example
go run . -manual
```

需要先启动 etcd：
```bash
# 启动 etcd
docker compose up -d

# 运行示例（manual 模式，保持运行）
go run . -manual
```

服务保持运行，`Ctrl+C` 退出服务。另开一个终端发布新值，即可看到变化打印：

```bash
docker exec starter-etcd-config etcdctl put gs-config-demo 'demo.message=manual-1'
# 打印：demo.message: "..." -> "manual-1"
```

不带 `-manual` 时，示例会自己发布一个新值、等待绑定字段热更新，打印
`hot-reload observed: hello-<hhmmss>` 后退出：

```bash
go run .
```

## 冒烟测试

```bash
./check.sh
```

`check.sh` 通过 docker compose 启动 etcd，运行示例并验证配置刷新，退出码 0 表示通过。