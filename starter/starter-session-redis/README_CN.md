# starter-session-redis

[English](README.md) | [中文](README_CN.md)

`starter-session-redis` 为 Go-Spring 应用贡献一个 Redis 后端的
[`session.SessionStore`](../../stdlib/session) bean,使 HTTP 会话可在多副本间共享:
副本 A 写入,副本 B 读取——这是 Spring Session 的等价能力,用「中间件 + 配置」达成,
而非照搬 `@EnableRedisHttpSession` 注解体系。

它遵循 *Contributor*(贡献者)形态(见 [starter/DESIGN.md](../DESIGN.md)):starter
不开端口、自身不持有客户端。它复用 `starter-go-redis` 注册的 `*redis.Client` bean,
在框架中立的 `session.SessionStore` 缝隙背后贡献一个 bean。因此把会话后端切换到任何
其它分布式存储,只是一次 blank-import 的替换——业务代码不变。

## 安装

```bash
go get go-spring.org/starter-session-redis
```

## 快速开始

### 1. 同时导入两个 starter

```go
import (
    _ "go-spring.org/starter-go-redis"
    _ "go-spring.org/starter-session-redis"
)
```

### 2. 先配置 Redis 客户端,再配置引用它的会话存储

```properties
# 由 starter-go-redis 管理的 Redis 客户端。
spring.go-redis.cache.addr=127.0.0.1:6379

# 绑定到该客户端的会话存储。`client` 即上面的 redis 实例名。
spring.session.redis.web.client=cache
spring.session.redis.web.key-prefix=myapp:session:
```

`client` 属性是**必填**的。缺失即启动失败(fail-fast)——starter 拒绝静默回退到某个
任意的 Redis 实例。

### 3. 注入存储并挂载会话中间件

存储以 `session.SessionStore` 接口导出。把它交给 `session.Manager` 并包裹你的
handler;中间件会从请求 Cookie 解析 session id,把会话加载进 context,并在响应头
发送前写回。

```go
import (
    "net/http"

    "go-spring.org/spring/gs"
    "go-spring.org/stdlib/session"
)

gs.Provide(func(store session.SessionStore) *gs.HttpServeMux {
    mgr := session.NewManager(store, session.Options{
        CookieName:  "SESSION",
        IdleTimeout: 30 * time.Minute, // 滑动续期:每次请求都刷新
        Secure:      true,             // 生产环境仅经 HTTPS 传输
    })

    mux := http.NewServeMux()
    mux.HandleFunc("/cart/add", func(w http.ResponseWriter, r *http.Request) {
        s, _ := session.FromContext(r.Context())
        s.Set("item", r.URL.Query().Get("item"))
    })
    mux.HandleFunc("/login", func(w http.ResponseWriter, r *http.Request) {
        s, _ := session.FromContext(r.Context())
        s.RenewID()               // 认证后换 id——防会话固定攻击
        s.Set("user", "alice")
    })

    return &gs.HttpServeMux{Handler: mgr.Middleware(mux)}
}, gs.TagArg("web"))
```

由于中间件只面向 `session.SessionStore`,多个副本上的多个 `Manager` 只要背靠同一个
Redis,就能透明地共享会话状态。

## 配置

所有配置项位于 `spring.session.redis.<name>` 之下:

| 配置项       | 默认值      | 说明                                                                    |
|--------------|-------------|-------------------------------------------------------------------------|
| `client`     | —           | **必填。** `spring.go-redis.<client>` 下 `*redis.Client` bean 的名字。   |
| `key-prefix` | `session:`  | 拼在每个 session id 前,使多个应用可安全共享同一个 Redis 实例。          |

Cookie 名称、路径、`Secure`、`SameSite` 以及空闲超时都在构建 `Manager` 时通过
`session.Options` 设置——它们是 HTTP 关注点而非存储关注点,故随中间件走,而不放在
存储配置里。

## 行为特性

* **跨副本共享**——会话以其 id 存在 Redis 中;任何收到该 Cookie 的副本都能加载到
  同一个会话。
* **滑动续期**——每个携带会话的请求都会把 Redis key 的 TTL(以及 Cookie 的
  `Max-Age`)刷新为空闲超时;活跃会话因此保持存活,空闲会话在最后一次请求后恰好
  `IdleTimeout` 过期。
* **防会话固定**——`Session.RenewID()` 轮换 id(删除旧的 Redis 条目)并保留属性;
  登录成功后立即调用。
* **惰性分配**——从不写会话的访客不会拿到 Cookie,也不会创建 Redis 条目;id 在第一次
  `Set` 时才签发。
* **安全 id**——256 位 `crypto/rand`,id 不可猜测。
* **fail-fast 配置**——缺失 `client` 直接拒绝启动,而不是等到第一次读会话才暴露。
