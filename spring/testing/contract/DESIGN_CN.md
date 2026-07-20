# contract Design
[English](DESIGN.md) | [中文](DESIGN_CN.md)

`contract` 是 Go-Spring 版的 Spring Cloud Contract:一份声明式契约(一对
请求 + 它必须返回的响应)同时驱动生产者验证与消费者 stub。它在 stdlib 层,
只依赖 `net/http` / `encoding/json` / `regexp`。

## 1. 职责与边界

- 定义一份 `Contract` 结构(`Name` / `Request` / `Response`),两端都读。
- 提供 `Verify(t, handler, contracts)`——生产者测试,把每条契约的请求真
  发给 `http.Handler`,断言真实响应匹配。
- 提供 `StubServer(contracts)`——消费者测试双,收到请求就照契约里承诺
  的响应答。
- 拒绝造 Go 专属 DSL。契约落盘是 JSON,让 Java 生产者能互操作;偏好 YAML
  的调用方自己反序列化后传 `[]Contract`。
- 拒绝复杂 matcher 语法。请求只按已设字段匹配(空 map / nil body 不约
  束),加上 `Body` 的 JSON 结构相等;响应按 status / headers / body 结
  构相等。

## 2. 关键抽象与缝隙

- **`Contract` = 一个 JSON 文件。** `Verify` 与 `StubServer` 读同一 struct,
  消费者 stub 永远无法编造生产者其实不返回的响应。
- **请求只匹配已设字段。** 空 `Query` / `Headers` = 不约束;`Body` 为 nil
  = 不看 body。让契约只钉关心的部分。
- **body 用 JSON 结构相等**——与断言库同款。不看数值精度、不看 key 序。
- **`StubServer` 返回普通 `*httptest.Server`。** 消费者当普通 HTTP 服务器
  调,不需要专属 client。
- **`FromFS(fsys, glob)` 助手** 从 `fs.FS` 加载契约文件——测试用
  `go:embed` 嵌进来,消费者与生产者读同一份嵌入集合。

## 3. 约束

- **stdlib 零依赖规则。** 不带 YAML parser、gomock、gomega。
- **契约文件是唯一 source of truth。** 两端读同一 JSON;消费者自己写内联
  stub 响应就违背了整个模式。
- **stub 确定性作答。** 多个契约对同一请求形状,取首个匹配。契约作者要让
  请求形状互不重叠。
- **只做 HTTP。** MQ 契约不在此处建模;MQ 测试直接用 `spring/messaging`
  的 in-memory driver。

## 4. 取舍与被否决方案

- **JSON on disk > Go DSL。** Go DSL 与 Java Spring Cloud Contract 生产
  者互操作不了,违背互通前提。
- **stdlib 选 JSON 不选 YAML。** YAML 要 parser 依赖(或手写一个必定跟不
  上规范);偏好 YAML 的调用方自己 unmarshal 后传 `[]Contract`。与
  `spring/i18n` 同样的零依赖姿势。
- **结构相等 > 复杂 matcher。** 真契约要么钉固定 body,要么就别钉。少数需
  正则的场景有 raw body 正则可用。
- **`Verify` 直接跑 `http.Handler`,不起服务器。** 进程内跑得快,同时保
  留生产者挂的中间件;要 live-server 由调用方自己包一层。
