# starter-grpc Example

演示 starter-grpc 的 gRPC 服务注册、拦截器与健康检查。

## 功能验证

- **gRPC 调用**：`Echo` 方法往返，请求消息原样返回
- **拦截器**：`LoggingInterceptor` 记录方法名和 `x-app` 元数据
- **元数据**：客户端发送 `x-app`，服务端返回 `x-handler`
- **健康检查**：标准 `grpc_health_v1` 健康服务

## 手动验证

```bash
cd starter-grpc/example
go run .
```

预期输出：
```
Response from server: Hello, gRPC!
Health status: SERVING
```

也可以使用 grpcurl 手动验证：
```bash
# 终端1：启动服务
go run .

# 终端2：grpcurl 测试
grpcurl -plaintext -d '{"message":"hello"}' \
  localhost:9494 EchoService/Echo
# -> {"message":"hello"}

grpcurl -plaintext localhost:9494 grpc.health.v1.Health/Check
# -> {"status":"SERVING"}
```

## 冒烟测试

```bash
./check.sh
```

`check.sh` 运行示例并等待其自测完成，退出码 0 表示通过。