# httpclt
[English](README.md) | [中文](README_CN.md)

`httpclt` is the runtime the generated declarative HTTP client (see
`stdlib/httpx` and `gs/gs-http-gen`) calls into. It holds no state itself: it
carries request `Metadata`, applies `RequestOption`s, encodes the body via
`stdlib/jsonflow`, and dispatches the actual call through the caller-supplied
`*http.Client` (falling back to `http.DefaultClient` when none is set).

## Features

- `Metadata` — target/schema/method/pattern/query/body/header/config, plus a
  `Client` field so a generated client can carry its own instrumented transport.
- `RequestOption` composition via `CombineMetadata`, with `WithHeader` and
  `WithConfig` built in.
- Streaming JSON decode helpers: `ObjectResponse[T]` for a type that implements
  `DecodeJSON`, and `JSONResponse[T]` for a generic `any`.
- `QueryStringer` extension point for query encoding; bodies implementing
  `EncodeForm` are sent as form-encoded, otherwise streamed as JSON.
- `DoRequest` is a `var` so tests / instrumentation can stub the whole HTTP hop.

## Usage

Generated clients call the helpers directly; a hand-written call looks like:

```go
import (
    "context"
    "net/http"

    "go-spring.org/stdlib/httpclt"
)

type GreetResp struct{ Message string }

func (r *GreetResp) DecodeJSON(d jsonflow.Decoder) error { /* ... */ }

meta := httpclt.Metadata{
    Target:  "user-svc",
    Schema:  "http",
    Method:  http.MethodGet,
    RawPath: "/greet",
    Header:  http.Header{"X-Trace": []string{"1"}},
    Client:  httpClient, // an *http.Client whose Transport is built by httpx
}
resp, out, err := httpclt.ObjectResponse(context.Background(), &GreetResp{}, meta)
```
