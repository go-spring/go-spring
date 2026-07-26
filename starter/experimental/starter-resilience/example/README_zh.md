# starter-resilience Example

演示 starter-resilience 的 Sentinel 熔断与限流。

## 功能验证

- **限流（Server 侧）**：5 QPS 限流，超出部分返回 429
- **熔断（Client 侧）**：连续 3 次拒绝拨号后熔断器打开
- **Sentinel 驱动**：通过 Sentinel 实现 resilience 语义

## 手动验证

```bash
cd starter-resilience/example
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