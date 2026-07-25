# starter-dubbo Example

演示 starter-dubbo 的 Dubbo RPC 服务注册与调用。

## 功能验证

- **RPC 调用**：客户端通过 Triple 协议调用 `Greet` 方法，验证请求/响应正确性
- **服务注册**：通过 `RegisterService` 将 `GreetProvider` 注册为 Dubbo 服务

## 手动验证

终端 1，启动服务并保持运行：
```bash
cd starter-dubbo/example
go run . -manual
```

终端 2，运行客户端验证脚本：
```bash
go run check_client.go
```

预期输出：
```
Response from server: Hello, Dubbo-Go!
OK: Dubbo RPC verified
```

验证完成后 `Ctrl+C` 退出服务。

## 冒烟测试

```bash
./check.sh
```

`check.sh` 运行示例并等待其自测完成，退出码 0 表示通过。