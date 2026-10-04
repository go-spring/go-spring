# starter-bigcache 使用说明 — 参考手册

详细使用文档。概览见 [README](README_CN.md)。本文所有行为声明均已对照本 starter 的源码
（`starter.go`、`config.go`、`client.go`、`driver.go`、`bytecache.go`、`observe.go`）与自校验的
[example/](example/) 核对——抽查点以 `文件:行` 标注。**BigCache 自身的语义——分片、life-window
淘汰、内存硬顶、只存字节——见[官方 README](https://github.com/allegro/bigcache)**；本文只讲
go-spring 的增量：接线、配置绑定、信号词汇。

**激活条件**：任一 `spring.bigcache.instances.*` key（[starter.go:34](starter.go#L34)）。每个
`spring.bigcache.instances.<name>` 条目创建一个名为 `<name>` 的 `*StarterBigCache.Cache` bean。
纯进程内缓存：无外部依赖、无地址、无连接池、无 health indicator——没有可探测的连通性。

---

## 1. 完整工程示例

可运行的工程是 [example/](example/)；本节按顺序说明它的内容。

### 1.1 依赖

```bash
go get go-spring.org/starter-bigcache
```

**前置外部系统：无。** example 自包含；`example/check.sh` 运行它，任一断言失败即非零退出。

### 1.2 配置

复制 [example/conf/app.properties](example/conf/app.properties)——三个实例，各展示一种形态：

```properties
# HTTP handler 由 gs 内建的 server 提供，地址在这里声明而不是留给框架默认值。
spring.http.server.addr=127.0.0.1:9090

# 家族级默认值：实例未设置的 key 一律继承。
spring.bigcache.default.shards=1024
spring.bigcache.default.life-window=10m
spring.bigcache.default.stats-enabled=true
spring.bigcache.default.driver=hook

# hot 只覆盖自己的存活时长；shards 与 stats-enabled 都是继承来的。
spring.bigcache.instances.hot.life-window=1m

# cold：不同的 life-window 正是第二个实例存在的理由。
spring.bigcache.instances.cold.life-window=30m

# cleaned / uncleaned：同一个 1s life-window，只有 clean-window 不同——
# 用来钉死"没有清理器时陈旧条目照样被返回"这一点。
spring.bigcache.instances.cleaned.life-window=1s
spring.bigcache.instances.cleaned.clean-window=100ms
spring.bigcache.instances.uncleaned.life-window=1s
spring.bigcache.instances.uncleaned.clean-window=0

# evict：极小且硬顶，也是唯一需要挂钩 driver 的实例——写满 1MB 即触发淘汰。
spring.bigcache.instances.evict.driver=hook
spring.bigcache.instances.evict.shards=2
spring.bigcache.instances.evict.life-window=1m
spring.bigcache.instances.evict.clean-window=0
spring.bigcache.instances.evict.max-entries-in-window=4096
spring.bigcache.instances.evict.max-entry-size=1024
spring.bigcache.instances.evict.hard-max-cache-size=1
```

某实例未设置的 key 会回退到家族级的 `spring.bigcache.default.<key>` 桶
（[starter.go:38](starter.go#L38)），因此多个实例的公共部分只写一次。

**⚠ 实例必须有自己的至少一个 key。** 激活条件是有 `spring.bigcache.instances.*` key，而每个
实例是那个 map 的一个子节点——一个全部靠继承的名字没有子节点，也就不会创建 bean。继承继承的是
*值*，不是"声明这个实例存在"。

### 1.3 代码

```go
package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/allegro/bigcache/v3"
	"go-spring.org/cloud/cache"
	"go-spring.org/log"
	"go-spring.org/spring/gs"
	StarterBigCache "go-spring.org/starter-bigcache"
	_ "go-spring.org/starter-bigcache"
)

// 注入 wrapper，每个已配置实例一个字段，按 bean 名匹配。
// 原生 *bigcache.BigCache 不作为 bean 注入：只能通过 wrapper 的 Client 字段拿到，
// 只读交付（见 README「设计说明」）。
type Service struct {
	Hot   *StarterBigCache.Cache `autowire:"hot"`
	Cold  *StarterBigCache.Cache `autowire:"cold"`
	Evict *StarterBigCache.Cache `autowire:"evict"`

	// 同一个 hot 实例经缓存抽象暴露为 bean "bigcache:hot"（§3.1）
	// ——同一份缓存的两张脸，不是两份缓存。
	Cache *cache.Cache `autowire:"bigcache:hot"`
}

func main() {
	// Service 不被任何其他 bean 引用，必须导出为根对象，否则容器不会实例化它。
	svrBean := gs.Provide(&Service{}).Export(gs.As[gs.Rooter]())

	http.HandleFunc("/get", func(w http.ResponseWriter, r *http.Request) {
		s := svrBean.Interface().(*Service)
		v, err := s.Hot.Get(context.Background(), "key")
		if err != nil {
			_, _ = w.Write([]byte(err.Error()))
			return
		}
		_, _ = w.Write(v)
	})
	http.HandleFunc("/set", func(w http.ResponseWriter, r *http.Request) {
		s := svrBean.Interface().(*Service)
		if err := s.Hot.Set(context.Background(), "key", []byte("value")); err != nil {
			_, _ = w.Write([]byte(err.Error()))
			return
		}
		_, _ = w.Write([]byte("OK"))
	})

	go func() { runTest(svrBean.Interface().(*Service)) }()
	gs.Run()
}

func runTest(s *Service) {
	ctx := context.Background()

	// SET/GET 往返。
	_ = s.Hot.Set(context.Background(), "key", []byte("value"))
	v, _ := s.Hot.Get(context.Background(), "key")
	fmt.Println("get:", string(v))

	// DELETE 之后是 miss：bigcache 用 ErrEntryNotFound 表示不存在。
	_ = s.Hot.Delete(context.Background(), "key")
	_, err := s.Hot.Get(context.Background(), "key")
	fmt.Println("miss is ErrEntryNotFound:", errors.Is(err, bigcache.ErrEntryNotFound))

	// 实例之间互不可见：写入 cold 的 key 在 hot 上查不到。
	_ = s.Cold.Set(context.Background(), "only-cold", []byte("cold-value"))
	_, err = s.Hot.Get(context.Background(), "only-cold")
	fmt.Println("hot does not see cold:", errors.Is(err, bigcache.ErrEntryNotFound))

	// stats-enabled 默认开，这里的命中/未命中计数可读——同时也导出为 OTel gauge（§4.1）。
	st := s.Hot.Stats()
	fmt.Println("hot stats:", st.Hits, st.Misses)

	// 经缓存抽象：JSON 编解码，未命中为 cache.ErrMiss。逐调用 TTL 被忽略（§3），
	// 所以这个值能活过它被赋予的 1 秒。
	_ = s.Cache.Set(ctx, "abstraction", "value", 1)
	time.Sleep(1100 * time.Millisecond)
	var av string
	fmt.Println("still there:", s.Cache.Get(ctx, "abstraction", &av) == nil, av)

	// 写爆硬顶的 evict 缓存（§4.2）。
	big := make([]byte, 900)
	for i := range 4000 {
		_ = s.Evict.Set(fmt.Sprintf("k-%d", i), big)
	}
	log.Infof(ctx, log.TagAppDef, "done")
}
```

完整文件（含手动验证开关、HTTP 往返断言与优雅退出）见 [example/main.go](example/main.go)；
它注册的自定义 `Driver` 在 [example/driver.go](example/driver.go)（§3.2）。

### 1.4 运行与验证

```bash
cd example && ./check.sh      # 运行 example；任一断言失败即非零退出
```

想手工拨弄，就用 `-manual` 起服务，用两个 handler 试：

```bash
go run . -manual
curl http://127.0.0.1:9090/set    # OK
curl http://127.0.0.1:9090/get    # value
```

---

## 2. 装配与时序

### 2.1 bean 生命周期

```
import _ "go-spring.org/starter-bigcache"
  └─ gs.Module(OnProperty("spring.bigcache.instances"))        [starter.go:34]
     仅当存在至少一个 spring.bigcache.instances.* key 时触发
        └─ flatten.WithFallback(instances, default)            [starter.go:38]
             实例未设的 key 回退到 spring.bigcache.default.<k>
              └─ conf.BindEach("${spring.bigcache.instances}") [starter.go:39]
                 按 map 的 key 为每个 <name> 绑定一个 Config
                    └─ r.Provide(newClient, name@1, c@2, driver@3).Name(name)
                       .Destroy((*Cache).Close)              [starter.go:48]
                       索引 0（*gs.ContextProvider）自动装配；Driver 参数由
                       ${...<name>.driver} 选定

gs.Run()
  ├─ ctor newClient                                            [starter.go:72]
  │    d == nil → DefaultDriver{}（未注册公司 Driver bean）
  │    → DefaultDriver.CreateClient(ctx, name, c)              [driver.go:81]
  │        bigcache.DefaultConfig(LifeWindow) + 其余六个旋钮
  │        → bigcache.New(ctx, conf)          ← 唯一可能因原生缓存而失败的步骤
  │          （例如 shards 不是 2 的幂）
  │        → NewCache(client, name)                          [client.go:78]
  │             → NewRawCache(client)                        [client.go:116]
  │             → NewObsCache(tail, client, name)            [client.go:161]
  │                  → newStatObserver(client, name)         [observe.go:229]
  │                      → buildInstruments()                [observe.go:170]
  │                        进程级，每进程只解析一次，走 singleton.Singleton
  │                        ——第一个被构造的 cache 为之后所有 cache 绑定这套仪器
  │                      把本实例的统计注册到共享 gauge 上，标签 instance=<name>
  │             链头成为嵌入的 InnerCache；Client 保留原生实例
  │             作为只读把手
  │    ctor 返回即 bean 完备：没有 Init 步骤；之后唯一允许触碰这个 cache 的
  │    是接流量前在链头外再包一层自定义层。
  ├─（可选）*cache.Cache bean "bigcache:<name>"               [starter.go:61]
  │    惰性——无人注入则不实例化
  ├─ 就绪：所有根可达 bean 构造完毕后应用就绪
  └─ 停机：(*Cache).Close = InnerCache.Release(true)       [client.go:241]
       ObsCache.Release → obs.close() → reg.Unregister()  （gauge 不再上报）
       标记继续下传 → RawCache.Release(true) → client.Close()
       （停止后台淘汰 goroutine）
```

两个值得知道的顺序事实：

- **销毁顺序是承重的。** `Unregister` 排在最前：cache 一旦关闭，它的统计就没有意义，而遗留的
  注册既会上报一个已死的 cache，又会把它钉在内存里（[client.go:89](client.go#L89)）。
- **⚠ `Close()` 不幂等。** bigcache 内部会关闭一个 channel，`Close()` 调两次会 panic。
  关闭路径只有一条——`Close()`——容器注册的销毁方法也是它。

### 2.2 命令面——走链条，因为 bigcache 没有 hook 点

bigcache（不像 go-redis 或 gorm）不提供插件或拦截器接缝，所以逐调用可观测性只能靠持有 wrapper
本身（[client.go:48-50](client.go#L48-L50)）。这决定了公开类型的形状：

- `Get`/`Set`/`Delete` 被重新实现并观测——它们是承载业务流量的三个操作
  （[client.go:111-135](client.go#L111-L135)）。
- 其余原生方法（`Stats`、`Len`、`Capacity`、`Close`、`KeyMetadata`、`Iterator`）以纯委托
  形式重新导出，有意不观测。
- 原生 `*bigcache.BigCache` 是导出的 `Client` 字段——只读把手，供链构造器使用；重组链条 = 在链头外包一层自定义层，不是绕过链条的暗道。

这些调用下面没有执行器：这里没有任何东西要走出进程，所以没有防护可施加，也没有值得写的访问日志。
因此本 starter 不接治理 bean、也不要 `cloud.ClientParams`——那会是一个背后空无一物的接缝。

### 2.3 一次 `Get("key")` 未命中，逐层走读

```
c.Get(ctx, "key")                                                     [client.go:115]
      → 链头 ObsCache.Get → obs.observe(ctx, "get", fn)  [observe.go:267]
          执行 fn → 下层 RawCache.Get → client.Get("key") → bigcache.ErrEntryNotFound
          statusOf(err) → "ok"                 [observe.go:259]
              未命中不算失败：缓存作出了回答，只是 key 不在
          counter  bigcache.operation.total   {operation="get",status="ok",instance="hot"} += 1
  原样把 bigcache.ErrEntryNotFound 返回给调用方
```

没有 span：trace 拓扑由边缘构成，进程内的微秒级调用不产生任何边缘。key 也**绝不**进指标——
缓存 key 来自开放集合，一旦成为标签，序列数会无界增长
（[observe.go:267-282](observe.go#L267-L282)）。

---

## 3. 逐 key 行为参考

所有 key 位于 `spring.bigcache.instances.<name>.` 下（[config.go:26](config.go#L26)）。

| key | 类型 | 默认值 | 行为与联动 | 配错后果 |
|-----|------|--------|-----------|----------|
| `shards` | int | 1024 | 分片数；bigcache 要求是 2 的幂。 | 非 2 的幂 → 启动时 `bigcache.New` 失败，并指名实例。 |
| `life-window` | duration | 10m | 条目存活多久后成为**陈旧**条目。整个实例一个值，不按条目。内部会截断成整秒，且判定是严格 `>`：超过 `life-window` 个整秒才算陈旧。**陈旧条目照样会被返回**——见下方 ⚠。 | 短 → 频繁 churn；长 → 读到陈旧值。 |
| `clean-window` | duration | 1m | 后台清理器多久移除一次陈旧条目。**决定"何时读不到"的是它，不是 `life-window`**。0 关闭清理 goroutine。 | 0 → 陈旧条目会一直被服务，直到容量淘汰把它挤掉。 |
| `max-entries-in-window` | int | 600000 | 仅启动期预分配提示——无运行时上限。 | 估低 → 启动期 realloc 抖动。 |
| `max-entry-size` | int | 500 | 单条目预分配提示，单位字节。 | 估低 → realloc 抖动。 |
| `hard-max-cache-size` | int | 0 | 内存硬顶，单位 MB；0 = 不限。 | 无必要地设置 → 提前淘汰（丢最旧条目）。 |
| `stats-enabled` | bool | **true** | 记录每 key 命中/未命中/冲突计数，可经 `Stats()`/`KeyMetadata()` 读取，并导出为五个 `bigcache.hits`… gauge。**默认开，与 bigcache 自己的 `DefaultConfig` 不同**——本 starter 的头条 gauge 读的正是它（[config.go:48-54](config.go#L48-L54)）。 | 关 → 那些 gauge 被导出为常量 **zero** 而非缺省：面板上没命中就只说明计数器没开，不代表缓存是冷的。关掉同时也省掉 bigcache 在开启期间维护的每 key 记账，这是关它唯一的理由。 |

**Driver 选择**——`spring.bigcache.instances.<name>.driver`
（[starter.go:51](starter.go#L51)）：留空 = 按类型注入唯一 `Driver` bean，未注册时回退内置
`DefaultDriver`；设值 = 指定该 bean 名，指定的 bean 不存在则启动失败。家族级回退是
`spring.bigcache.default.driver`。

**⚠ `life-window` 不是读侧 TTL。** 它只是把条目标记为陈旧，并不会把它藏起来：`Get` 不看年龄，
一律返回；移除是清理器的职责——每个 `clean-window` 一次。因此在 `clean-window=0` 时，一个值可以
无限期活过它的 `life-window`，直到容量淘汰把它拿走。`example/` 对这个语义的两半都做了断言：同一个
1s 的 `life-window` 配两种清理器设置，一个条目没了、另一个照样返回。

**⚠ 整个实例一个 TTL。** BigCache 按单一全局 `life-window` 过期，没有按条目的 TTL。经缓存抽象
访问时，`SetBytes` 的 `ttlSeconds` 参数被**有意忽略**
（[bytecache.go:55-60](bytecache.go#L55-L60)）——签名来自共享的 `cache.ByteCache` 契约，而 BigCache
没有任何东西可以兑现它。需要多个 TTL 档 → 定义多个命名实例。

### 3.1 beans

| bean | 类型 | 名字 | 注入方式 |
|------|------|------|---------|
| client | `*StarterBigCache.Cache` | `<name>` | `autowire:"<name>"` |
| 缓存抽象 | `*cache.Cache` | `bigcache:<name>` | `autowire:"bigcache:<name>"` |

`*cache.Cache` bean 是惰性的：无人注入则不实例化，因此没有配置开关
（[starter.go:61-65](starter.go#L61-L65)）。在该边界上 `bigcache.ErrEntryNotFound` 映射为
`cache.ErrMiss`，且删除不存在的 key 不算错误（[bytecache.go:44-69](bytecache.go#L44-L69)）。

### 3.2 扩展点

客户端装配由 `Driver` 接口负责（[driver.go:68](driver.go#L68)）：

```go
type Driver interface {
	CreateClient(ctx context.Context, name string, c Config) (*Cache, error)
}
```

把它作为**可选容器 bean** 提供——其构造函数可在装配期注入从配置文件绑定的配置：

```go
gs.Provide(func(c *MyConf) StarterBigCache.Driver { return myDriver{c} })
```

这是触达 starter 未绑定的 `bigcache.Config` 字段（例如 `OnRemove`）的唯一途径。
[example/driver.go](example/driver.go) 注册了一个这样的 Driver bean，名为 `hook`，由
`spring.bigcache.default.driver=hook` 选中；它自己拥有整套装配并挂上 `OnRemove`，这也是
`example/` 能断言"钩子确实被触发"的原因。

Driver **自行归因构造失败**：它返回的错误会不经包装地到达容器，因此应当指名阶段与实例，例如
`errutil.Explain(err, "bigcache: create instance %q", name)`——范例见
[DefaultDriver](driver.go#L81)。

---

## 4. 验证与故障演练

### 4.1 观测逐调用信号与统计 gauge

[example-otel/](example-otel/) 接上 starter-otel 的进程内 Prometheus 拉取导出器（并关掉 gs 默认的
`:9090` HTTP server，让导出器独占该端口）：

```properties
spring.http.server.enabled=false
spring.observability.enable=true
spring.observability.metrics.exporter=prometheus
spring.observability.metrics.port=9090
spring.observability.metrics.path=/metrics
spring.observability.trace.exporter=none
```

```bash
cd example-otel && go run . -manual
# 制造流量，然后：
curl -s :9090/metrics | grep 'bigcache_'
# bigcache_operation_total{operation="get",status="ok",instance="hot"} 计数调用次数；
# bigcache_hits{instance="hot"} 仅在 stats-enabled=true 时增长
```

信号读法：计数器带 `operation=<get|set|delete>`、`status=<ok|error>`、
`instance=<name>`。未命中计入 `status="ok"`——命中率是 gauge 的职责，不在 status 轴上。gauge
在抓取时拉取（无逐调用开销），且是**按进程**而非按调用。meter scope 为
`go-spring.org/starter-bigcache`（[observe.go:49](observe.go#L49)）。

注意 `bigcache_operation_total` 只在**首个**同类操作之后出现：gauge 是可观测仪器
（始终上报），而计数器是事件（发生后才上报）。刚启动就 `grep` 不到并不代表接线坏了
——先打一次 `Get`。`example-otel` 也断言了这些序列，包括未命中计入 `status="ok"`、以及缓存 key
从不成为标签。

### 4.2 淘汰演练（example 的 `evict` 实例）

`hard-max-cache-size=1`（1 MB）配 `max-entry-size=1024`，写超上限即淘汰最旧条目。可观测形状：
`bigcache_entries{instance="evict"}` 在上限处走平，同时被淘汰 key 的 `bigcache_misses` 上升。
`example/` 从缓存侧断言它：常驻条目数少于写入数，且 `OnRemove` 钩子被触发过。

### 4.3 缓存抽象接线，以及被忽略的 TTL

```go
_ = s.Cache.Set(ctx, "k", "v", 1)       // 经 *cache.Cache 抽象；1 = ttlSeconds
var v string
err := s.Cache.Get(ctx, "k", &v)        // JSON 解码进指针
if errors.Is(err, cache.ErrMiss) { /* 不存在 */ }
```

抽象用 JSON 序列化，未命中报 `cache.ErrMiss`；wrapper 报 `bigcache.ErrEntryNotFound`。注意 TTL
参数是**整数秒**（`ttlSeconds int`），不是 `time.Duration`。

要看到被忽略的 TTL：以 `ttlSeconds=1` 写入，睡过 1 秒再读——值仍在，因为过期只跟实例的
`life-window` 走（[bytecache.go:58](bytecache.go#L58)）。这一点由 `example/main.go` 的
Feature 5 断言，适配器层由 `bytecache_test.go` 覆盖。

### 4.4 启动失败演练：拒绝的 meter

仪器集每进程只构造一次，在第一个 cache 构造时。若自定义 `MeterProvider` 拒绝某个仪器，启动会
失败，并指名阶段，而不是从库里 panic 出来：

```
wire bean hot(<注册处 file:line>), err constructor returned error: bigcache: create gauge "bigcache.hits": <sdk error>
```

前一刻建好的原生 cache 会在退出路径上被关闭，因此构造失败不会留下淘汰 goroutine
（[driver.go:96-102](driver.go#L96-L102)）。注意这是 `NewCache` 失败的**唯一**途径——被调用时原生
cache 已经打开且健康。

### 4.5 span：没有，是有意的

wrapper 不开 span。trace 的价值在于边缘，而进程内的微秒级调用没有边缘——计数器（§2.3）与
gauge（§4.2）就是全部信号。如果某次缓存调用确实需要出现在 trace 里，由掌握业务上下文的调用方
去记录。

---

## 5. 排障表

| 症状 | 最可能的原因 | 处置 |
|------|--------------|------|
| 启动失败：`bigcache: create instance "<name>": ...` | 原生缓存建不起来——最常见是 `shards` 不是 2 的幂 | 修正该实例的取值。 |
| 启动失败：`bigcache: create gauge "<metric>": ...` | 自定义 `MeterProvider` 拒绝了某个仪器（进程级） | 检查 OTel 配置；消息里点名的就是被拒的那个仪器。 |
| gauge 全零而 `Len()` 正常 | `stats-enabled` 被设成了 `false` | 对需要观察的实例改回 `true`。 |
| 一次停机/重启循环后 gauge 消失 | `Close` 注销了它们并关闭了 cache | 预期行为——注册的生命周期就是 cache 的生命周期。 |
| panic："close of closed channel" | `Close()` 调了两次 | 只调一次 `Close()`；容器的销毁注册就是它。 |
| 值被截断 / realloc 抖动 | `max-entry-size` 估低 | 它是预分配提示——按真实条目大小设。 |
| 条目提前消失 | `hard-max-cache-size` 硬顶在淘汰最旧条目 | 调高或取消硬顶。 |
| 超过 `life-window` 的值仍被返回 | `life-window` 不会藏起条目，而没有东西把它移除——`clean-window=0` 时永远不会 | 设一个非零 `clean-window`，或把这个实例当作没有读侧 TTL。 |
| 传给 `Set` 的 TTL 过了还读到旧值 | `life-window` 是按实例的；逐调用 TTL 被忽略 | 按 `life-window` 设；或改用多个实例。 |
| 日志里没有逐调用记录 | 本来就没有——bigcache 不写访问日志，本 starter 也不发射 | 改读指标。 |
| 两个副本数据不一致 | 条目在各进程堆内；副本之间没有任何失效同步 | 预期行为——需要一致性请用网络后端。 |

---

## 6. 设计体检表

| 指标 | 数值 |
|------|------|
| 配置 key 总数 | 每实例 7 个（+7 个从 `spring.bigcache.default.*` 继承）+ 1 个 driver key |
| 其中必填 | 0 |
| quickstart 前置外部依赖数 | 0 |
| 文档中"注意/坑"条数 | 4 |

一处有意偏离组件默认：`stats-enabled` 出厂为**开**，而 bigcache 自己的 `DefaultConfig` 是关。
理由见 §3——不开的话本 starter 的头条 gauge 就是静默的零，一个把自家指标抹掉的默认值，比一个
多花一张每 key 映射表的值更糟。在记账成本压过计数价值的地方逐实例关掉它即可。

设计嫌疑清单：

- 缓存抽象的 `ttlSeconds` 参数被接受后丢弃（[bytecache.go:58](bytecache.go#L58)）；共享接口无法
  表达 BigCache 的实例级 TTL。现在有三处在陈述它（适配器、README、本文 §3/§5）——风险不是读者
  看不到，而是这三处会各自漂移。

已移除：每实例 `health` 开关及其常量 UP 的 indicator——进程内堆缓存没有可探测的连通性，该
indicator 不可能失败，而一个常量 UP 项放进 AND 聚合的就绪判定里不携带任何信号。bigcache 是客户端
starter"health 默认开"规则的既有例外；它不贡献 `health.Indicator`，也没有 `ping` 探针。
