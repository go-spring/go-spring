# starter-go-zero/rest Example

演示 starter-go-zero/rest 的 HTTP 服务路由。

## 功能验证

- **路由**：`/greet` 查询参数 + JSON 响应

## 手动验证

终端 1，启动服务并保持运行：
```bash
cd starter-go-zero/rest/example
go run . -manual
```

终端 2，执行验证命令：
```bash
curl 'http://127.0.0.1:8888/greet?name=world'
# -> {"message":"Hi, world"}
```

验证完成后 `Ctrl+C` 退出服务。

## 冒烟测试

```bash
./check.sh
```

`check.sh` 运行示例并等待其自测完成，退出码 0 表示通过。