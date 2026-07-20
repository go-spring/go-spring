# httpsvr
[English](README.md) | [中文](README_CN.md)

`httpsvr` 是极薄的 HTTP 服务端工具包:基于 Go 1.22+ `ServeMux` 的 `Server` 缝隙、
`RequestContext` 抽象、以及 JSON / SSE 泛型 handler。它是 `spring/httpclt` 的服
务端对偶,同样只 import 标准库。

## 特性

- `Server` 接口 + `SimpleServer`——基于 `http.ServeMux`,支持方法级
  pattern(`"GET /users/{id}"`)。
- `RequestContext` 接口 + `SimpleContext`——统一 `*http.Request` /
  `http.ResponseWriter` / `PathValue`,可经 `WithRequestContext` /
  `GetRequestContext` 存 / 取 `context.Context`。
- `RequestObject`(`Bind` + `Validate`)与 `ReadRequest`——按 `Content-Type`
  选 JSON 或 form 解码,不识别时用首字节嗅探,仅对 POST/PUT/PATCH 解析 body。
- `HandleJSON[Req, Resp]`——泛型 JSON handler 包装,写 `application/json` 并初
  始化 `ctxcache`。
- `HandleStream[Req, Resp]` + `Event[T]`——SSE,支持 `id` / `event` / `retry`
  字段。
- 可覆写的 `ErrorHandler` 与 `ReadBody`(默认上限 10 MiB)。

## 用法

```go
import (
    "context"
    "net/http"

    "go-spring.org/spring/web/httpsvr"
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
