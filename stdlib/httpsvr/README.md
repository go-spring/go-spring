# httpsvr

[English](README.md) | [中文](README_CN.md)

`httpsvr` is a thin HTTP server toolkit: a Go 1.22+ `ServeMux`-based `Server`
seam, a `RequestContext` abstraction, and generic handler wrappers for JSON and
Server-Sent Events. It is the server-side counterpart to `stdlib/httpclt` —
the shape that generated handlers plug into.

## Usage

Import path: `go-spring.org/stdlib/httpsvr`.

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

### API

| API | Description |
|---|---|
| `Server` (interface) / `Router` | Routing seam: `Route(Router)` registers one route (Method + Pattern + Handler) |
| `NewSimpleServer(addr)` / `SimpleServer` | Default `Server` on `http.ServeMux`, Go 1.22+ method-scoped patterns |
| `RequestContext` / `SimpleContext` / `NewSimpleContext` | Request/response pair abstraction with `PathValue` |
| `WithRequestContext` / `GetRequestContext` | Store / retrieve a `RequestContext` in a `context.Context` |
| `NewRequestContext` | Factory type: `func(r *http.Request, w http.ResponseWriter) RequestContext` |
| `RequestObject` (interface) | `Bind(*http.Request) error` + `Validate() error` |
| `ReadRequest(r, obj)` | Decode body (JSON or form by Content-Type) then `Bind` then `Validate` |
| `ReadBody` (var) | Overridable body reader, default 10 MiB cap |
| `ErrorHandler` (var) | Overridable error reporter, default 500 + message |
| `HandleJSON[Req, Resp]` | Generic JSON handler wrapper |
| `HandleStream[Req, Resp, T]` + `Event[T]` | SSE handler wrapper; `Resp` must be `*Event[T]` |
| `NewEvent[T]` / `Event[T]` methods | Build one SSE event: `ID` / `Event` / `Data` / `Retry` setters + `Has*` / `Get*` |

## Server: the routing seam

The `Server` interface is a single method, `Route(Router)`:

- `SimpleServer` is the default implementation — `http.ServeMux` with
  method-scoped patterns (`"GET /users/{id}"`).
- A starter wanting a different underlying router (chi, gin, ...) implements
  that one method and everything else keeps working.

Why no pluggable router abstraction? Go 1.22 gave `http.ServeMux`
method-aware patterns, which is enough for the seam-level responsibility of
this package; a third-party router would violate the zero-dependency rule.
Conversely, the package does no binding-tag magic, no middleware chain, no
dependency injection — those belong at the starter layer or in user code.

## RequestContext: the request/response pair

The `RequestContext` interface + `SimpleContext` unify `*http.Request` /
`http.ResponseWriter` / `PathValue` access, stashable in `context.Context`
via `WithRequestContext` / `GetRequestContext`.

The point: even a handler that received only a `context.Context` (passed
through `ctxcache` or friends) can still reach the writer and respond.

## Request parsing: ReadRequest and RequestObject

`RequestObject` (`Bind` + `Validate`) and `ReadRequest` pick JSON vs form
decoding by `Content-Type`:

- **Method gate**: bodies are read only for `POST` / `PUT` / `PATCH`; any
  other method skips `decodeBody`, and a `GET`-with-body request is treated
  as bodyless.
- **Content negotiation**: JSON vs form by `Content-Type`, falling back to a
  first-byte sniff so an unlabelled body still parses. Real APIs either use
  JSON or `x-www-form-urlencoded`, and the sniff covers the common case of a
  missing header; richer content negotiation is deferred.
- **Bind timing**: `RequestObject.Bind` runs after body decode; a decode
  error short-circuits before `Bind`, so `Bind` may assume decoded fields are
  already populated for body-carrying methods.

Body reads go through the overridable `ReadBody` (default 10 MiB cap), so an
application can enforce a smaller cap without wrapping `HandleJSON`.

## Response framing: HandleJSON and HandleStream

- `HandleJSON[Req, Resp]` is the generic JSON handler wrapper: writes
  `application/json` and initializes `ctxcache`. `Content-Type` is set before
  the handler runs, so a business handler cannot forget it.
- `HandleStream[Req, Resp]` + `Event[T]` provide Server-Sent Events with
  `id` / `event` / `retry` fields. It requires the `http.ResponseWriter` to
  implement `http.Flusher` and reports a 500 through `ErrorHandler`
  otherwise — wrapping writers must not hide flushing.

Errors surface through the overridable `ErrorHandler`, so an application can
switch to a JSON error envelope, again without wrapping `HandleJSON`.

## Where cross-cutting concerns live

The package ships no middleware slice — chains belong at higher layers: the
`cloud/experimental/security` middleware chain, per-family method-level
decorators, or a starter wrapping the `Server.Route` seam. Baking one in here
would force users into that ordering.

## License

Apache License 2.0. See [LICENSE](../../LICENSE).
