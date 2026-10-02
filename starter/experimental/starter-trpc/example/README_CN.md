# starter-trpc Example

演示 starter-trpc 的 tRPC 服务注册与调用。

## 功能验证

- **RPC 调用**：客户端通过 `Greet` 方法往返，请求名称返回问候语
- **服务注册**：通过 `ServiceRegister` 将 `GreetServiceImpl` 绑定到 tRPC Server
- **直连模式**：通过 `ip://` target scheme 直连，无需注册中心

## 手动验证

终端 1，启动服务并保持运行：
```bash
cd starter-trpc/example
go run . -manual
```

服务正在 :8000 监听。可以用 tRPC 生成的客户端 proxy 代码连接验证。

验证完成后 `Ctrl+C` 退出服务。

## 冒烟测试

```bash
./check.sh
```

`check.sh` 运行示例并等待其自测完成，退出码 0 表示通过。