# starter-bigcache OTel 示例

把 BigCache 统计作为 OpenTelemetry gauge 暴露，通过 Prometheus 拉取式 exporter 采集。

## 展示什么

- **统计 gauge**：`bigcache.hits`、`bigcache.misses`、`bigcache.delete_hits`、
  `bigcache.delete_misses`、`bigcache.collisions`、`bigcache.entries`、`bigcache.capacity`，
  按实例名打 label（`cache.name`）。
- **逐操作信号**：`bigcache.operation.total` 计数器与 `bigcache.operation.duration` 直方图，
  标签为 `operation` × `status` × `cache.name`。
- `starter-otel` 在进程内 Prometheus 端点（`:9090/metrics`）暴露以上全部，无需外部 collector。

示例驱动 `hot`（20 次 SET/GET 命中 + 5 次读不存在的 key），抓取 `/metrics` 并断言：

- `bigcache_hits{cache_name="hot"}` 至少等于驱动的 20 次命中，即 gauge 确实接在 `Stats()` 上，
  而不是永不更新的零；
- `get` 与 `set` 的逐操作计数器与直方图都在，未命中计入 `status="ok"` 而非 error，且没有任何
  缓存 key 出现在指标标签里。

**span。** wrapper 每个操作都开一个 span，它们走内建的 `stdout` exporter
（`spring.observability.trace.exporter=stdout`），直接打在示例输出里——去读它，而不是相信属性名。
想送到 collector，换成 `otlp-grpc` 加一个 endpoint 即可。

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
