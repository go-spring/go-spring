# starter-hertz Example

演示 starter-hertz 的 HTTP 服务路由、中间件与健康检查。

## 功能验证

- **路由**：`/echo/:name` 路径参数 + JSON 响应
- **路由**：`/greet` 查询参数 + JSON 响应
- **中间件**：自定义 `X-App` 响应头注入
- **内置中间件**：`X-Request-Id`、`X-Content-Type-Options` 安全头
- **健康检查**：`/healthz` 端点

## 手动验证

```bash
cd starter-hertz/example
go run .
```

预期输出：
```
Response from server: {"message":"Hello, hertz"}
Response from server: {"message":"Hi, world"}
Health from server: ok
```

也可以手动 curl 验证：
```bash
# 终端1：启动服务
go run .

# 终端2：测试各端点
curl http://localhost:8003/echo/hertz
# -> {"message":"Hello, hertz"}

curl 'http://localhost:8003/greet?name=world'
# -> {"message":"Hi, world"}

curl -v http://localhost:8003/echo/hertz 2>&1 | grep -i x-app
# -> X-App: go-spring

curl http://localhost:8003/healthz
# -> ok
```

## 冒烟测试

```bash
./check.sh
```

`check.sh` 运行示例并等待其自测完成，退出码 0 表示通过。