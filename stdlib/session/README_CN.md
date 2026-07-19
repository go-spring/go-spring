# session
[English](README.md) | [中文](README_CN.md)

`session` 是与框架无关、零依赖的服务端 HTTP 会话抽象——Spring Session 等价
能力用 Go 惯用法表达,而非对 `@EnableRedisHttpSession` 机制的移植。有状态的
Web / SSO 部署可以做到副本 A 写会话、副本 B 读会话,业务 handler 无需改动。

## 特性

- 零第三方依赖。
- 三段式拆分(与 `cache` / `lock` / `security` 同构):
  - `Session`——id + 属性 bag + createdAt;通过 `FromContext` 从 ctx 拿,业
    务代码不直接构造。
  - `SessionStore`——`Load` / `Save(ttl)` / `Delete`。远端后端实现更窄的
    `ByteStore`,由 `FromByteStore` 抬升(JSON 编码)。命名 store 通过
    `Register` / `Get` / `MustGet` 注册。内置 `Memory` 已注册为 `"memory"`。
  - `Manager`——唯一 HTTP 缝隙。`Manager.Middleware` 按 cookie 加载、挂到
    ctx、在首个响应字节前写回。
- 惰性分配 id:未触碰的访问不落库、不发 cookie。
- 滑动续期:任何携带 session 的请求都刷新 TTL 与 cookie `Max-Age`。
- `Session.RenewID` 在权限变更(登录)时轮换 id,防会话固定。
- `Session.Invalidate` 销毁服务端状态并让 cookie 立即过期(登出)。

## 快速开始

Import 路径: `go-spring.org/stdlib/session`。

```go
package main

import (
    "fmt"
    "net/http"
    "time"

    "go-spring.org/stdlib/session"
)

func main() {
    store := session.NewMemory() // 或从 starter 拿到分布式后端
    mgr := session.NewManager(store, session.Options{
        CookieName:  "SESSION",
        IdleTimeout: 30 * time.Minute,
    })

    mux := http.NewServeMux()
    mux.HandleFunc("/login", func(w http.ResponseWriter, r *http.Request) {
        s, _ := session.FromContext(r.Context())
        s.RenewID()             // 防会话固定
        s.Set("user", "u-1")
        _, _ = w.Write([]byte("ok"))
    })
    mux.HandleFunc("/me", func(w http.ResponseWriter, r *http.Request) {
        s, _ := session.FromContext(r.Context())
        if v, ok := s.Get("user"); ok {
            fmt.Fprintf(w, "user=%v", v)
            return
        }
        http.Error(w, "unauthenticated", http.StatusUnauthorized)
    })

    _ = http.ListenAndServe(":8080", mgr.Middleware(mux))
}
```

跨副本共享用 `starter-session-redis`;它基于 Redis client 通过
`FromByteStore` 贡献一个 `session.SessionStore` bean,`Manager` API 完全不
变。
