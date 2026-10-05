# starter-config-file configtree 示例

用 `configtree` provider 演示"标量 key 文件目录"的配置热更新。

## 功能验证

- **树导入**：每个扁平 key 文件成为一个属性（`db.user`、`db.password`、`server.port`）
- **K8s 风格挂载**：复刻 Secret/ConfigMap 卷的 `..data` 原子软链交换
- **热更新**：交换后绑定的 `gs.Dync[T]` 字段自动刷新，无需重启

## 手动验证

```bash
cd starter-config-file/example-configtree
go run . -manual
```

服务保持运行，`Ctrl+C` 退出。另开一个终端改写某个 key 文件，即可看到变化打印：

```bash
echo manual-1 > mount/db.user
# 打印：db.user: "alice" -> "manual-1"
```

不带 `-manual` 时，示例自己改写挂载（kubelet 式的 `..data` 原子交换）、等待绑定
字段热更新，打印 `hot-reload observed: db.user= bob-<hhmmss>` 后退出：

```bash
go run .
```

## 冒烟测试

```bash
./check.sh
```

`check.sh` 铺设 Secret 风格挂载、改写它并断言绑定字段热更新，退出码 0 表示通过。
