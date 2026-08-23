# httpclt

[English](README.md) | [中文](README_CN.md)

`httpclt` is the runtime the generated declarative HTTP client (produced by the
`go-spring.org/gs-http-gen` code generator) calls into. It holds no state: it
carries `Metadata`, applies `RequestOption`s, encodes the body via
`stdlib/jsonflow`, and finally dispatches the request through `DoRequest` (a
replaceable var that defaults to `http.DefaultClient`). It lives in `stdlib`
and imports only `net/http`, `stdlib/jsonflow`, and a couple of standard
packages, so generated clients stay stdlib-only and never leak a starter
dependency into caller code.

## Usage

Import path: `go-spring.org/stdlib/httpclt`. Generated clients call these
helpers directly; a hand-written call looks like:

```go
import (
    "context"
    "fmt"
    "net/http"

    "go-spring.org/stdlib/httpclt"
)

type GreetResp struct{ Message string }

meta := httpclt.Metadata{
    Target:  "user-svc",
    Schema:  "http",
    Method:  http.MethodGet,
    RawPath: "/greet",
    Header:  http.Header{"X-Trace": []string{"1"}},
}
resp, out, err := httpclt.JSONResponse[*GreetResp](context.Background(), meta)
if err != nil {
    return err
}
defer resp.Body.Close() // the default DoRequest already closed it; double Close is safe
fmt.Println(out.Message)
```

### API

| API | Description |
|---|---|
| `Metadata` | Full context of one request: Target/Schema/Method/Pattern/RawPath/Query/Body/Header/Config |
| `RequestOption` / `CombineMetadata` | Function options that modify a `Metadata` in place |
| `WithHeader(http.Header)` | Option that merges request headers into `Metadata.Header` |
| `WithConfig(map[string]string)` | Option that merges entries into `Metadata.Config` |
| `QueryStringer` (interface) | `QueryForm() (string, error)` — query encoding extension point |
| `ResponseObject` (interface) | `DecodeJSON(jsonflow.Decoder) error` — streaming decode extension point |
| `ObjectResponse[T ResponseObject]` | Streams the response through T's own `DecodeJSON` |
| `JSONResponse[T any]` | Streams the response into any T via `jsonflow.UnmarshalRead` |
| `DoRequest` (var) | The single dispatch seam; replace wholesale to inject a transport stack |

The three extension points (`QueryForm` / `EncodeForm` / `DecodeJSON`) are all
implemented by generated types: the runtime never reflects over business
structs, staying fast and dependency-free, with the dual responsibility (the
per-type encode/decode code) carried by the code generator. Note `EncodeForm`
is not a named interface — a body implementing
`EncodeForm() (string, error)` is sent form-encoded (see below).

The fields of `Metadata` are the contract with `gs-http-gen` output — renaming
a field breaks it. Some fields exist purely for that contract: `Pattern` is
emitted by the generator but never read by the runtime, which only uses
`RawPath` (the path with placeholders already processed). Responses default to
streaming JSON: `ObjectResponse` / `JSONResponse` decode incrementally through
`jsonflow` and never buffer the full response body in memory.

### Request Lifecycle

`ObjectResponse` / `JSONResponse` share the same internal `doRequest` path,
in a fixed order:

1. **query**: when `meta.Query != nil`, call `QueryForm()`; a non-empty result
   is appended to `RawPath` (`?k=v`);
2. **body**: if `meta.Body` implements `EncodeForm() (string, error)`, it is
   sent as a form string; otherwise (when non-nil) it is streamed as JSON via
   `jsonflow.MarshalWrite`;
3. **build the request**: `http.NewRequestWithContext(ctx, Method, RawPath,
   body)`, then `req.Host = Target`, `URL.Host = Target`, `URL.Scheme =
   Schema` — that is, `Target` may be a service name (discovery scenario) or
   an `IP:PORT` (direct connection), and it is the replaced `DoRequest` that
   decides how to interpret it; headers are merged last;
4. **dispatch**: hand off to `DoRequest(req, meta, fn)`, which performs the
   request and feeds the response body to the decode callback `fn`.

### The DoRequest Contract

The default implementation is "send → feed `resp.Body` to `fn` → close the
body → return the resp". The replacement receives an **already-built
`*http.Request`** plus the original `Metadata` and the decode callback `fn`,
so it controls the whole request/response lifecycle: swap the transport
(discovery + load balancing), short-circuit retries, wrap resilience, emit
logging/metrics/tracing — all it must guarantee is that `fn` gets called and
the body gets closed. `Metadata` travels with the request: the replacement
can read `Target` for routing, while `Config` carries extra conventions
between the caller and the replacement.

Transport, timeouts, cookie jar, and TLS config are never constructed inside
`httpclt` — all of that belongs to the replaced `DoRequest`. That is exactly
why client-side integrations (`starter-http-client`, tests, contract stubs)
stay pluggable without touching generated code.

For a real wiring example see `starter-http-client`: it assembles a
process-wide `dispatchTransport`, routes each request by `req.Host` (i.e. the
Target) to its configured transport, and replaces `httpclt.DoRequest`
wholesale — generated code is completely unaware. The combined implementation
of discovery, load balancing, resilience, and trace propagation lives in
`go-spring.org/cloud/experimental/httpx`.

## License

Apache License 2.0. See [LICENSE](../../LICENSE).
