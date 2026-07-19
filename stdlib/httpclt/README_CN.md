# httpclt
[English](README.md) | [中文](README_CN.md)

`httpclt` 是生成的声明式 HTTP 客户端(见 `stdlib/httpx` 与 `gs/gs-http-gen`)运
行时。本身无状态:承载 `Metadata`、应用 `RequestOption`、经 `stdlib/jsonflow` 编
码 body,最终经 `*http.Client` 发出请求(未设置则回落到 `http.DefaultClient`)。

## 特性

- `Metadata`——target/schema/method/pattern/query/body/header/config,加 `Client`
  字段,让生成客户端可携带自己已埋点的传输层。
- `RequestOption` 通过 `CombineMetadata` 组合,内置 `WithHeader`、`WithConfig`。
- 流式 JSON 解码:`ObjectResponse[T]`(T 实现 `DecodeJSON`)与
  `JSONResponse[T]`(泛型 `any`)。
- `QueryStringer` 是 query 编码扩展点;body 若实现 `EncodeForm` 则以表单发送,
  否则流式 JSON。
- `DoRequest` 是 `var`,测试/埋点可整段替换真正的 HTTP 调用。

## 用法

生成客户端直接调用这些 helper;手写调用示例:

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
    Client:  httpClient, // httpx 装配得到的 *http.Client
}
resp, out, err := httpclt.ObjectResponse(context.Background(), &GreetResp{}, meta)
```
