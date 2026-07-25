# starter-thrift Example

演示 starter-thrift 的 Thrift RPC 服务注册、中间件装饰器与多协议配置。

## 功能验证

- **RPC 调用**：客户端通过 `Echo` 方法往返，compact 协议 + framed 传输
- **中间件装饰器**：`loggingProcessor` 包裹生成的 Processor，记录每次 RPC 调用
- **多轮调用**：两次独立 RPC 调用，验证中间件计数 = 2

## 手动验证

```bash
cd starter-thrift/example
go run .
```

预期输出：
```
Response from server: Hello, Thrift!
Response from server: Middleware works!
```

## 冒烟测试

```bash
./check.sh
```

`check.sh` 运行示例并等待其自测完成，退出码 0 表示通过。