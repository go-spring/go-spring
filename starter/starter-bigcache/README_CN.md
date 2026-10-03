# starter-bigcache

[English](README.md) | [中文](README_CN.md)

`starter-bigcache` 基于 [BigCache](https://github.com/allegro/bigcache) 提供了进程内缓存封装，
让你在 Go-Spring 应用中轻松集成并使用高性能、对 GC 友好的内存缓存。

## 安装

```bash
go get go-spring.org/starter-bigcache
```

## 快速开始

### 1. 引入 `starter-bigcache` 包

参考 [main.go](example/main.go) 文件。

```go
import _ "go-spring.org/starter-bigcache"
```

### 2. 配置 BigCache 实例

在项目的[配置文件](example/conf/app.properties)中添加 BigCache 配置，例如：

```properties
spring.bigcache.instances.main.life-window=10m
```

### 3. 注入 BigCache 实例

参考 [main.go](example/main.go) 文件。

```go
import StarterBigCache "go-spring.org/starter-bigcache"

type Service struct {
    Cache *StarterBigCache.Cache `autowire:"main"`
}
```

注入的 bean 是 starter 的 `*Cache` 封装。原生 `*bigcache.BigCache` 存放于未导出字段且不提供访问器，
因此所有操作都留在封装之后：Get/Set/Delete 自己发射操作的 span 与指标，其余原生方法
（Stats、Len、Reset、Close 等）则以纯委托形式重新导出。这些调用不会被限流或熔断，也不写访问日志：
进程内缓存没有需要保护的外部依赖，也没有进程外的调用需要记录。见[设计说明](README_CN.md#设计说明)。

### 4. 使用 BigCache 实例

参考 [main.go](example/main.go) 文件。

```go
err := s.Cache.Set(ctx, "key", []byte("value"))
value, err := s.Cache.Get(ctx, "key")
```

## 核心特性

[example/](example/) 自断言接线与整个命令面，`example/check.sh` 一把跑完。按顺序覆盖：

* **SET/GET** —— 用 `Set(...)` 写入，再用 `Get(...)` 读回。
* **DELETE + 未命中** —— 用 `Delete(...)` 删除键，确认随后的 `Get(...)` 返回 `ErrEntryNotFound`。
* **实例隔离** —— 写入某个命名实例的键在另一个实例中不可见，证明多实例接线正确。
* **Get/Set/Delete 之外** —— `Len`、`Iterator` 与 `Reset`。
* **`life-window` 的真实语义** —— 两个实例共用 1s 窗口、只有 `clean-window` 不同：一个不再服务陈旧条目，
  另一个照样服务。
* **自定义 Driver** —— 一个按名选择的 Driver bean，用来触达 `bigcache.Config.OnRemove`。
* **缓存抽象** —— 同一实例经 `cloud/cache` 访问，含被忽略的逐调用 TTL。
* **HTTP handler** —— 被实际驱动并断言，不只是写在文档里。

列表见 [example/README_CN.md](example/README_CN.md)；可观测那一侧（gauge、逐操作指标与 span）见
[example-otel/](example-otel/)。

## 高级特性

* **支持多个 BigCache 实例**：你可以在配置文件中定义多个 BigCache 实例，并在项目中按名称引用它们。
* **支持 BigCache 扩展**：你可以通过实现 `Driver` 接口来扩展 BigCache 的创建逻辑。当容器中存在多个
  Driver bean 时，实例可按名指定：`spring.bigcache.instances.<name>.driver = <bean 名>`（留空 = 按类型注入唯一
  Driver bean；指定的 bean 不存在则启动失败）。
* **可观测**：Get/Set/Delete 自己发射 `get`/`set`/`delete` span，以及 `bigcache.operation.total` 计数器与
  `bigcache.operation.duration` 直方图，标签为 `operation` × `status` × `instance`。
  key 作为 `bigcache.key` 只进 span，永不作为指标标签。
* **命中率统计**：默认开启（`stats-enabled`）——读取 `cache.Stats()` 获取命中/未命中/冲突计数，
  或抓取 starter 导出的 OTel 可观测 gauge。设 `stats-enabled=false` 可省掉 bigcache 开启期间维护的每 key 记账。
* **淘汰/过期回调**：通过自定义 `Driver`（实现 `CreateClient`，在构建的
  `bigcache.Config` 上设置 `OnRemove`）注册条目淘汰/过期回调。
* **优雅关闭**：destroy 回调会调用 `Close()`，停止后台清理 goroutine。
* **缓存抽象后端**：除包装类型外，每个实例另提供一个 `cloud/cache.Cache` bean，名为
  `bigcache:<instance>`——用 autowire tag `bigcache:<instance>` 按名注入即可经缓存抽象使用。
  该 bean 是惰性的：无人注入则不实例化，因此无配置开关。
  注意 BigCache 按单一全局 `life-window` 过期，因此每次调用的 TTL 会被忽略；若纯粹作为本地层，通常更适合用
  `cache.Memory`（不序列化、保留具体类型）。

## 设计说明

* **进程内，不是缓存集群。** 条目存在本进程堆上：两个副本各持一份、彼此不失效。这是零跳读取的代价——
  如果需要跨副本一致，请换用走网络的缓存后端。
* **容量在启动期定死。** `shards` 必须是 2 的幂，`max-entries-in-window` × `max-entry-size` 大致框定实例
  预分配的内存。运行期不支持扩容，所以实例的形态要一开始就定好。
* **条目是 `[]byte`。** 缓存把值当不透明字节；编解码由调用方自己做。经 `cloud/cache` 那条路会用 JSON 序列化。
* **`life-window` 不是读侧 TTL**。它把条目标记为陈旧，但不把它藏起来：`Get` 不看年龄，一律返回；
  移除是清理器的职责，每个 `clean-window` 一次。`clean-window=0` 时一个值会活过它的 `life-window`，
  直到容量淘汰把它拿走——启动期容量规划默认你懂这个细节。
* **`life-window` 是整个实例一个 TTL**，不是逐条目：一个实例里的所有条目共用它。需要几种 TTL 就建几个命名实例——
  这正是多实例桶的用途。
* **本 starter 只提供存储，策略在上一层。** loader/refresh 语义属于 `cloud/cache`，不属于某个后端 starter：
  本模块提供缓存本身，抽象层提供重载行为。
* **自己观测，不受治理。** 本 starter 自己发 span 与指标，完全不参与框架的执行器链：不注入治理 bean、
  不接收 `ClientParams`、没有任何规则能给它设防。限流或熔断在这里都是错的——依赖就在本进程内，
  "下游挂了"不可能发生；而一次瞬时的 `ErrEntryTooLarge` 把熔断打开后，反而会拒掉本来能命中的 `Get`。
  它也不写访问日志：缓存调用频率高，而且它到不了进程外的任何东西。它与框架唯一共享的是 span 属性载体
  ——它开的 span 会自动接住上层贡献的属性，两边都不用配合。
