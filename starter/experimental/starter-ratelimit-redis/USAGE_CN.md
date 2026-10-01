# starter-ratelimit-redis 使用说明 — 参考手册

[English](USAGE.md) | [中文](USAGE_CN.md)

详尽使用参考。概览见 [README_CN.md](README_CN.md)。下文所有行为声明均对照 starter 源码
（`starter.go`、`config.go`）、`starter-go-redis/experimental/ratelimit.go` 的计数器实现、
把存储注入 driver bean 的 [governance 装配](../../starter-governance-file/wiring.go) 以及
自校验的 [example/](example)（`example/check.sh`）核实。限流语义本身记在
`cloud/resilience`——本文只写本 starter 的增量。

**激活方式**：任一 `spring.ratelimit.redis.*` 键即武装 starter 的 module，且该配置块唯一的键
必须设置：`spring.ratelimit.redis.client` 指名的 `*goredis.Client` bean（由 starter-go-redis
在 `spring.go-redis.instances.<client>` 下提供）所连的 Redis 实例承载计数器。starter 只贡献
一个 `resilience.Counters` 类型的 bean。容器里没有它时，每个 executor 用自己的一份预算计数，
所以贡献本存储就是全部的开关：[starter-governance-file](../../starter-governance-file) 的 driver bean
会注入它，进程内每个 executor 从此花 Redis 里每个 scope 的同一个预算——而且是跨副本的，这是
executor 本地预算做不到的。

---

## 1. 完整工程示例

两个 HTTP"副本"共享 Redis 中的一个全局令牌预算——跨副本限流的演练场。文件树（即仓内示例，
原文照录）：

```
demo/
├── go.mod
├── main.go
└── conf/
    ├── app.properties
    └── governance.yaml
```

**go.mod**（关键依赖）：

```
require (
    go-spring.org/spring                   v1.3.x
    go-spring.org/starter-go-redis         latest
    go-spring.org/starter-ratelimit-redis  latest
)
```

**main.go**：

```go
package main

import (
    "go-spring.org/spring/gs"
    _ "go-spring.org/starter-go-redis"
    _ "go-spring.org/starter-ratelimit-redis"
)

func main() { gs.Run() }
```

**web.go** —— 应用的全部限流面。预算来自治理规则里的 `resilience.ClientPolicy`，调用走 executor：

```go
package main

import (
    "context"
    "errors"
    "net/http"

    "go-spring.org/cloud/resilience"
    "go-spring.org/spring/gs"
)

// 两个 handler 共享的 service label；conf/governance.yaml 里的规则以它为键，
// 它也是预算按之保存的 scope。
const service = "ratelimit-redis:api"

func init() {
    gs.Provide(func(mgr *resilience.Manager) *gs.HttpServeMux {
        mux := http.NewServeMux()
        // 两个 handler 模拟两个副本：不共享任何进程内状态。两者都花本进程
        // 贡献的同一个 Redis 计数器存储。
        mux.Handle("/a/", serve(mgr.ClientExecutorFor("ratelimit-redis", service)))
        mux.Handle("/b/", serve(mgr.ClientExecutorFor("ratelimit-redis", service)))
        return &gs.HttpServeMux{Handler: mux}
    })
}

func serve(exec resilience.ClientExecutor) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        err := exec.Execute(r.Context(), func(context.Context) error { return nil })
        switch {
        case err == nil:
            _, _ = w.Write([]byte("ok"))
        case errors.Is(err, resilience.ErrRateLimited):
            http.Error(w, "429 Too Many Requests", http.StatusTooManyRequests)
        default:
            http.Error(w, "executor error: "+err.Error(), http.StatusInternalServerError)
        }
    }
}
```

**conf/app.properties** —— 上述用到的全部配置面，含注释：

```properties
# --- redis 客户端（归 starter-go-redis；计数器通过它计数）-------------------
spring.go-redis.instances.cache.addr=127.0.0.1:6379

# --- 本 starter 唯一的键 ------------------------------------------------------
# 哪个 *goredis.Client bean 提供共享计数器。必填。
spring.ratelimit.redis.client=cache

# --- 治理规则（独立文件，独立刷新通道）---------------------------------------
spring.governance.source.file.path=conf/governance.yaml
```

**conf/governance.yaml** —— 预算本身，是 policy，不是 starter 配置：

