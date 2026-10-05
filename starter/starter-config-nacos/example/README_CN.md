# starter-config-nacos 示例

演示 starter-config-nacos 的 Nacos 配置管理。

## 功能验证

- **配置加载**：从 Nacos 读取 data-id 对应的配置
- **配置热更新**：通过 Nacos API 发布新配置，应用实时感知变化
- **Dync 动态绑定**：通过 `Dync[T]` 绑定配置，自动刷新

> 需要 Nacos 服务运行。`check.sh` 通过 docker compose 启动 Nacos。

## 手动验证

```bash
cd starter-config-nacos/example
go run . -manual
```

需要先启动 Nacos：
```bash
# 启动 Nacos
docker compose up -d

# 运行示例（manual 模式，保持运行）
go run . -manual
```

服务保持运行，`Ctrl+C` 退出服务。另开一个终端发布新值，即可看到变化打印：

```bash
curl -fsS -X POST 'http://127.0.0.1:8848/nacos/v1/cs/configs' \
  -d 'dataId=gs-config-demo&group=DEFAULT_GROUP&content=demo.message=manual-1'
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

`check.sh` 通过 docker compose 启动 Nacos，运行示例并验证配置刷新，退出码 0 表示通过。