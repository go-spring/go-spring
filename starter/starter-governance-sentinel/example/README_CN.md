# starter-governance-sentinel 示例

演示融合路径：sentinel-golang 作为 resilience 引擎的**限流算法**被选中。

## 展示内容

- 一个 `ratelimit.Factory` 同时收集内置 provider 和 sentinel provider；
  策略用 `limiter: sentinel` 指定算法。
- 调用仍然走 resilience 引擎，所以拒绝是引擎的中立 `ErrRateLimited`，
  不会把 sentinel 的错误泄露出去。
- 预算 2/s：连续三次请求放行两次、拒绝一次。

## 运行

```bash
cd starter-governance-sentinel/example
go run .
```

预期输出：

```
sentinel limiter: 2 admitted, 1 rate-limited (budget 2/s)
resilience seams smoke: OK
```

## 冒烟测试

```bash
./check.sh
```

退出码 0 即通过。