```yaml
spring:
  governance:
    enabled: true
    client:
      rules:
        - service: ratelimit-redis:api
          rate-limit: 2      # 持续 2/s
          burst: 5           # 瞬时额度
```

规则对被匹配的 service 完整取代 `spring.governance.client.default`（不做逐字段合并），所以规则要带上该 service
需要的全部参数。

**验证**（本地 Redis，例如 `example/docker-compose.yml`）：

```bash
go run . &
for i in $(seq 1 10); do curl -s -o/dev/null -w '%{http_code}\n' localhost:9090/a/; done
# 前 5 个（burst）→ 200，其余 → 429；/b/ 花的是同一个预算
```

可运行的 [example/](example) 断言共享预算与补充；`example/check.sh` 用 docker compose 包起来。

---

## 2. 装配与时序

### 2.1 bean 生命周期

```
import starter-go-redis + starter-ratelimit-redis
  └─ gs.Module(gs.OnProperty("spring.ratelimit.redis"))
        └─ conf.Bind("${spring.ratelimit.redis}") → Config{Client}
             ├─ Client == "" 时 fail fast（启动报错并点名该属性）
             └─ Provide func(client *goredis.Client) (resilience.Counters, error)
                  （gs.TagArg(c.Client)；接口返回类型即 bean 类型——
                   不需要 Export）

gs.Run()
  ├─ 本存储成为容器里唯一的 resilience.Counters bean
  ├─ bean 装配：client 指名的 *goredis.Client bean 按名注入；
  │     NewCounters 包住它的 UniversalClient（client 为 nil 是构造函数错误）
  ├─ resilience driver bean 注入 resilience.Counters，所以它构建的每个 executor
  │     都花这份存储——存储本身也因此被实例化（driver 需要它）
  └─ SIGTERM 时：无需释放——存储自身不持有状态，redis 客户端的 Close
                 归 starter-go-redis
```

### 2.2 一次 Execute 调用，逐层

`exec.Execute(ctx, fn)` 配规则 `rate-limit: 2, burst: 5`：

1. executor 的限流阶段调用 `Counters.Allow(ctx, service, policy, 1)`——executor 对每次尝试都
   计费，scope 即该 executor 构建时绑定的 service。
2. `RateLimit` 为零直接放行为无限流，**不碰 Redis**。规则丢失意味着完全没有限流：这是治理
   配置的症状，不是存储的症状。
3. 原子 Lua 脚本完全在 Redis 内跑，键为 `ratelimit:<scope>`（`tokens` + `ts` 的 hash）：按
   `elapsed × rate` 补充并以 `burst` 封顶、够则扣减、`HSET` 新状态、`EXPIRE` 到
   `ceil(burst/rate)+1` 秒（空闲键自删——被弃用的预算不会泄漏）。
4. 脚本回答 1 或 0。因为状态在 Redis，所有花同一个键的副本共享一个预算——本 starter 的全部
   意义所在。
5. 0 以 `resilience.ErrRateLimited` 返回；调用方回 429（示例与 gateway 都如此）。
6. `Burst <= 0` 在存储内部缺省为 `max(1, int(RateLimit))`。
7. Redis 故障时 `Allow` 返回错误：executor 记录后放行，所以计数器后端坏掉退化为"不限流"，
   而不是一次故障。

### 2.3 存储不做的事

- 不实现 sliding window：`algorithm: sliding-window` 的 policy 按令牌桶计数。共享预算的正确性
  来自那条原子脚本，而窗口无法廉价地套进一条脚本——计数器共享时请用令牌桶。
- 不排队：`rate-limit-max-wait` 被忽略，超限的单位立即被拒绝。等一个令牌意味着轮询 Redis；
  排队留给内存存储。
- 自身不记指标——被限流的调用体现在 executor 观测层的 `resilience.outcome=rate_limited` 上。
- 不裁决：拒绝超限调用、计数器出错时放行还是拒绝，都是 executor 的决定。

---

## 3. 配置参考

所有键都在 `spring.ratelimit.redis` 下（精确匹配，无宽松形式）。配置块存在即武装本 starter；
其下不应再有别的键。

| 键 | 类型 | 缺省 | 行为 / 交互 | 配错的后果 |
|----|------|------|-------------|------------|
| `client` | string | — | **必填。** `spring.go-redis.instances.<client>` 下 `*goredis.Client` bean 的名字；注册 bean 前先校验，构造时以 `TagArg(c.Client)` 注入。 | 为空 → 启动失败并点名该属性；名字没有对应 bean → 启动期 bean 装配失败。 |

