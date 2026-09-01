# starter-cache 使用说明 — 参考手册

详细使用文档。模块全景见 [starter/README](../README.md)。所有行为声明均已对源码核对：本 starter
源码（`starter.go`、`starter_test.go`）、缓存抽象（`cloud/cache/cache.go`、`codec.go`）与四个
后端 driver 注册（`starter-go-redis/starter.go`、`starter-redigo/starter.go`、
`starter-bigcache/starter.go`、`starter-memcached/starter.go`）。**缓存语义本身（miss 与错误的
区分、codec、TTL）见 `cloud/cache` 文档** —— 下文均为 go-spring 的接线增量：driver 注册表、
`${spring.cache}` 模块与其暴露的 bean。

**激活条件**：任一 `spring.cache.*` key —— 模块为 `gs.OnProperty("spring.cache")` 前缀匹配
（`starter.go:42`）。starter 本身是惰性的：只引入 `starter-cache` 不注册任何 driver；引入后端
starter（go-redis / redigo / bigcache / memcached）才会在其 `init()` 里注册 driver（如
`starter-go-redis/starter.go:81`）。`driver` 指向未注册后端时启动即报错，并列出已排序的注册名
（`starter.go:101-113`）。

**尚无独立 example** —— 下文工程镜像已验证的后端 example（`starter-go-redis/example`、
`starter-bigcache/example`，均由各自 `check.sh` 跑通）与本模块 `starter_test.go`；尚未以独立
`spring.cache` example 冒烟（见 §6 嫌疑 1）。

---

## 1. 完整工程示例

一个通过 façade 暴露两级缓存的服务：Redis 共享缓存 + 进程内 bigcache 热点层。文件树：

```
demo/
├── go.mod
├── main.go
├── service.go
└── conf/
    └── app.properties
```

**前置依赖**（仅 1 个外部系统 Redis；bigcache 为进程内）：

```bash
docker run -d --name redis -p 127.0.0.1:6379:6379 redis:7
```

**go.mod**（关键依赖）：

```
require (
    go-spring.org/spring           v1.3.x
    go-spring.org/starter-cache    latest
    go-spring.org/starter-go-redis latest   // 注册 "go-redis" driver
    go-spring.org/starter-bigcache latest   // 注册 "bigcache" driver
)
```

**main.go**：

```go
package main

import (
    _ "demo/conf"
    "go-spring.org/spring/gs"
    _ "go-spring.org/starter-bigcache"
    _ "go-spring.org/starter-cache"
    _ "go-spring.org/starter-go-redis"
)

func main() { gs.Run() }
```

**service.go** —— 注入 façade bean，每个缓存暴露一对 Set/Get：

```go
package conf

import (
    "errors"
    "net/http"
    "time"

    "go-spring.org/cloud/cache"
    "go-spring.org/spring/gs"
)

type Service struct {
    // ⚠ bean 名是后端实例名（beanID），不是 spring.cache 条目名 —— 见 §2.2。
    // 此处条目 "shared" 用 driver go-redis:shared，*cache.Cache bean 叫 "shared"
    // 只是因为我们刻意同名；若 driver 写 go-redis:main，bean 就叫 "main"。
    Shared *cache.Cache `autowire:"shared"`
    Hot    *cache.Cache `autowire:"hot"`
}

func init() {
    b := gs.Provide(&Service{}).Export(gs.As[gs.Rooter]()) // root：无其他注入方

    http.HandleFunc("/set", func(w http.ResponseWriter, r *http.Request) {
        s := b.Interface().(*Service)
        key, val := r.URL.Query().Get("k"), r.URL.Query().Get("v")
        // 类型化 Set：JSON codec，60s TTL。
        if err := s.Shared.Set(r.Context(), key, val, 60*time.Second); err != nil {
            http.Error(w, err.Error(), 500)
            return
        }
        _ = s.Hot.Set(r.Context(), key, val, 30*time.Second)
        _, _ = w.Write([]byte("ok"))
    })
    http.HandleFunc("/get", func(w http.ResponseWriter, r *http.Request) {
        s := b.Interface().(*Service)
        var v string
        if err := s.Shared.Get(r.Context(), r.URL.Query().Get("k"), &v); err != nil {
            if errors.Is(err, cache.ErrMiss) {
                http.Error(w, "miss", 404)
                return
            }
            http.Error(w, err.Error(), 500)
            return
        }
        _, _ = w.Write([]byte(v))
    })
}
```

