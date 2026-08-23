# httpclt

[English](README.md) | [中文](README_CN.md)

`httpclt` 是生成的声明式 HTTP 客户端（代码生成器 `go-spring.org/gs-http-gen`）的运行时。本身无状态：承载 `Metadata`、应用 `RequestOption`、经 `stdlib/jsonflow` 编码 body，最终经 `DoRequest`（可替换的 var，默认 `http.DefaultClient`）发出请求。它位于 `stdlib`，只 import `net/http`、`stdlib/jsonflow` 与少量标准库，让生成客户端保持 stdlib-only，不把 starter 依赖泄给调用方。

## 使用方式

导入路径：`go-spring.org/stdlib/httpclt`。生成客户端直接调用这些 helper；手写调用示例：

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
defer resp.Body.Close() // 默认 DoRequest 已关闭过，重复 Close 安全
fmt.Println(out.Message)
```

### API 列表

| API | 说明 |
|---|---|
| `Metadata` | 一次请求的全部上下文：Target/Schema/Method/Pattern/RawPath/Query/Body/Header/Config |
| `RequestOption` / `CombineMetadata` | 以函数式选项就地修改 `Metadata` |
| `WithHeader(http.Header)` | 向 `Metadata.Header` 合并请求头的选项 |
| `WithConfig(map[string]string)` | 向 `Metadata.Config` 合并配置项的选项 |
| `QueryStringer`（接口） | `QueryForm() (string, error)`——query 编码扩展点 |
| `ResponseObject`（接口） | `DecodeJSON(jsonflow.Decoder) error`——流式解码扩展点 |
| `ObjectResponse[T ResponseObject]` | 用 T 自带的 `DecodeJSON` 流式解码响应 |
| `JSONResponse[T any]` | 用 `jsonflow.UnmarshalRead` 流式解码到任意 T |
| `DoRequest`（var） | 唯一的派发缝隙，整体替换即可注入传输层 |

三个扩展点（`QueryForm` / `EncodeForm` / `DecodeJSON`）都由生成类型实现：
运行时不对业务 struct 做运行时反射，保持快 + 零依赖，对偶职责（类型的编解码
代码）由代码生成器承担。其中 `EncodeForm` 不是命名接口——body 只要实现
`EncodeForm() (string, error)` 方法即走表单编码（见下节）。

`Metadata` 的字段是与 `gs-http-gen` 输出的契约——改字段名即破坏契约，部分字段
仅为契约保留：`Pattern` 由生成器输出但运行时从不读取，运行时只用 `RawPath`
（占位符已处理完的路径）。响应默认走流式 JSON：`ObjectResponse` / `JSONResponse`
经 `jsonflow` 增量解码，不在内存中缓冲整个响应体。

### 一次请求的生命周期

`ObjectResponse` / `JSONResponse` 内部走同一条 `doRequest` 路径，顺序固定：

1. **query**：`meta.Query != nil` 时调 `QueryForm()`，非空则拼到 `RawPath` 后（`?k=v`）；
2. **body**：`meta.Body` 实现 `EncodeForm() (string, error)` 则以表单字符串发送；
   否则（非 nil 时）经 `jsonflow.MarshalWrite` 流式编码为 JSON；
3. **构造请求**：`http.NewRequestWithContext(ctx, Method, RawPath, body)`，然后
   `req.Host = Target`、`URL.Host = Target`、`URL.Scheme = Schema`——也就是说
   `Target` 既是服务名（discovery 场景）也是 `IP:PORT`（直连场景），由被替换的
   `DoRequest` 决定如何解释它；最后合并 `Header`；
4. **派发**：交给 `DoRequest(req, meta, fn)`，由它执行请求并把响应体喂给解码
   回调 `fn`。

### DoRequest 的契约

默认实现是"发送 → 把 `resp.Body` 喂给 `fn` → 关闭 body → 返回 resp"。替换方
拿到的是**已构造好的 `*http.Request`** 加上原始 `Metadata` 和解码回调 `fn`，
因此它掌控完整的请求/响应生命周期：可以换 transport（discovery + 负载均衡）、
短路重试、包 resilience、打日志/指标/追踪，最后只要保证 `fn` 被调用、body 被
关闭即可。`Metadata` 随请求透传，替换方可以读 `Target` 做路由，`Config` 则
用来携带调用侧与替换方之间的额外约定。

传输层、超时、Cookie jar、TLS 配置都不在 `httpclt` 内构造——全部由被替换的
`DoRequest` 决定。这正是 client 侧集成（`starter-http-client`、测试、contract
桩）可插拔而不改生成代码的原因。

实际接线参考 `starter-http-client`：它组装进程级 `dispatchTransport`，按
`req.Host`（即 Target）分派到各自配置好的 transport，整体替换 `httpclt.DoRequest`
——生成代码对此完全无感。discovery、负载均衡、resilience、trace 透传等能力的
组合实现见 `go-spring.org/cloud/experimental/httpx`。

## 许可证

Apache License 2.0，详见 [LICENSE](../../LICENSE)。