⚠ 限流参数（`rate-limit`、`burst`、`algorithm`、`window`、`rate-limit-max-wait`）**不是**
starter 配置：它们是治理规则文档上按 service 生效的 `resilience.ClientPolicy` 字段。

---

## 4. 验证与故障演练

### 4.1 "副本"之间共享预算

```bash
for i in $(seq 1 10); do
  p=/a/; [ $((i%2)) -eq 1 ] || p=/b/
  curl -s -o/dev/null -w "%{http_code} " localhost:9090$p
done; echo   # 恰好 burst 个 200 交替落在 /a/ 与 /b/，其余 429
```

Redis 中的状态：

```bash
redis-cli hgetall ratelimit:ratelimit-redis:api   # tokens + ts
redis-cli ttl ratelimit:ratelimit-redis:api       # ceil(burst/rate)+1
```

### 4.2 突发与补充演练

配 `rate-limit: 2, burst: 5`：先抽干预算（5×200），再等约 2.2 秒确认连续补充（`curl` 循环 →
至少 `rate × 秒数` 个新的 200）。示例正是自动做这件事。

### 4.3 Redis 故障演练

停掉 Redis（`docker compose stop redis`）：每次 `Allow` 报错，executor 放行，于是各端点继续
回 200——计数器后端坏掉意味着不限流，而不是故障。恢复 Redis 即恢复限流。

### 4.4 经 starter-gateway

没有任何按路由的东西：本 starter 配好后，一条路由的预算已经由所有 gateway 副本共享，过滤器
只写预算。

```properties
spring.gateway.route.api.filters=rateLimit(rate=100)
```

### 4.5 冒烟测试

```bash
cd example && ./check.sh    # docker 门控：compose 起 redis，跑自断言示例
```

---

## 5. 排错

| 症状 | 可能原因 | 处理 |
|------|----------|------|
| 启动失败 `ratelimit-redis: missing required property "spring.ratelimit.redis.client"` | 配置块存在但 `client` 为空 | 填成已存在的 `spring.go-redis.instances.<name>`。 |
| 装配计数器存储时启动失败（没有名为 `<x>` 的 `*goredis.Client` bean） | `client` 指的是没配置的 redis 实例 | 补上 `spring.go-redis.instances.<x>`（或改正名字）。 |
| 各副本各自限流 | 没有存储被贡献（每个 executor 保持自己的私有预算），或副本指向不同 Redis 实例 | 每个副本都配置本 starter，并指向同一个 Redis。 |
| 完全不限流，全是 200 | 该 service 的 policy `rate-limit: 0`，或 Redis 不可达（executor 放行） | 在规则文件里给正数 `rate-limit`；检查 Redis。 |
| 实际突发与窗口算法对不上 | 规则要求 `algorithm: sliding-window`，而共享存储按令牌桶计数 | 把上限建模成 `rate-limit`/`burst`，或让 sliding window 继续用内存存储。 |
| 调用堆积而不是被拒绝 | 设了 `rate-limit-max-wait`，但共享存储不排队 | 排队继续留给内存存储，或去掉该参数。 |
| `spring.governance.enabled=false` 的应用完全没有限流 | 治理中心被关，所有 executor 都是直通 | 打开治理；本 starter 只搬计数器。 |

---

## 6. 设计健康度

| 指标 | 值 |
|------|-----|
| 配置键数 | 1 |
| 必填 | 1（`client`） |
| 快速上手外部依赖 | 1（Redis） |
| "注意"条目 | 2（sliding window 按令牌桶计数、不排队） |

设计疑点（审计台账用）：

- 已解决（2026-09）：按实例的 limiter 注册表已删除。容器里至多有一个计数器存储，由容器挑选
  （除非有后端 starter 贡献，否则一个都没有），所以旧失败模式——driver 名被两次占用、某个路由
  引用了没人注册的 driver 名、限流状态依赖同一测试二进制内的装配顺序——都不存在了。
- 存储的两条明示边界（sliding window 按令牌桶计数、不排队）是有意的：共享预算的正确性来自
  那条原子脚本。凡描述该存储之处都会重复这两条，以免调用方被它们意外到。
