# httpsvr

[English](README.md) | [中文](README_CN.md)

`httpsvr` 是极薄的 HTTP 服务端工具包：基于 Go 1.22+ `ServeMux` 的 `Server` 缝隙、`RequestContext` 抽象、以及 JSON / SSE 泛型 handler。它是 `stdlib/httpclt` 的服务端对偶——生成 handler 所插入的服务端骨架。

## 使用方式

导入路径：`go-spring.org/stdlib/httpsvr`。

```go
import (
    "context"
    "net/http"

    "go-spring.org/stdlib/httpsvr"
)

type GreetReq struct {
    Name string `form:"name"`
}
func (r *GreetReq) Bind(rq *http.Request) error { r.Name = rq.URL.Query().Get("name"); return nil }
func (r *GreetReq) Validate() error              { return nil }

type GreetResp struct{ Message string }

func main() {
    s := httpsvr.NewSimpleServer(":8080")
    s.Route(httpsvr.Router{
        Method:  http.MethodGet,
        Pattern: "/greet",
        Handler: func(w http.ResponseWriter, r *http.Request) {
            httpsvr.HandleJSON(w, r, &GreetReq{}, func(ctx context.Context, req *GreetReq) *GreetResp {
                return &GreetResp{Message: "Hello, " + req.Name + "!"}
            })
        },
    })
    _ = s.ListenAndServe()
}
```

### API 列表

| API | 说明 |
|---|---|
| `Server`（接口）/ `Router` | 路由缝隙：`Route(Router)` 注册一条路由（Method + Pattern + Handler） |
| `NewSimpleServer(addr)` / `SimpleServer` | 默认 `Server` 实现，基于 `http.ServeMux`，Go 1.22+ 方法级 pattern |
| `RequestContext` / `SimpleContext` / `NewSimpleContext` | 请求/响应对抽象，含 `PathValue` |
| `WithRequestContext` / `GetRequestContext` | 向 `context.Context` 存 / 取 `RequestContext` |
| `NewRequestContext` | 工厂类型：`func(r *http.Request, w http.ResponseWriter) RequestContext` |
| `RequestObject`（接口） | `Bind(*http.Request) error` + `Validate() error` |
| `ReadRequest(r, obj)` | 先解码 body（按 Content-Type 选 JSON / form），再 `Bind`，再 `Validate` |
| `ReadBody`（var） | 可覆写的 body 读取，默认上限 10 MiB |
| `ErrorHandler`（var） | 可覆写的错误出口，默认 500 + 错误信息 |
| `HandleJSON[Req, Resp]` | 泛型 JSON handler 包装 |
| `HandleStream[Req, Resp, T]` + `Event[T]` | SSE handler 包装；`Resp` 须为 `*Event[T]` |
| `NewEvent[T]` / `Event[T]` 方法 | 构造一条 SSE 事件：`ID` / `Event` / `Data` / `Retry` 设置器 + `Has*` / `Get*` |

## Server：路由缝隙

`Server` 接口只有一个方法 `Route(Router)`：

- `SimpleServer` 是默认实现——基于 `http.ServeMux`，支持方法级 pattern（`"GET /users/{id}"`）。
- starter 想换底层 router（chi / gin……）只需实现这一个方法，其他不动。

为什么不提供可插拔的 router 抽象？Go 1.22 的 `http.ServeMux` 已支持方法级 pattern，足以承担本包的缝隙职责；引第三方 router 会破坏零依赖约定。反过来，本包也不做绑定 tag 魔法、无中间件链、无 DI——这些属于 starter 层或用户代码。

## RequestContext：请求/响应对

`RequestContext` 接口 + `SimpleContext` 统一 `*http.Request` / `http.ResponseWriter` / `PathValue` 访问，可经 `WithRequestContext` / `GetRequestContext` 存 / 取 `context.Context`。

它的价值在于：即使 handler 只拿到 `ctx`（经 `ctxcache` 等中间机制传递），也能取回 writer 完成响应。

## 请求解析：ReadRequest 与 RequestObject

`RequestObject`（`Bind` + `Validate`）与 `ReadRequest` 负责按 `Content-Type` 选 JSON 或 form 解码：

- **方法门槛**：只对 `POST` / `PUT` / `PATCH` 读 body；其他方法跳过 `decodeBody`，带 body 的 `GET` 视作无 body。
- **内容协商**：按 `Content-Type` 选 JSON / form，不识别时用首字节嗅探——漏设 header 的 body 也能解析。真实 API 要么 JSON 要么 `x-www-form-urlencoded`，嗅探已覆盖常见场景；更完整的内容协商延后。
- **Bind 时机**：`RequestObject.Bind` 在 body 解码之后运行；解码失败直接短路、不会调用 `Bind`，故对带 body 的方法 `Bind` 可假定字段已解码填充。

Body 读取经可覆写的 `ReadBody`（默认上限 10 MiB），应用可下调上限而无需包装 `HandleJSON`。

## 响应编帧：HandleJSON 与 HandleStream

- `HandleJSON[Req, Resp]` 是泛型 JSON handler 包装，写 `application/json` 并初始化 `ctxcache`。`Content-Type` 在 handler 执行前就已设置，业务 handler 不会忘设。
- `HandleStream[Req, Resp]` + `Event[T]` 提供 SSE，支持 `id` / `event` / `retry` 字段。它要求 `http.ResponseWriter` 实现 `http.Flusher`，否则经 `ErrorHandler` 报 500——包装 writer 时不能丢失 Flusher。

错误出口统一走可覆写的 `ErrorHandler`，应用可改用 JSON 错误体格式，同样无需包装 `HandleJSON`。

## 横切能力的归属

本包不内置中间件切片——链式装配属于更高层：`cloud/experimental/security` 的中间件、各家族自带的方法级装饰器，或 starter 包装 `Server.Route` 缝隙。在这里内置会锁死顺序。

## 许可证

Apache License 2.0，详见 [LICENSE](../../LICENSE)。
