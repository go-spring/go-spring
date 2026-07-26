# starter-lua-filter Example

演示 starter-lua-filter 的 Lua 脚本过滤器。

## 功能验证

- **Lua 过滤器**：通过 Lua 脚本拦截和修改 HTTP 请求/响应
- **请求拦截**：验证 X-App header 的 guard 逻辑
- **响应透传**：合法请求透传到业务 handler

## 手动验证

终端 1，启动服务并保持运行：
```bash
cd starter-lua-filter/example
go run . -manual
```

终端 2，执行验证命令：
```bash
curl -H 'X-App: go-spring' http://127.0.0.1:9090/hello
# -> 200 OK

curl http://127.0.0.1:9090/hello
# -> 403 Forbidden
```

验证完成后 `Ctrl+C` 退出服务。

## 冒烟测试

```bash
./check.sh
```

`check.sh` 运行示例并等待其自测完成，退出码 0 表示通过。
