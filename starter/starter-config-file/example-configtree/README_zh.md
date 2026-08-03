# starter-config-file configtree 示例

用 `configtree` provider 演示"标量 key 文件目录"的配置热更新。

## 特性

- **树导入**：每个扁平 key 文件成为一个属性（`db.user`、`db.password`、`server.port`）
- **K8s 风格挂载**：复刻 Secret/ConfigMap 卷的 `..data` 原子软链交换
- **热更新**：交换后绑定的 `gs.Dync[T]` 字段自动刷新，无需重启

## 手动验证

```bash
cd starter-config-file/example-configtree
go run . -manual
```

程序会持续运行，按 Ctrl+C 退出。不加 `-manual` 时，`runTest()` 自动执行后退出。

## 冒烟测试

```bash
cd starter-config-file/example-configtree
bash check.sh
```

铺设 Secret 风格挂载，把 `gs.Dync[string]` 字段绑定到 `db.user` / `db.password` /
`server.port`，原子交换 `..data`，断言绑定字段热更新。失败时非零退出。

## configtree 与 file-watch 的选择

- `configtree:<dir>` —— 标量 key 文件目录（路径→key、内容→value）。典型场景：Kubernetes Secret / env 风格 ConfigMap 挂载。
- `file-watch:<file>` —— 单份完整配置文档。典型场景：承载 `application.yaml` 的 ConfigMap key。
