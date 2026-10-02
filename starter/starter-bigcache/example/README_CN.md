# starter-bigcache Example

演示 starter-bigcache 的进程内热缓存。

## 功能验证

example 是一个自断言程序：[main.go](main.go) 里的 `runTest` 按顺序走完下列各项，任一项
失败即非零退出。

- **SET/GET**：用 `Set` 写入，用 `Get` 读回。
- **DELETE + miss**：删除后的 key 读回 `ErrEntryNotFound`。
- **实例隔离**：写入某个命名实例的 key，在另一个实例上不可见。
- **命中率统计**：`stats-enabled` 开着，`Stats()` 能读到计数。
- **缓存抽象**：同一个 `hot` 实例以 `*cache.Cache` bean（`bigcache:hot`）注入——JSON 编解码、
  未命中为 `cache.ErrMiss`，并演示逐调用 TTL 被**忽略**（BigCache 只按实例的 `life-window` 过期）。
- **硬顶淘汰**：写爆 `evict` 实例会丢掉最旧的条目；断言覆盖两头——常驻条目数少于写入数，
  **并且**淘汰钩子确实被触发过。
- **自定义 Driver**：[driver.go](driver.go) 注册一个 Driver bean，由
  `spring.bigcache.default.driver=hook` 选中。它自己拥有装配，从而能够触达
  `bigcache.Config.OnRemove`——starter 唯一没绑定的东西，也是上面那条淘汰断言得以成立的原因。
- **配置继承**：`spring.bigcache.default.*` 存放所有实例共享的键，因此 `hot` 只覆盖自己的
  `life-window`。
- **Get/Set/Delete 之外的缓存面**：`Len`、`Iterator` 与 `Reset`。
- **`life-window` 到底做什么**：两个实例共用 1s 窗口，只有 `clean-window` 不同。等过窗口后，一个不再
  服务陈旧条目，另一个照样服务——因为 `Get` 从不检查年龄，只有清理器会移除条目。
- **HTTP 端点**：`hot` 实例上的 `/get` 与 `/set`——由 `runTest` 实际驱动并断言，不只是写在文档里。

> 有意不提供 TTL 过期功能。BigCache 没有逐条目 TTL，因此抽象层的 `ttlSeconds` 是作为"被忽略"
> 写进文档的，而不是假装它能用；上面那条演示的正是相反的结果。

## 手动验证

终端 1，启动服务并保持运行：
```bash
cd starter-bigcache/example
go run . -manual
```

终端 2，执行验证命令：
```bash
curl http://127.0.0.1:9090/get
```

验证完成后 `Ctrl+C` 退出服务。

## 冒烟测试

```bash
./check.sh
```

`check.sh` 运行示例并等待其自测完成，退出码 0 表示通过。
