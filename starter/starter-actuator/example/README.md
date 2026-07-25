# starter-actuator Example

演示 starter-actuator 的健康检查、就绪探测和启动探测端点。

## 功能验证

- **健康检查 `/health`**：始终返回 UP
- **就绪探测 `/readiness`**：聚合自定义 HealthIndicator，反映依赖状态
- **启动探测 `/startup`**：反映启动完成状态
- **应用信息 `/info`**：暴露应用元数据
- **动态切换**：通过开关 HealthIndicator 验证 readiness 状态变化

## 手动验证

```bash
cd starter-actuator/example
go run .
```

预期输出（断言通过后程序自动退出）：
```
UP
UP
DOWN
UP
```

也可以手动 curl 验证：
```bash
# 终端1：启动服务（保持运行）
go run .

# 终端2
curl http://127.0.0.1:9090/health
# -> {"status":"UP"}

curl http://127.0.0.1:9090/readiness
# -> {"status":"UP"}

curl http://127.0.0.1:9090/info
# -> {"app":{...}}
```

## 冒烟测试

```bash
./check.sh
```

`check.sh` 运行示例并等待其自测完成，退出码 0 表示通过。