**conf/app.properties**（后端块逐字取自 `starter-go-redis/example/conf/app.properties` 与
`starter-bigcache/example/conf/app.properties`，各裁剪为单实例）：

```properties
# --- 后端：go-redis 实例 "shared" -------------------------------------------
spring.go-redis.shared.addr=127.0.0.1:6379

# --- 后端：bigcache 实例 "hot" ----------------------------------------------
spring.bigcache.hot.life-window=1m
spring.bigcache.hot.shards=256
spring.bigcache.hot.stats-enabled=true

# --- cache façade 条目 -------------------------------------------------------
# driver 格式 "<driver>:<beanID>" —— beanID 指向要包装的后端 bean。
# 暴露的 *cache.Cache bean 名为 "shared"/"hot"（即 beanID），不是条目名；
# 此处条目名 == beanID 是刻意选择。
spring.cache.shared.driver=go-redis:shared
spring.cache.hot.driver=bigcache:hot
```

**验证**（与后端 example 的 manual 模式同构）：

```bash
curl -i 'localhost:8080/set?k=user:1&v=alice'          # ok
curl -i 'localhost:8080/get?k=user:1'                  # alice
curl -i 'localhost:8080/get?k=user:2'                  # 404 miss
redis-cli -h 127.0.0.1 GET 'user:1'                    # "\"alice\"" —— JSON codec，共享存储
sleep 61 && curl -i 'localhost:8080/get?k=user:1'      # 404（60s TTL 到期）
```

`redis-cli` 往返证明值经应用落入了后端 Redis 的裸 key（façade 不加任何前缀 —— key 的命名空间
划分是调用方的职责）。

---

## 2. 装配与时序

### 2.1 生命周期时间线

```
import starter-go-redis / starter-bigcache / ...
  └─ 后端 init()：StarterCache.RegisterDriver("<name>", driver)     [重复注册 init 即 panic]
import starter-cache
  └─ init()：注册 gs.Module(gs.OnProperty("spring.cache"), ...)     (starter.go:41-64)
gs.Run()
  ├─ 模块触发（存在任一 spring.cache.* key）：
  │    conf.Bind(p, &map[string]{Driver}, "${spring.cache}")        (starter.go:46)
  │    逐条目遍历（map 顺序不确定，且顺序无关）：
  │      strings.Cut(driver, ":") → (driverName, beanID)            (starter.go:50)
  │      GetDriver(driverName) —— 未注册则报错并列出已注册名        (starter.go:54,101-113)
  │      d(beanID)(r, p) → 后端的 ModuleFunc：
  │        r.Provide(func(c *Client) *cache.Cache {
  │            return cache.New(bytecache.NewByteCache(...))
  │        }, gs.TagArg(beanID)).Name(beanID)                       (starter-go-redis/starter.go:82-85)
  ├─ bean 装配：cache 构造函数按名（TagArg）注入后端 wrapper；
  │    后端 bean 的 Init 已先武装 resilience/observe
  ├─ Run：无 —— 不开端口、无自身就绪信号
  └─ 停机：无 —— cache.Cache 没有 Close；由后端 bean 的
       Destroy（go-redis Client.Destroy、bigcache Cache.Destroy）负责释放资源
```

多个缓存条目各自产出一个 `*cache.Cache` bean。校验：`driver` 格式非法（缺 `:`、某半为空）使模块
失败并报 `cache: invalid driver %q (want "<driver>:<beanID>")`（`starter.go:51-53`）；未知 driver 名
报错并列出已注册名（`starter.go:112`）。

### 2.2 driver 解析 —— `<driver>:<beanID>` 的确切机制

`spring.cache.users.driver=go-redis:main` 分三步解析：

