# starter-config-apollo 示例

演示 starter-config-apollo 的 Apollo 配置管理。

## 功能验证

- **配置加载**：从 Apollo namespace 读取配置
- **配置热更新**：向 mock 发布新值，应用实时感知变化
- **Dync 动态绑定**：通过 `Dync[T]` 绑定配置，自动刷新

> 自包含：示例内置一个 mock Apollo 服务（agollo 驱动的 meta、configfiles、
> configs 与 notifications/v2 端点），无需 docker，也无需真实 Apollo 栈。

## 手动验证

```bash
cd starter-config-apollo/example
go run . -manual
```

mock Apollo 服务（监听 `127.0.0.1:18080`）随进程启动，应用保持运行。
`Ctrl+C` 退出。

另开一个终端发布新值，即可看到变化打印：

```bash
curl -fsS -X POST 'http://127.0.0.1:18080/publish?value=manual-1'
# 打印：demo.message: "..." -> "manual-1"
```

不带 `-manual` 时，示例断言冷加载值，自己发布一个新值、等待热更新，打印
`hot-reload observed: hello-<hhmmss>` 后退出。

## 冒烟测试

```bash
./check.sh
```

`check.sh` 运行示例（mock 由示例自身启动），冷加载取值、发布新值并验证配置刷新，退出码 0 表示通过。
