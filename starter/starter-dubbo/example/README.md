# starter-dubbo Example

演示 starter-dubbo 的 Dubbo RPC 服务注册与调用。

## 功能验证

- **RPC 调用**：客户端通过 Triple 协议调用 `Greet` 方法，验证请求/响应正确性
- **服务注册**：通过 `RegisterService` 将 `GreetProvider` 注册为 Dubbo 服务

## 手动验证

```bash
cd starter-dubbo/example
go run .
```

预期输出：
```
Response from server: Hello, Dubbo-Go!
```

程序通过 `runTest()` 自动发起客户端调用并断言响应结果，断言失败时以非零退出码退出。

## 冒烟测试

```bash
./check.sh
```

`check.sh` 运行示例并等待其自测完成，退出码 0 表示通过。