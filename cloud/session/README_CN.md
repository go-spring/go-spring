# session
[English](README.md) | [中文](README_CN.md)

`session` 是与框架无关、零依赖的服务端 HTTP 会话抽象。有状态的 Web / SSO
部署可以做到副本 A 写会话、副本 B 读会话,业务 handler 无需改动。内置
`Memory` store 服务单机与测试;分布式后端由 `starter-session-redis` 以同一
接口贡献。

## 特性

- 零第三方依赖;兼容任何 `net/http` 系路由。
- 类型化属性存取:`session.Set[T]` / `session.Get[T]`,字节后端往返自动经
  JSON 重编码。
- 惰性分配 id:未触碰的访问不落库、不发 cookie。
- 滑动续期:任何携带 session 的请求都刷新 TTL 与 cookie `Max-Age`。
- `Session.RenewID` 在权限变更(登录)时轮换 id,防会话固定。
- `Session.Invalidate` 销毁服务端状态并让 cookie 立即过期(登出)。

## 快速开始

Import 路径: `go-spring.org/cloud/session`。

```go
package main

import (
    "fmt"
    "net/http"
    "time"

    "go-spring.org/cloud/session"
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
        s.RenewID()                     // 防会话固定
        session.Set(s, "user", "u-1")   // 类型化属性存取
        _, _ = w.Write([]byte("ok"))
    })
    mux.HandleFunc("/me", func(w http.ResponseWriter, r *http.Request) {
        s, _ := session.FromContext(r.Context())
        if v, ok, _ := session.Get[string](s, "user"); ok {
            fmt.Fprintf(w, "user=%v", v)
            return
        }
        http.Error(w, "unauthenticated", http.StatusUnauthorized)
    })

    _ = http.ListenAndServe(":8080", mgr.Middleware(mux))
}
```

上述流程的可执行自断言版本(惰性分配、登录轮换、类型化属性、登出、空闲过
期)在 [example/](example/)。

## 使用方式

### 挂载中间件

在一个 store 上构造 `Manager`,包住使用 session 的 handler——会话传输只
存在这一处。多个 Manager(乃至跨副本)可共享同一个 store,这正是会话状态
共享的方式:

```go
mgr := session.NewManager(store, session.Options{IdleTimeout: 30 * time.Minute})
mux.Handle("/profile", mgr.Middleware(profileHandler))
```

被包裹的 handler 内部从请求 context 拿 session:

```go
s, _ := session.FromContext(r.Context())
```

### 读写属性

Go 方法不能带类型参数,因此类型化存取是包级函数:

```go
session.Set(s, "cart", []string{"sku-1", "sku-2"})
items, ok, err := session.Get[[]string](s, "cart")
```

进程内 `Memory` store 直接返回原类型。从字节型后端加载的会话属性会以
`map[string]any` / `float64` 形态回来;`Get` 经 JSON 重编码仍能得到目标类
型——这也意味着远端存储的属性应保持 JSON 友好。无法解码成 `T` 的值会报
错,而非静默零值。`s.Delete(key)` 删除单个属性;`s.Keys()` 列出全部。

### 登录、登出

```go
// 登录:轮换客户端在认证前持有的 id,再记录已认证用户。旧 id 在写回时失效。
s.RenewID()
session.Set(s, "user", user)

// 登出:删除 store 条目并让客户端 cookie 过期。
s.Invalidate()
```

两者都在写回时生效,早于首个响应字节到达客户端。

### 配置 cookie 与空闲超时

`Options` 覆盖 `CookieName`(默认 `"SESSION"`)、`Path`、`Domain`、
`Secure`、`SameSite`(默认 `Lax`)与 `IdleTimeout`(默认 30m)。
`IdleTimeout` 是会话可空闲的时长:每个携带会话的请求都会把期限向后滑。
非正值表示服务端永不过期、cookie 为会话 cookie。cookie 恒为 `HttpOnly`
且不可配置;id 为 `crypto/rand` 32 字节、base64url 编码。

### 贡献分布式后端

远端 store 在自家 client 上实现窄接口 `ByteStore`(`Get` / `Set` /
`Delete` 操作 `[]byte`),经 `FromByteStore` 抬升为完整 `SessionStore`,
JSON 序列化由它统一承担:

```go
store := session.FromByteStore(myRedisByteStore)
mgr := session.NewManager(store, opt)
```

store 通过构造器注入(`NewManager`)到达 `Manager`——没有包级 store 注册表,
store 状态不会意外跨测试/重启共享。内置 `Memory` 显式构造
(`session.NewMemory()`);需要活 client 的后端(Redis 等)以 bean 形式贡献。
`starter-session-redis` 正是这么做的;换成它之后 `Manager` API 完全不变。

## 行为契约

- **尽早改属性。** `Set-Cookie` 必须先于响应体,因此会话在首次
  `WriteHeader` / `Write` 之前提交(handler 未写响应时在中间件退出前再兜底
  一次)。首次写之后的属性变更静默不落盘,与任何 header 同约束。
- **匿名流量零分配。** 未触碰 session 的请求拿不到 id、不落库、不发
  cookie。
- **携带即重存。** 即便属性没改,携带会话的请求也会 `Save` 一次,刷新
  store TTL 与 cookie `Max-Age`(滑动续期)。
- **响应中途 store 失败**:新会话不发 cookie——客户端不会拿到 store 里不
  存在的 id;响应本身正常完成。

## 设计说明

- `SessionStore` 是持久化缝隙;同一份中间件无需改动即可服务进程内
  `Memory` 或共享 Redis 后端。
- `ByteStore` 序列化用 JSON,不用 gob:跨语言可读、无版本意外。
- 本包不是身份提供者——session 属性是任意 bag;"调用者是谁"归
  `cloud/security`。
