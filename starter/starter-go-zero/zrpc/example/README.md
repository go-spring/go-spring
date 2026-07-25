# starter-go-zero/zrpc Example

演示 starter-go-zero/zrpc 的 gRPC 服务注册与健康检查。

## 功能验证

- **gRPC 健康检查**：注册标准 `grpc_health_v1` 健康服务，验证 SERVING 状态

## 手动验证

```bash
cd starter-go-zero/zrpc/example
go run .
```

预期输出：
```
Health status from server: SERVING
```

也可以使用 grpcurl 手动验证：
```bash
# 终端1：启动服务
go run .

# 终端2
grpcurl -plaintext localhost:8081 grpc.health.v1.Health/Check
# -> {"status":"SERVING"}
```

## 冒烟测试

```bash
./check.sh
```

`check.sh` 运行示例并等待其自测完成，退出码 0 表示通过。