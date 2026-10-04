# starter-config-apollo 示例

自包含：示例启动一个 mock Apollo config service（agollo 驱动的 meta、
configfiles、configs 与 notifications/v2 端点），导入 starter，断言整条远程
配置链路 —— 属性冷加载进 Dync 字段，向 mock 发布新值后热更新。无需 docker，
也无需真实 Apollo 栈。

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
./check.sh   # 冒烟测试
```
