# httpsvr Design
[English](DESIGN.md) | [中文](DESIGN_CN.md)

`httpsvr` 为生成的 handler 提供服务端骨架。它位于 `stdlib` 零依赖层:仅用
`net/http` 与仓库内 `jsonflow` / `ctxcache` / `errutil`。

## 1. 职责与边界

- 提供路由缝隙(`Server.Route`),starter 可换任意底层 router 实现;内置
  基于 `http.ServeMux` 的 `SimpleServer`。
- 提供 JSON、SSE 泛型 handler 包装,让 Go 1.22+ 生成 handler 只是薄适配器,而不
  重写解析/校验/编帧。
- 拒绝成为完整 web 框架:无中间件链、无 tag 魔法、无 DI。这些属于 starter 或用
  户代码。

## 2. 关键抽象与缝隙

- **`Server` 接口**——只一个方法 `Route(Router)`。starter 想换 router(chi /
  gin……)实现这一个即可,其他不动。
- **`RequestContext`**——请求/响应对,通过 `WithRequestContext` 存进
  `context.Context`,即使 handler 只拿到 `ctx`(经 `ctxcache` 等)也能取回
  writer。
- **`ReadBody` 与 `ErrorHandler` 变量**——刻意可变,应用可下调 body 上限或改
  错误体格式,而无需包装 `HandleJSON`。
- **`RequestObject` 接口(`Bind` + `Validate`)**——与生成请求类型的契约。
  `ReadRequest` 按 `Content-Type` 选 JSON / form,不识别时用首字节嗅探,让无
  header 的 body 也能解析。

## 3. 约束

- 只对 `POST` / `PUT` / `PATCH` 读 body;其他方法跳过 `decodeBody`,带 body 的
  `GET` 视作无 body。
- `HandleStream` 要求 `http.ResponseWriter` 实现 `http.Flusher`,否则经
  `ErrorHandler` 报 500。包装 writer 时不能丢失 Flusher。
- JSON 路径在 handler 执行前就设 `Content-Type: application/json`,业务
  handler 不会忘设。
- `RequestObject.Bind` 在 body 解码之后运行;body 解码失败直接短路,不会调用
  `Bind`,故 `Bind` 可假定 body 已解码。

## 4. 取舍与被否决方案

- **不做自定义 router。** Go 1.22 的 `http.ServeMux` 已支持方法级 pattern,足
  以承担本包的缝隙职责;引第三方 router 会破坏零依赖约定。
- **不内置中间件切片。** 链式装配属于更高层(`aspect`、`security` 中间件、
  starter 的 `MiddlewareContributor`)。在这里内置会锁死顺序。
- **JSON / form 两条编码路径 + 首字节嗅探。** 更完整的内容协商延后:真实 API
  要么 JSON 要么 `x-www-form-urlencoded`,嗅探覆盖漏设 header 的常见场景。
