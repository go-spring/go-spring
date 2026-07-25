# starter-gateway Example

演示 starter-gateway 的 API 网关代理与过滤器。

## 功能验证

- **反向代理**：将请求转发到后端 upstream 服务
- **请求过滤器**：`addRequestHeader` 过滤器注入 `X-From` 头
- **端到端验证**：启动内嵌 backend，验证代理 + 过滤全链路

## 手动验证

```bash
cd starter-gateway/example
go run .
```

预期输出（断言通过后程序自动退出）：
```
echo: /api/echo, from: go-spring-gateway
```

也可以手动 curl 验证：
```bash
# 终端1：启动服务
go run .

# 终端2
curl -i http://127.0.0.1:9440/api/echo
# -> HTTP/1.1 200 OK
# -> echo: /api/echo, from: go-spring-gateway
```

## 冒烟测试

```bash
./check.sh
```

`check.sh` 运行示例并等待其自测完成，退出码 0 表示通过。