1. **前缀**：`"go-redis"` 选中注册表条目 —— 即运行哪个后端 starter 的适配器。注册表在各后端
   starter 的 `init()` 期填充（`starter-go-redis/starter.go:81`、`starter-redigo/starter.go:85`、
   `starter-bigcache/starter.go:69`、`starter-memcached/starter.go:67`）。
2. **beanID**：`"main"` 被传入 driver，在其 ModuleFunc 里用两次 —— 一次作
   `gs.TagArg(beanID)`（按名注入名为 `main` 的后端 wrapper bean，即 `spring.go-redis.main.*`
   实例），一次作新 `*cache.Cache` bean 的 `.Name(beanID)`。
3. **条目名被丢弃**：map key `users` 只是配置里的分组槽位，从不会进入 bean 名（绑定目标是
   `map[string]struct{Driver}`，`starter.go:43-45`）。

⚠ **后果（§6 嫌疑 2）**：暴露的 bean 以**后端 bean** 命名，而非缓存条目。两个条目包装同一后端
—— `users.driver=go-redis:main` 与 `sessions.driver=go-redis:main` —— 都会试图 provide 名为
`main` 的 `*cache.Cache`：装配期 duplicate bean 报错。反过来 `users.driver=go-redis:users` 产出的
`users` bean 与后端自身 wrapper 同名也仅是 (Name,Type) 键的碰撞 —— 类型不同（`*Client` vs
`*cache.Cache`），可共存。按 beanID 注入 `*cache.Cache`。

### 2.3 一次 Set/Get 调用走读

`Cache.Set(ctx, "user:1", "alice", 60s)`（类型化）→ `codecOr().Marshal(val)`（默认 JSON，
`cache.go:123-129`）→ 内嵌 `ByteCache` 的 `SetBytes` → 后端适配器（go-redis：
`bytecache.NewByteCache(c.UniversalClient)`，`starter-go-redis/starter.go:83`）下发原生
`SET key val PX 60000`。`Get` 为镜像：`GetBytes` → redis Nil 应答返回 `(nil, cache.ErrMiss)`
（`starter-go-redis/bytecache/bytecache.go:40-45`）→ `codecOr().Unmarshal` 进指针
（`cache.go:113-119`）。非正 TTL 表示不过期（`cache.go:57-59`）。

### 2.4 四个 driver

| driver 名 | 注册方 | 包装 bean 类型 | 后端前缀 | 示例条目 |
|---|---|---|---|---|
| `go-redis` | `starter-go-redis`（`starter.go:81`） | `*StarterGoRedis.Client`（single/sentinel/cluster 全模式） | `spring.go-redis.<id>` | `spring.cache.c.driver=go-redis:c` + `spring.go-redis.c.addr=...` |
| `redigo` | `starter-redigo`（`starter.go:85`） | `*StarterRedigo.Pool` | `spring.redigo.<id>` | `spring.cache.c.driver=redigo:c` + `spring.redigo.c.addr=...` |
| `bigcache` | `starter-bigcache`（`starter.go:69`） | `*StarterBigCache.Cache`（内嵌 `*bigcache.BigCache`） | `spring.bigcache.<id>` | `spring.cache.hot.driver=bigcache:hot` + `spring.bigcache.hot.life-window=1m` |
| `memcached` | `starter-memcached`（`starter.go:67`） | `*StarterMemcached.Client`（内嵌 `*memcache.Client`） | `spring.memcached.<id>` | `spring.cache.c.driver=memcached:c` + `spring.memcached.c.servers=127.0.0.1:11211` |

四个适配器都以 `cache.New(bytecache.NewByteCache(<client>))` 构造、不传 `WithCodec` —— façade
codec 恒为默认 JSON；`WithCodec` 只能以程序化构造 `cache.Cache` 的方式触达，配置不可达。

---

## 3. 逐 key 行为参考

façade 自身每个缓存条目仅 1 个 key；工程示例的后端实例贡献其余。已用
`grep -rhoE 'value:"[^"]+"' <starter dirs> --include='*.go' | sort -u` 核对。

### 3.1 façade（`spring.cache.<name>.*`，`starter.go:43-45`）

