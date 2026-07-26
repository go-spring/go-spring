# starter-actuator Example

演示 starter-actuator 的健康检查、就绪探测和启动探测端点。

## 功能验证

- **健康检查 `/health`**：始终返回 UP
- **就绪探测 `/readiness`**：聚合自定义 HealthIndicator，反映依赖状态
- **启动探测 `/startup`**：反映启动完成状态
- **应用信息 `/info`**：暴露应用元数据
- **动态切换**：通过开关 HealthIndicator 验证 readiness 状态变化

## 手动验证

终端 1，启动服务并保持运行：
```bash
cd starter-actuator/example
go run . -manual
```

终端 2，执行验证命令：
```bash
curl http://127.0.0.1:9370/health
# -> {"status":"UP"}

curl http://127.0.0.1:9370/readiness
# -> {"status":"UP"}

curl http://127.0.0.1:9370/info
# -> {"app":{...}}
```

验证完成后 `Ctrl+C` 退出服务。

## 冒烟测试

```bash
./check.sh
```

`check.sh` 运行示例并等待其自测完成，退出码 0 表示通过。
