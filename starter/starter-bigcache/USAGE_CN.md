# starter-bigcache 使用说明

详细使用文档。概览见 [README](README_CN.md)。锚定自校验的 [example/](example/)
（`example/check.sh`）。BigCache 语义（分片、life-window 淘汰、容量硬顶）见
[官方文档](https://github.com/allegro/bigcache)——本文只讲 go-spring 的接线增量。

**激活条件**：任一 `spring.bigcache.*` key（`OnProperty("spring.bigcache")` 前缀匹配——
每个 `spring.bigcache.<name>` 条目创建一个实例）。

## 1. 快速开始

无外部依赖（纯进程内缓存）。

```properties
spring.bigcache.main.life-window=10m
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
Get/Set/Delete 会经过 observe（访问日志/指标/trace）与 resilience（限流/熔断，经治理
中心）接线。原生客户端以内嵌字段 `Cache.BigCache` 提供，供第三方 API 使用。

## 2. 全量配置参考

按实例，位于 `spring.bigcache.<name>`：

| Key | 类型 | 默认值 | 必填 | 说明 |
|-----|------|--------|------|------|
| `spring.bigcache.<name>.shards` | int | 1024 | 否 | 必须是 2 的幂（bigcache 要求） |
| `spring.bigcache.<name>.life-window` | duration | 10m | 否 | 条目寿命，超过即视为过期 |
| `spring.bigcache.<name>.clean-window` | duration | 1m | 否 | 后台淘汰间隔；0 关闭后台清理 |
| `spring.bigcache.<name>.max-entries-in-window` | int | 600000 | 否 | 仅用于启动期预分配 |
| `spring.bigcache.<name>.max-entry-size` | int | 500 | 否 | 预期单条目最大字节数；仅预分配提示 |
| `spring.bigcache.<name>.hard-max-cache-size` | int | 0 | 否 | 内存硬顶（MB）；0 = 不限 |
| `spring.bigcache.<name>.stats-enabled` | bool | false | 否 | 开启 `Stats()` 计数（同时喂 OTel gauge） |

无 per-config 的 `driver` key：装配由可选 Driver bean（见 §3）或内置 `DefaultDriver` 负责。

⚠ 当同一后端实例又通过 `spring.cache.<name>.driver=bigcache:<instance>` 暴露时，
`Cache.Set` 传入的 TTL 会被忽略——BigCache 按单一全局 `life-window` 统一过期。

## 3. beans / driver / 观测

- 每实例一个 `*StarterBigCache.Cache` bean，名为 `<name>`；`Init` 装配 observe+resilience，
  `Destroy` 调用 `Close()`（停止淘汰 goroutine）。
- 每实例一个健康 indicator，名 `bigcache:<name>`（经 `[]health.Indicator` 收集）。
- 每实例 OTel gauge（meter `go-spring.org/starter-bigcache`，label `cache.name`）：
  `bigcache.hits` / `misses` / `delete_hits` / `delete_misses` / `collisions` / `entries` /
  `capacity`。引入 starter-otel 时导出，否则为 no-op。
- 缓存抽象 driver：向 starter-cache 注册了 `bigcache`，故
  `spring.cache.<name>.driver=bigcache:<instance>` 可将实例暴露为 `*cache.Cache` bean。
- 自定义客户端装配：装配由 `Driver` 接口（`CreateClient(ctx, Config)`，driver.go）负责。公司/
  伞包 starter 可把自己的 `Driver` 作为**可选容器 bean** 提供
  （`gs.Provide(func() StarterBigCache.Driver{...})`，因为是 bean，可在装配期注入从配置绑定
  的配置）；无该 bean 时 starter 在装配内回退到内置 `DefaultDriver`。这仍是设置
  `bigcache.Config.OnRemove` 等字段的唯一途径。
- resilience：实例级资源标签 `bigcache:<name>`；策略来自治理中心（starter-govern），
  未引入时为 no-op。

## 4. 设计体检表

| 指标 | 数值 |
|------|------|
| 配置 key 总数 | 每实例 7 个 + 全局 3 个 |
| 其中必填 | 0 |
| quickstart 前置外部依赖 | 0 |
| "注意/坑" 条数 | 1（经缓存抽象时 TTL 被忽略） |

设计嫌疑清单：

1. `Driver` 与 starter-cache 的 `Driver` 类型同名不同签名——文档层面的可读性隐患。
2. `DefaultDriver` 作为 `driver` key 的默认取值（`driver:=DefaultDriver`），字面量
   "DefaultDriver" 泄漏进用户配置。
3. README 此前记载的 `SetOnRemove` / `AsCache` 在代码中不存在——已于 2026-08-27 修正；
   example 的 Feature-5 注释仍引用已删除的 `SetOnRemove`。