| Key | 类型 | 默认值 | 行为 / 联动 | 配错后果 |
|-----|------|--------|------------|----------|
| `spring.cache.<name>.driver` | string | —（必填） | 格式 `"<driver>:<beanID>"`。driver 前缀选后端适配器；beanID 选后端实例**并**成为 `*cache.Cache` 的 bean 名（§2.2）。任一 `spring.cache.*` key 激活模块。 | 缺 `:` 或某半为空 → 启动报 `cache: invalid driver`（`starter.go:51-53`）。未知 driver → 报错并列出已注册名（`starter.go:112`）。beanID 无对应后端 bean → 装配失败（autowire 不可解析）。 |

⚠ `<name>` 部分是纯分组槽位：不校验任何东西，也不命名任何东西。

### 3.2 工程示例引用的后端实例 key

**`spring.go-redis.shared.*`**（全部 key 来自 `starter-go-redis/config.go:35-139`；完整语义见其 USAGE）：

| Key | 类型 | 默认值 | 说明 |
|-----|------|--------|------|
| `addr` | string | `` | `host:port`；未设 `service-name` 时必填 |
| `mode` | string | `single` | `single`/`sentinel`/`cluster`；sentinel 需 `master-name`+`sentinel-addrs`，cluster 需 `addrs` |
| `password` / `username` / `db` | string/string/int | ``/``/0 | 认证与逻辑库 |
| `pool-size` / `max-idle` / `max-retries` | int | 10/5/0 | 连接池与重试 |
| `dial-timeout` / `read-timeout` / `write-timeout` / `conn-max-lifetime` | duration | 5s/3s/3s/2m | 时间预算 |
| `driver` | string | `DefaultDriver` | 后端内部客户端工厂，与 cache driver 无关 |
| `tracing.enabled` / `metrics.enabled` | bool | true/true | redisotel 钩子（无 starter-otel 时为 no-op） |

**`spring.bigcache.hot.*`**（`starter-bigcache/config.go:28-52`；全量）：

| Key | 类型 | 默认值 | 说明 |
|-----|------|--------|------|
| `shards` | int | 1024 | 分片数 |
| `life-window` | duration | 10m | 条目生命周期（配合 §1 的 TTL 测试） |
| `clean-window` | duration | 1m | 淘汰扫描间隔 |
| `max-entries-in-window` / `max-entry-size` / `hard-max-cache-size` | int | 600000/500/0 | 容量约束 |
| `stats-enabled` | bool | false | 暴露统计给 health/metrics 路径 |
| `driver` | string | `DefaultDriver` | 后端内部工厂 |

（redigo / memcached 的实例 key —— `spring.redigo.<id>.*`、`spring.memcached.<id>.*` —— 同构；
见 `starter-redigo/config.go:29-106`、`starter-memcached/config.go:28-61`。工程示例未用，不展开。）

---

## 4. 验证与故障演练

1. **经应用往返**：§1 的 curl SET/GET 加 `redis-cli GET user:1`，裸 key 下看到 JSON 载荷即证明
   落入后端存储。
2. **miss 与后端宕机的区分**：GET 不存在的 key → 经 `errors.Is(err, cache.ErrMiss)` 走 HTTP 404
   （`cache.go:134`）；停掉 redis（`docker stop redis`）再 GET 已存在的 key → HTTP 500（后端错误
   路径，不是 miss）。这正是该抽象存在的意义。
3. **bean 名不匹配演练**：把条目改为 `spring.cache.users.driver=go-redis:main` 且
   `autowire:"users"` → 容器失败：没有名为 `users` 的 `*cache.Cache` bean（bean 是 `main`，
   §2.2）。改 tag 为 `main` 或改 beanID 为 `users`。
4. **重复包装演练**：在 `users.driver=go-redis:main` 之外再加
   `spring.cache.sessions.driver=go-redis:main` → `main` 的 `*cache.Cache` 重复 bean，装配失败。
   每个后端实例一个 façade bean。
5. **未知 driver 演练**：`spring.cache.x.driver=redis:main`（笔误，漏 `go-`）→ 启动报
   `cache: no driver registered as "redis" (registered: [bigcache go-redis ...])`
   （`starter.go:112`）—— 修法（import starter 或改前缀）直接写在报错里。
