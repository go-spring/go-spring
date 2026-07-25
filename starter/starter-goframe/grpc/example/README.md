# starter-goframe/grpc Example

演示 starter-goframe/grpc 的 gRPC 服务注册与调用。

## 功能验证

- **gRPC 调用**：`Echo` 方法往返，请求消息原样返回

## 手动验证

终端 1，启动服务并保持运行：
```bash
cd starter-goframe/grpc/example
go run . -manual
```

终端 2，执行验证命令：
```bash
grpcurl -plaintext -d '{"message":"hello"}' \
  localhost:8001 echo.EchoService/Echo
# -> {"message":"hello"}
```

验证完成后 `Ctrl+C` 退出服务。

## 冒烟测试

```bash
./check.sh
```

`check.sh` 运行示例并等待其自测完成，退出码 0 表示通过。