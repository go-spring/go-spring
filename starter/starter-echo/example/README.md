# starter-echo Example

演示 starter-echo 的 HTTP 服务路由、中间件与健康检查。

## 功能验证

- **路由**：`/echo/:name` 路径参数 + JSON 响应
- **路由组**：`/api/greet` 查询参数路由
- **中间件**：自定义 `X-App` 响应头注入
- **内置中间件**：`X-Request-Id`、`X-Content-Type-Options` 安全头
- **健康检查**：`/healthz` 端点

## 手动验证

终端 1，启动服务并保持运行：
```bash
cd starter-echo/example
go run . -manual
```

终端 2，执行验证命令：
```bash
curl http://localhost:8002/echo/echo
# -> {"message":"Hello, echo"}

curl 'http://localhost:8002/api/greet?name=world'
# -> {"message":"Hi, world"}

curl -v http://localhost:8002/echo/echo 2>&1 | grep -i x-app
# -> X-App: go-spring

curl -v http://localhost:8002/echo/echo 2>&1 | grep -i x-request-id
# -> X-Request-Id: <uuid>

curl http://localhost:8002/healthz
# -> ok
```

验证完成后 `Ctrl+C` 退出服务。

## 冒烟测试

```bash
./check.sh
```

`check.sh` 运行示例并等待其自测完成，退出码 0 表示通过。