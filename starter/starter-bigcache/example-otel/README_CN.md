# starter-bigcache OTel 示例

把 BigCache 统计作为 OpenTelemetry gauge 暴露，通过 Prometheus 拉取式 exporter 采集。

## 展示什么

- **统计 gauge**：`bigcache.hits`、`bigcache.misses`、`bigcache.delete_hits`、
  `bigcache.delete_misses`、`bigcache.collisions`、`bigcache.entries`、`bigcache.capacity`，
  按实例名打 label（`instance`）。
- **逐操作信号**：`bigcache.operation.total` 计数器，
  标签为 `operation` × `status` × `instance`。
- `starter-otel` 在进程内 Prometheus 端点（`:9090/metrics`）暴露以上全部，无需外部 collector。

示例驱动 `hot`（20 次 SET/GET 命中 + 5 次读不存在的 key），抓取 `/metrics` 并断言：

- `bigcache_hits{instance="hot"}` 至少等于驱动的 20 次命中，即 gauge 确实接在 `Stats()` 上，
  而不是永不更新的零；
- `get` 与 `set` 的逐操作计数器都在，未命中计入 `status="ok"` 而非 error，且没有任何
  缓存 key 出现在指标标签里。

**没有 span。** wrapper 有意不开 span：trace 的价值在于边缘，进程内的微秒级调用没有边缘。
计数器与 gauge 就是全部信号。

## 运行

```bash
cd starter-bigcache/example-otel
go run .
```

## 手动模式

```bash
go run . -manual
# 另开一个终端：
curl http://localhost:9090/metrics | grep bigcache
```

## 冒烟测试

```bash
./check.sh
```
