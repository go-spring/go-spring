# starter-oauth2-client 设计

[English](DESIGN.md) | [中文](DESIGN_CN.md)

`starter-oauth2-client` 属于 **Contributor** 形态(见
[starter/DESIGN.md](../DESIGN.md) §2.3),发布开箱即用的 `*http.Client` 与
`oauth2.TokenSource` bean——它们代替调用方自动获取 / 刷新 OAuth2 token。
不开端口;生成的 client 注入到应用调用受保护上游的地方。

## 1. 职责与边界

- **在范围内(v1):**OAuth2 的*客户端*侧。两种 grant:`client_credentials`
  (`*http.Client` + `oauth2.TokenSource` 同名双 bean;`EndpointParams` 支持
  Auth0 audience / Azure resource)与 `authorization_code`(`*oauth2.Config`)。
- **不在范围内:**资源服务器 / token *校验*——那在 `starter-security-jwt`。
  共享校验核心的分层(stdlib 侧还是 web 框架绑)暂不决定,等真需求出现。

## 2. 关键决策

- **只有多实例,经 `gs.Group`。**`${spring.oauth2.client}` 每项一个具名
  `*http.Client`;加上游是纯配置变更。`destroy` 传 `nil`,因为
  `*http.Client` 没有 `Close()`。
- **同前缀,不同 bean 类型。**同一 `${spring.oauth2.client}` 分组下还注册
  同名的 `oauth2.TokenSource` bean(bean 唯一性=类型+名)。gRPC metadata、
  WebSocket 握手等需要裸 token 的场景直接注入 source,无需另开前缀。
- **`authorization_code` 独立前缀。**`${spring.oauth2.authcode}` 发布
  `*oauth2.Config`。不同 grant 视为不同能力——分前缀让两种表面独立配置,
  避免 client-credentials 字段与 auth-code 字段互相污染。
- **`EndpointParams` 是 `map[string]string`。**`conf` 支持 map 绑定
  (`${...:=}` 空默认合法),再转 `url.Values` 传给
  `clientcredentials.Config.EndpointParams`——Auth0 `audience` /
  Azure `resource` 因此免代码改动可用。
- **韧性在这里作最外层。**starter 把 `stdlib/resilience` 作为 transport
  的最外层包装(重试可重挑、熔断按逻辑名聚合)。开启是同一实例内的纯
  配置开关。

## 3. 约束

- **token URL / client id 缺失时 fail-fast。**必填字段空即启动错,不做
  "半可用默认"。
- **外部依赖仅 `golang.org/x/oauth2`**——内部 spring / log / stdlib 靠
  `go.work`。企业内网拉 `x/oauth2` 曾需
  `GOPROXY=https://goproxy.cn,direct`;此模块靠 std 邻近但内网 proxy 未
  vendor。

## 4. 取舍 / 弃选方案

- **默认单例 `*http.Client`——弃选。**同 client starter 家族规则 §2.2:双
  注册易错,条件单例语义晦涩。
- **把 `client_credentials` 与 `authorization_code` 合并为同一 bean 类型
  ——弃选。**两者产出的类型不同(`*http.Client` 与 `*oauth2.Config`),
  字段也不同;分前缀让两个表面各自清晰。
- **资源服务器校验放本 starter——弃选。**属于*服务端*(`starter-security-jwt`
  + `stdlib/security`),不该混进客户端配置表面。
