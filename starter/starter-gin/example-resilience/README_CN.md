# starter-gin 韧性(resilience)示例

演示 starter-gin 的**入站韧性准入**:开启 `spring.gin.server.resilience.enabled` 后,每个请求都会通过所选韧性驱动的 `Executor`,超过配置限流的突发流量会在执行业务处理器之前被以 HTTP 429 丢弃(熔断打开时为 503)。

## 特性

- **纯配置驱动准入**:无需任何中间件代码——starter 根据 `spring.gin.server.resilience.*` 自动接入韧性准入中间件。
- **限流**:超过 `rate-limit` 的突发被以 `429` 拒绝。
- **驱动可插拔**:`driver=default`(内置,零依赖)或 `sentinel`。

## 手动验证

```bash
cd starter/starter-gin/example-resilience
go run . -manual
```

另开一个终端,突刺该路由:

```bash
for i in $(seq 1 20); do curl -s -o /dev/null -w "%{http_code}\n" http://127.0.0.1:8081/; done
```

应能看到 `200` 与 `429` 混合。

## 冒烟测试

```bash
./check.sh
```

`check.sh` 触发一次突发并断言服务既有 `200` 也会 `429` 丢弃;退出码 0 表示通过。
