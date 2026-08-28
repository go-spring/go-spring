# starter-session-redis 使用说明 — 参考手册

[English](USAGE.md) | [中文](USAGE_CN.md)

详尽使用参考。概览见 [README_CN.md](README_CN.md)。下文所有行为声明均对照 starter 源码
（`starter.go`、`config.go`、`store.go`）、共享抽象
[cloud/experimental/session](../../../cloud/experimental/session) 与自校验的
[example/](example)（`example/check.sh`）核实。session 语义（cookie 处理、空闲超时、
`RenewID`）在 session 包；Redis 语义见
[Redis 官方文档](https://redis.io/docs/latest/commands/set/)——本文只写 go-spring 的增量。

**激活方式**：任一 `spring.session.redis.<name>.*` 配置即为每个 `<name>` 注册一个
`session.SessionStore` 实例；每个实例复用其 `client` 字段指名的 `*redis.Client` bean
（由 starter-go-redis 在 `spring.go-redis.<client>` 下提供）。本 starter 不持有任何连接。

---

## 1. 完整工程示例

两个 HTTP"副本"经 Redis 共享同一 session 存储——登录轮换 session id，两个副本读到同一
session。文件树：

```
demo/
├── go.mod
├── main.go
├── web.go
└── conf/
    └── app.properties
```

**go.mod**（关键依赖）：

```
require (
    github.com/redis/go-redis/v9     latest
    go-spring.org/spring             v1.3.x
    go-spring.org/starter-go-redis   latest
    go-spring.org/starter-session-redis latest
)
```

**main.go**：

```go
package main

import (
    "go-spring.org/spring/gs"
    _ "go-spring.org/starter-go-redis"
    _ "go-spring.org/starter-session-redis"
)

func main() { gs.Run() }
```

**web.go** —— 应用的全部 session 面：

```go
package main

import (
    "net/http"
    "time"

    "go-spring.org/cloud/experimental/session"
    "go-spring.org/spring/gs"
)

func init() {
    // store bean 按名注入；Manager 的构造（cookie、空闲超时）是应用策略，
    // 不属于 starter 配置。
    gs.Provide(func(store session.SessionStore) *gs.HttpServeMux {
        opt := session.Options{
            CookieName:  "SESSION",        // 默认值
            IdleTimeout: 30 * time.Minute, // 默认值；过期是滑动的
        }
        mgrA := session.NewManager(store, opt) // "副本 A"
        mgrB := session.NewManager(store, opt) // "副本 B"

        mux := http.NewServeMux()
        mux.Handle("/a/set", mgrA.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            s, _ := session.FromContext(r.Context())
            s.Set("user", r.URL.Query().Get("user"))
        })))
        mux.Handle("/a/login", mgrA.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            s, _ := session.FromContext(r.Context())
            s.RenewID() // 防 session 固定：换 id、保数据
        })))
        mux.Handle("/b/get", mgrB.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            s, _ := session.FromContext(r.Context())
            if v, ok := s.Get("user"); ok {
                _, _ = w.Write([]byte(v.(string)))
            }
        })))
        return &gs.HttpServeMux{Handler: mux}
    }, gs.TagArg("web"))
}
```

**conf/app.properties** —— 上述代码用到的完整注释配置面：

```properties
# --- redis client（starter-go-redis 持有；store 按名复用） ---------------------
spring.go-redis.cache.addr=127.0.0.1:6379

# --- session store --------------------------------------------------------------
spring.session.redis.web.client=cache                 # 必填；为空 fail-fast
spring.session.redis.web.key-prefix=starter-session-redis:example:
```

**验证**（本地 Redis，可用 `example/docker-compose.yml`）：

```bash
go run . &
curl -si 'localhost:9090/a/set?user=alice' | grep -i set-cookie   # SESSION=...
curl -s --cookie 'SESSION=<id>' localhost:9090/b/get              # alice —— 跨副本
```

可运行的 [example/](example) 断言跨副本读取、登录轮换 id（旧 id 立即失效）、空闲超时
过期；`example/check.sh` 用 docker compose 包裹执行。

---

## 2. 装配与时序

### 2.1 bean 生命周期

```
import starter-go-redis + starter-session-redis
  └─ gs.Module(gs.OnProperty("spring.session.redis"))
        └─ conf.BindEach("${spring.session.redis}") 逐条目 <name>：
             ├─ client == "" 时 fail fast（启动报错并点名实例）
             └─ Provide newStore → bean "<name>"  （Export session.SessionStore；
                  gs.ValueArg(c), gs.TagArg(c.Client)  无 destroy 钩子）

gs.Run()
  ├─ 配置绑定：${spring.session.redis.<name>} → Config（value tag）
  ├─ newStore：不拨号——注入现成的 *redis.Client bean。
  │    Store 内嵌 session.FromByteStore(&redisByteStore{client, prefix})：
  │    session（反）序列化全部留在 stdlib 抽象里；具体类型只是
  │    可导出命名的包装（gs bean 不能返回未导出的接口实现）。
  ├─ bean 装配：消费方 autowire:"<name>" 解析
  └─ SIGTERM：无需释放——没有 destroy 钩子；session 留在 Redis 里
                 由 key TTL 过期。redis client 的 Close 属于 starter-go-redis。
```

### 2.2 一次请求的逐层走读（滑动过期）

带 cookie `SESSION=<id>` 的 `GET /b/get`：

1. `Manager.Middleware` 读 cookie（名字来自 `session.Options`，默认 `SESSION`）。
2. store 的 `Get(ctx, id)` 映射为 `GET <key-prefix><id>`——值是
   `session.FromByteStore` 产出的不透明 JSON blob；`redis.Nil`（未命中）映射为
   `found=false`，Manager 据此准备一个惰性分配的新 session。
3. 响应时 `sessionWriter.commit` 写穿为 `SET <key-prefix><id> <json> EX <IdleTimeout>`——
   **Redis key 的 TTL 就是空闲超时**，过期与滑动续期都由 Redis 服务端执行：经过中间件且
   已有 session 的每次请求（无论改没改）都会重新保存并把整个空闲窗口续满。全新的、
   未被触碰的 session 什么都不写——匿名流量不分配 session。非正的空闲超时映射为 Redis
   expiration 0 = 永不过期。
4. `RenewID()`（登录）生成新 id、数据落在新 id 下并删旧 key——旧 session id 立即失效
   （example 有断言）。
5. cookie 处理：`Set-Cookie`（`Max-Age = IdleTimeout`）在首个字节到达客户端前恰好写一次
   （由 Manager 的 responseWriter 包装保证 header 顺序）。

HTTP 相关（cookie 名/路径/Secure/SameSite、空闲超时）都在 Manager 构造时的
`session.Options` 上——刻意不做成 starter 配置；一个 store 可以背多个 Manager。

---

## 3. 逐 key 行为参考

所有 key 位于 `spring.session.redis.<name>` 之下（精确匹配，无宽松形态）。

| Key | 类型 | 默认值 | 行为 / 联动 | 配错后果 |
|-----|------|--------|-------------|----------|
| `client` | string | — | **必填**。`spring.go-redis.<client>` 下的 `*redis.Client` bean 名，注册前检查。`TagArg(c.Client)` 即 store 与 redis 实例的接线 seam。 | 空 → 启动失败并点名实例；拼错 → 启动期装配失败。 |
| `key-prefix` | string | `session:` | 拼在每个 session id 前再进 Redis，让共享一个 Redis 的多应用键空间不冲突。 | 跨应用共享前缀 → session 跨应用泄漏（同 cookie 值可互相解析）。 |

⚠ 这就是全部配置面：两个 key。其余（cookie、过期、序列化）要么是 `session.Options`
要么在共享抽象里。

---

## 4. 验证与故障演练

### 4.1 跨副本 session 共享

```bash
curl -si 'localhost:9090/a/set?user=alice' | awk -F': ' '/[Ss]et-[Cc]ookie/{print $2}'
curl -s --cookie 'SESSION=<id>' localhost:9090/b/get     # "alice" —— 副本 B 读到副本 A 的写
```

底层：

```bash
redis-cli --scan --pattern 'starter-session-redis:example:*'
redis-cli ttl 'starter-session-redis:example:<id>'       # = IdleTimeout
```

### 4.2 滑动窗口演练

以 `IdleTimeout: 2s`（同 example）：

```bash
id=<cookie>
redis-cli ttl "starter-session-redis:example:$id"   # ~2
sleep 1; curl -s --cookie "SESSION=$id" localhost:9090/b/get >/dev/null
redis-cli ttl "starter-session-redis:example:$id"   # 回到 ~2 —— 请求重新续满
sleep 3
curl -s --cookie "SESSION=$id" localhost:9090/b/get # 空 —— Redis 判定空闲过期
```

### 4.3 session id 轮换演练（防固定攻击）

```bash
old=$(curl -si 'localhost:9090/a/set?user=alice' | awk ...)     # 第一个 id
new=$(curl -si --cookie "SESSION=$old" localhost:9090/a/login | awk ...)  # 轮换后的 id
curl -s --cookie "SESSION=$new" localhost:9090/b/get   # alice —— 数据保留
curl -s --cookie "SESSION=$old" localhost:9090/b/get   # 空 —— 旧 id 已销毁
```

### 4.4 冒烟测试

```bash
cd example && ./check.sh    # docker 门控：compose 起 redis，跑自校验 example
```

---

## 5. 排障表

| 症状 | 可能原因 | 处置 |
|------|----------|------|
| 启动报 `session-redis: instance "<n>" missing required property ...client` | 实例缺 `client` | 设为既有 `spring.go-redis.<name>`。 |
| 启动期装配 `*redis.Client` 失败 | `client` 拼错 | 改名对齐 `spring.go-redis.<client>` 条目。 |
| 无关应用之间 session 串了 | 同一 Redis 上 `key-prefix` 相同（或都用默认 `session:`） | 每个应用独立 `key-prefix`。 |
| session 永不过期 | `Options` 的 `IdleTimeout` 为 0/负 → Redis "永不过期" | 设正的 `IdleTimeout`。 |
| 持续访问却中途过期 | 流量绕过了 Manager 的 Middleware，TTL 没人滑动 | 经过 `mgr.Middleware` 且已有 session 的请求才续窗口；确认中间件真的包住了路由。 |
| 登录后旧 session 仍有效 | handler 没调 `RenewID` | 权限变化处调 `s.RenewID()`。 |
| Redis 重启全员掉线 | session 仅在内存 | 属预期；配 Redis 持久化或接受重新登录。 |
| `curl` 看不到 cookie | `Set-Cookie` 在首个 body 字节前一次性写入；之后的写入被包装丢弃 | 别在流式开始后检查；对原始响应用 `-i`。 |

---

## 6. 设计体检表

| 指标 | 数值 |
|------|------|
| 配置 key 总数 | 2 |
| 其中必填 | 1（`client`） |
| quickstart 前置外部依赖 | 1（Redis） |
| "注意/坑"条数 | 2 |

设计嫌疑清单：

- 无结构性问题。两 key 配置面极简，HTTP/session 的分层干净；唯一摩擦是所有 redis 消费型
  starter 共享的跨模块 `client` 名引用（上文已注明）。
- 本 starter 完全无可观测（没有 lock 家族的 observer 块）——store 时延/错误不可见；未来可
  做 observe-session 桥，镜像 observe-lock。
