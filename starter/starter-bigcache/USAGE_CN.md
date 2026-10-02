# starter-bigcache 使用说明

详细使用文档。概览见 [README](README_CN.md)。锚定自校验的 [example/](example/)
（`example/check.sh`）。BigCache 语义（分片、life-window 淘汰、容量硬顶）见
[官方文档](https://github.com/allegro/bigcache)——本文只讲 go-spring 的接线增量。

**激活条件**：任一 `spring.bigcache.instances.*` key（`OnProperty("spring.bigcache")` 前缀匹配——
每个 `spring.bigcache.instances.<name>` 条目创建一个实例）。

## 1. 快速开始

无外部依赖（纯进程内缓存）。

```properties
spring.bigcache.instances.main.life-window=10m
```

```go
package main

import (
	"go-spring.org/spring/gs"
	StarterBigCache "go-spring.org/starter-bigcache"
	_ "go-spring.org/starter-bigcache"
)

type Service struct {
	Cache *StarterBigCache.Cache `autowire:"main"`
}

func main() { gs.Run() }
```

注入的 bean 是 starter 的 `*Cache` 封装而非原生 `*bigcache.BigCache`：其
Get/Set/Delete 会声明各自的操作身份，并经 resilience 层（限流/熔断，经治理中心）
执行——span、指标与访问日志由该层发射，starter 自身不再发射。原生客户端存放于未导出字段
且不提供访问器，其余原生方法（Stats/Len/Capacity/Reset/Close 等）以纯委托形式重新导出。

## 2. 全量配置参考

按实例，位于 `spring.bigcache.instances.<name>`：

| Key | 类型 | 默认值 | 必填 | 说明 |
|-----|------|--------|------|------|
| `spring.bigcache.instances.<name>.shards` | int | 1024 | 否 | 必须是 2 的幂（bigcache 要求） |
| `spring.bigcache.instances.<name>.life-window` | duration | 10m | 否 | 条目寿命，超过即视为过期 |
| `spring.bigcache.instances.<name>.clean-window` | duration | 1m | 否 | 后台淘汰间隔；0 关闭后台清理 |
| `spring.bigcache.instances.<name>.max-entries-in-window` | int | 600000 | 否 | 仅用于启动期预分配 |
| `spring.bigcache.instances.<name>.max-entry-size` | int | 500 | 否 | 预期单条目最大字节数；仅预分配提示 |
| `spring.bigcache.instances.<name>.hard-max-cache-size` | int | 0 | 否 | 内存硬顶（MB）；0 = 不限 |
| `spring.bigcache.instances.<name>.stats-enabled` | bool | false | 否 | 开启 `Stats()` 计数（同时喂 OTel gauge） |
| `spring.bigcache.instances.<name>.health` | bool | true | 否 | 是否注册 `bigcache:<name>` 健康 indicator；false 则不卷入聚合健康 |

`driver` key 按名指定 Driver bean：留空 = 先回退家族级 `spring.<family>.default.driver`，再按类型注入唯一 Driver bean（无 bean 时装配内回退到内置
`DefaultDriver`）；配置 bean 名则显式选定一个，指定的 bean 不存在则启动失败。

⚠ 经缓存抽象访问（注入名为 `bigcache:<instance>` 的 `*cache.Cache` bean）时，
`Cache.Set` 传入的 TTL 会被忽略——BigCache 按单一全局 `life-window` 统一过期。

## 3. beans / driver / 观测

- 每实例一个 `*StarterBigCache.Cache` bean，名为 `<name>`；构造（`NewCache`）即注册统计 gauge
  并装配治理，`Destroy` 调用 `Close()`（停止淘汰 goroutine）。
- 每实例一个健康 indicator，名 `bigcache:<name>`（经 `[]health.Indicator` 收集；由 `health` 控制，默认 true）。
- 每次调用（由 resilience 层发射，meter `go-spring.org/cloud/resilience`）：
  调用级 `db.client.operation.duration`（含重试与退避）、尝试级 `db.client.attempt.duration`
  （每次下游尝试一条记录）、在途 `db.client.active_requests`，以及按状态分类的
  `resilience.client.calls`。标签为 `db.system=bigcache`、`db.operation=<get|set|delete>`、`status`；
  key 仅以 `db.statement` 进 span/日志，绝不作为指标标签。
- 每实例 OTel gauge（meter `go-spring.org/starter-bigcache`，label `cache.name`）：
  `bigcache.hits` / `misses` / `delete_hits` / `delete_misses` / `collisions` / `entries` /
  `capacity`。这是 starter 仍自有的唯一信号——按进程而非按调用，resilience 层无从发射。
  引入 starter-otel 时导出，否则为 no-op。
- 缓存抽象 bean：除包装类型外，每实例另提供一个 `*cache.Cache` bean，名为
  `bigcache:<instance>`（适配器在 this package's bytecache.go）——用 autowire tag
  `bigcache:<instance>` 按名注入。无人注入则不实例化，因此无配置开关。
- 自定义客户端装配：装配由 `Driver` 接口（`CreateClient(ctx, name, Config, cloud.ClientParams)`，driver.go）负责。公司/
  伞包 starter 可把自己的 `Driver` 作为**可选容器 bean** 提供
  （`gs.Provide(func() StarterBigCache.Driver{...})`，因为是 bean，可在装配期注入从配置绑定
  的配置）；无该 bean 时 starter 在装配内回退到内置 `DefaultDriver`。这仍是设置
  `bigcache.Config.OnRemove` 等字段的唯一途径。当容器中存在多个 Driver bean 时，实例可按名指定：
  `spring.bigcache.instances.<name>.driver = <bean 名>`（留空 = 先回退家族级 `spring.<family>.default.driver`，再按类型注入唯一 Driver bean；指定的 bean
  不存在则启动失败）。
- resilience：实例级服务标签 `bigcache:<name>`；执行器在构造期由 `NewCache` 从传入的
  `cloud.ClientParams` 派生（`Resilience` 为 `*resilience.Manager` bean，`Fault` 为
  `*fault.Injector` bean，用于包裹加固）。容器内没有治理 bean 时（零值 `Governance`），
  降级为 `resilience.Unmanaged`——仍可观测，仅一次性告警提示无保护，而非静默裸跑。

## 4. 设计体检表

| 指标 | 数值 |
|------|------|
| 配置 key 总数 | 每实例 7 个 + 全局 3 个 |
| 其中必填 | 0 |
| quickstart 前置外部依赖 | 0 |
| "注意/坑" 条数 | 1（经缓存抽象时 TTL 被忽略） |

设计嫌疑清单：

1. README 此前记载的 `SetOnRemove` / `AsCache` 在代码中不存在——已于 2026-08-27 修正；
   example 的 Feature-5 注释仍引用已删除的 `SetOnRemove`。