6. **格式非法演练**：`spring.cache.x.driver=go-redis`（无 `:beanID`）→ `cache: invalid driver
   "go-redis" (want "<driver>:<beanID>", e.g. "go-redis:main")`（`starter.go:52`）。
7. **多 driver 共存**：工程示例本身 —— 一个文件里 go-redis 与 bigcache 条目并存，两个 bean 可同
   时注入；各后端观测相互独立（`redis:shared` / `bigcache:hot` health indicator，
   `starter-go-redis/starter.go:58`、`starter-bigcache/starter.go:60`）。
8. **TTL 语义**：§1 的 `sleep 61` —— 正 ttl 的 Set 会过期；`SetBytes` 的 ttl `<= 0` 永不过期
   （`cache.go:57-59`）。

---

## 5. 排障表

| 症状 | 可能原因 | 处置 |
|------|---------|------|
| `cache: no driver registered as "x" (registered: [...])` | 未 import 后端 starter，或 driver 前缀笔误 | import `starter-go-redis`/`starter-redigo`/`starter-bigcache`/`starter-memcached`，或改前缀（`starter.go:101-113`） |
| `cache: invalid driver ... want "<driver>:<beanID>"` | 缺 `:` 或某半为空 | 写成 `go-redis:main` 形式（`starter.go:51-53`） |
| 容器失败：没有名为 `users` 的 `*cache.Cache` bean | bean 以 beanID 而非条目名命名（§2.2） | 对齐 `driver` 的 beanID 与 autowire tag |
| 容器失败：duplicate bean | 两个缓存条目包装同一后端实例 | 每后端实例一个条目，或再建一个后端实例 |
| 容器失败：cache 构造参数不可解析 | beanID 无对应后端实例（缺 `spring.go-redis.<beanID>`） | 补后端条目或改 beanID |
| 装配正常但 `Get` 返回 500 | 后端宕机 / 认证错误 —— 以后端错误而非 miss 呈现 | 查后端 starter 的 health indicator 与其排障表 |
| 取回的值带 JSON 引号（`"alice"` 而非 `alice`） | façade codec 固定 JSON（`cache.go:82-88`） | 反序列化进类型化值，或调用侧用 `GetBytes/SetBytes` 配自己的 codec |
| 设了 ttl 却像永不过期 | 传入的 ttl 非正（`<=0` 即不过期，`cache.go:57-59`） | 传正的 duration |

---

## 6. 设计体检表

| 指标 | 数值 |
|------|------|
| 配置 key（façade） | 每条目 1 个（`driver`） |
| 其中必填 | 1 |
| quickstart 前置外部依赖 | 取决于后端（本示例 1 个 —— Redis；bigcache 0 个） |
| "注意/坑" 条数 | 3 |

设计嫌疑清单（保留既有条目 + 本轮新增）：

1. **无独立 example** 覆盖 `spring.cache` —— 本文档的工程由已验证后端 example 拼装，但未作为整
   体冒烟；建议在现有后端 example 里加一段 `spring.cache.*`。（既有）
2. **cache bean 以后端 beanID 而非缓存条目名命名** —— `starter.go:58` 只把 `beanID` 传给
   driver，而各后端均 `.Name(beanID)`（`starter-go-redis/starter.go:85` 等）；条目名被校验出存在
   后即丢弃。两者不同时易意外；"一个后端之上按关注点建多个 façade 别名" 不可行（duplicate
   bean）；且 `Driver` 类型签名 `func(beanID string) gs.ModuleFunc` 不破坏式改动就无法携带条目
   名。候选：bean 以条目名命名，beanID 仅用于查找。（既有，本轮已钉到 file:line）
3. **模块无 README**（只有 USAGE）；driver 注册表契约只存在于代码注释。（既有）
4. 本轮新增：façade codec 在 starter 路径上硬连 JSON —— `cloud/cache` 有 `WithCodec`，但
   没有 driver 传选项，也没有配置面。
5. 本轮新增：模块遍历缓存条目 map 时不查重同一 `(driver, beanID)` —— 失败形态是晚期的
   duplicate-bean 装配错误，而非早期点名条目的 `invalid driver` 式报错。
