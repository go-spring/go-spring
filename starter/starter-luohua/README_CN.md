# Starter Luohua

[English](README.md) | [中文](README_CN.md)

**公司级聚合（伞包）starter**（*聚合/profile* 原型）：把 go-spring 再基线到一家假想公司的约定上——
通过既有 go-spring 标准组件的**公开扩展缝**来接线它们，绝不重新实现。

空白导入并配置 `spring.luohua.*`：

```go
import _ "go-spring.org/starter-luohua"
```

准则——*标准组件 + 公司自定义位*：luohua 提供的每个默认，背后都有一个显式缝
（配置键 / `Register*` 驱动注册表 / `OnMissingBean` 默认），让"用了标准组件但想在关键位置自定义"的团队能改。
若某 luohua 默认必须走私有路径才能生效，错的是缝，不是默认。

## 目前再基线了什么

### wire 词表 — `spring.luohua.propagate.*`

| 键 | 默认 | 含义 |
| --- | --- | --- |
| `enabled` | `true` | 整个基线的总开关。 |
| `propagate.load-test-header` | 空 | 覆盖 `traffic.HeaderLoadTest`，让压测标识的探测/注入用 luohua 自己的 header（G1 缝）。gRPC metadata 键由小写派生。 |
| `propagate.headers` | 空 | luohua 具名 header propagator 全链路携带的业务头（`X-Tenant`、`X-User`…）。它们挂在 OTel 全局 propagator 上，httpx / gin / echo / grpc 全部生效。 |
| `observability.fields` | 空 | luohua 在每条日志打印的上下文字段（与 `propagate.headers` 同名，如 `X-Tenant`）。 |

具名 header propagator 以名字 `luohua` 注册进 `starter-otel/trace`（G2 缝）。要让它跑起来，
把它复合进全局 propagator：

```properties
spring.observability.trace.propagator=w3c,luohua
spring.luohua.propagate.headers=X-Tenant
```

只注册没意义——没人列它的名字就不会运行。

### 身份 — `spring.luohua.identity.*`

| 键 | 默认 | 含义 |
| --- | --- | --- |
| `secret` | 必填 | 共享 HMAC 密钥。存在即装配一个 `LuohuaSSO` `security.TokenValidator` bean。 |
| `issuer` | `luohua` | 期望的 token 签发者。 |

`LuohuaSSO.Issue(subject, tenant, authorities, ttl)` 铸造 token(供测试/example);
`Validate` 直接接入各族的 `Authenticate` / `Authorize` 安全壳。自带 OIDC/JWT 验签?
提供你自己的 `security.TokenValidator` 即可——luohua 从不特判自己的实现。

### 错误词表 — `spring.luohua.i18n.*`

| 键 | 默认 | 含义 |
| --- | --- | --- |
| `default-locale` | `zh` | 词表兜底语言。 |

luohua 一套词表以 `i18n.MessageSource` 默认提供,`OnMissingBean` 让步——app 自带即覆盖。

### 标准缓存 — `luohua`

luohua 以统一公司名 `luohua` 提供一个进程内、带 TTL 的 `cache.Cache` bean：

```go
Cache *cache.Cache `autowire:"luohua"`
```

该 bean 无配置门控：无人注入即不实例化。

## 治理

luohua 刻意**不造自己的治理引擎** —— 出站调用已走 go-spring 中性的 `resilience.ExecutorFor` /
`fault.InjectorFor` 缝、挂在单一治理权威下;没有自研后端的公司应骑官方引擎、按舰队钉默认策略。
写在治理规则文档里即可(见 starter-governance):

```properties
govern.driver=default
govern.rules[0].resources=orders
govern.rules[0].timeout=500ms
govern.rules[0].max-retries=2
```

要注册公司自己的 resilience 后端(替换 `default`)是 fork 本 starter 后的一行 `resilience.RegisterDriver(name, …)` ——
缝在,luohua 只是不假装造一个引擎。

## 如何启用

任何 `spring.luohua.*` 键都会装配该模块；`spring.luohua.enabled=false` 静默整个基线。
能力组合既有 starter，且 `OnMissingBean` 让步——app 显式给 bean/键即覆盖，luohua 不与公司 bean 争抢。

## 你保留的自定义位

luohua 碰到的每个东西都可覆盖：压测头是包级变量可直接重赋；propagator 列表是配置；log 钩子是**组合**而非替换旧的。
要更严格的顺序，就在 luohua 之后装你自己的 `log.FieldsFromContext`。

## 完整基线（`app.properties`）

一个服务**空白导入 `starter-luohua` 和 `starter-otel`**（otel starter 拥有 Tracer/Meter provider 与上面组合成的
propagator），再用一块配置武装整个基线：

```properties
# otel：离线不需要 exporter，但要传播 trace + luohua 的 header
spring.observability.trace.exporter=none
spring.observability.metrics.enable=false
spring.observability.trace.propagator=w3c,luohua

# luohua 基线
spring.luohua.enabled=true
spring.luohua.identity.secret=replace-me
spring.luohua.identity.issuer=luohua
spring.luohua.i18n.default-locale=zh
spring.luohua.propagate.load-test-header=X-Luohua-LoadTest
spring.luohua.propagate.headers=X-Tenant,X-User
spring.luohua.observability.fields=X-Tenant
```

`spring.luohua.propagate.headers` 指定 `luohua` propagator 携带的业务 header（它只有在
`spring.observability.trace.propagator` 里出现 `luohua` 时才生效）；
`spring.luohua.observability.fields` 指定其中哪些也打印到每条日志。
