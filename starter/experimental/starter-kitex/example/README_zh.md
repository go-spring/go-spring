# starter-kitex Example

演示 starter-kitex 的 Kitex RPC 服务注册与调用。

## 功能验证

- **RPC 调用**：客户端通过 `Echo` 方法往返，请求消息原样返回
- **服务注册**：通过 `ServiceRegister` 将 `EchoServiceImpl` 绑定到 Kitex Server
- **直连模式**：无需注册中心，客户端直接通过 `host:port` 连接

## 手动验证

终端 1，启动服务并保持运行：
```bash
cd starter-kitex/example
go run . -manual
```

服务正在 :8888 监听。可以用 Kitex 生成的客户端代码连接验证。

验证完成后 `Ctrl+C` 退出服务。

## 冒烟测试

```bash
./check.sh
```

`check.sh` 运行示例并等待其自测完成，退出码 0 表示通过。