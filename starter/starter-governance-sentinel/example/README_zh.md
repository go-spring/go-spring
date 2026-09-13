# starter-governance-sentinel Example

演示 starter-governance-sentinel 的 Sentinel 熔断与限流。

## 功能验证

- **dialer seam 上的熔断**：对着无人监听的地址，连续 3 次拒绝拨号后熔断器打开，第 4 次直接以 `ErrCircuitOpen` 短路，不碰网络
- **组合策略**：一份 policy 同时含限流 + 熔断 + 重试——上游先 503 两次再成功，在重试预算内被透明恢复
- **Sentinel 驱动**：两者都跑在 `NewSentinelDriver()`（容器外入口）构建的 `sentinel` 驱动上

## 手动验证

```bash
cd starter-governance-sentinel/example
go run .
```

预期输出：
```
resilience seams smoke: OK
```

## 冒烟测试

```bash
./check.sh
```

`check.sh` 运行示例并等待其自测完成，退出码 0 表示通过。