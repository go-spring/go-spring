# starter-goframe/grpc Example

演示 starter-goframe/grpc 的 gRPC 服务注册与调用。

## 功能验证

- **gRPC 调用**：`Echo` 方法往返，请求消息原样返回

## 手动验证

```bash
cd starter-goframe/grpc/example
go run .
```

预期输出：
```
Response from server: hello
```

也可以使用 grpcurl 手动验证：
```bash
# 终端1：启动服务
go run .

# 终端2
grpcurl -plaintext -d '{"message":"hello"}' \
  localhost:8001 echo.EchoService/Echo
# -> {"message":"hello"}
```

## 冒烟测试

```bash
./check.sh
```

`check.sh` 运行示例并等待其自测完成，退出码 0 表示通过。