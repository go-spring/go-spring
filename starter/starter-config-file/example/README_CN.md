# starter-config-file 示例

演示 starter-config-file 的本地文件配置热更新。

## 功能验证

- **配置加载**：从 `conf/app.properties` 读取初始值
- **文件监听**：通过 `Dync[T]` 监听文件变化，自动热更新
- **动态值验证**：修改文件后配置值实时变化

## 手动验证

```bash
cd starter-config-file/example
go run . -manual
```

服务保持运行，`Ctrl+C` 退出。另开一个终端改写挂载文件，即可看到变化打印：

```bash
echo 'demo.message=manual-1' > mount/application.properties
# 打印：demo.message: "..." -> "manual-1"
```

不带 `-manual` 时，示例自己改写挂载（kubelet 式的 `..data` 原子交换）、等待绑定
字段热更新，打印 `hot-reload observed: updated-<hhmmss>` 后退出：

```bash
go run .
```

## 冒烟测试

```bash
./check.sh
```

`check.sh` 运行示例并等待其自测完成，退出码 0 表示通过。