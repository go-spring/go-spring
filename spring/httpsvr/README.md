# httpsvr
[English](README.md) | [中文](README_CN.md)

`httpsvr` is a thin HTTP server toolkit: a Go 1.22+ `ServeMux`-based `Server`
seam, a `RequestContext` abstraction, and generic handler wrappers for JSON and
Server-Sent Events. It is the server-side counterpart to `spring/httpclt` and,
like it, imports only stdlib packages.

## Features

- `Server` interface + `SimpleServer` — `http.ServeMux` with method-scoped
  patterns (`"GET /users/{id}"`).
- `RequestContext` interface + `SimpleContext` — unified `*http.Request` /
  `http.ResponseWriter` / `PathValue` access, stashable in `context.Context`
  via `WithRequestContext` / `GetRequestContext`.
- `RequestObject` (`Bind` + `Validate`) and `ReadRequest` — body parsing that
  picks JSON vs form by `Content-Type`, with sniff fallback, honouring
  `MethodPost/Put/Patch` only.
- `HandleJSON[Req, Resp]` — generic JSON handler wrapper, writes
  `application/json` and initializes `ctxcache`.
- `HandleStream[Req, Resp]` + `Event[T]` — Server-Sent Events with `id` /
  `event` / `retry` fields.
- Overridable `ErrorHandler` and `ReadBody` (default 10 MiB cap).

## Usage

```go
import (
    "context"
    "net/http"

    "go-spring.org/spring/httpsvr"
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
