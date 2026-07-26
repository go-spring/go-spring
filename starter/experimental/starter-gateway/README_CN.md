# starter-gateway

[English](README.md) | [中文](README_CN.md)

`starter-gateway` 是一个独立的 API 网关，运行在自己的监听端口上（默认 `:9440`），
与业务应用同进程共存。它将 Spring Cloud Gateway 的 **Route / Predicate / Filter**
模型带入 Go-Spring，但用 Go 的惯用法而非运行时 DSL 表达：断言就是一个
`func(*http.Request) bool`，过滤器就是一个 `func(next http.Handler) http.Handler`，
路由即普通的函数组合。

路由完全通过 `spring.gateway.routes.<id>` 配置声明，并且**支持热更新**——任何标准的
配置刷新（`starter-config-file` 卷监听、`starter-config-nacos` 等）都会重建已编译的路由
表，无需任何网关专属机制。编译失败的路由会保留上一份路由表，因此一次错误的改动绝不会
让网关宕机。

上游分为**直连**（`http(s)://host:port`）与**服务发现**（`lb://<service>`）两类，后者复用
`spring/discovery` + `spring/loadbalance`。转发经由 `spring/resilience` 完成重试／熔断／
限流，网关还会向 actuator 管理端口贡献 `/gateway/metrics`。

## 安装

```bash
go get go-spring.org/starter-gateway
```

## 快速开始

### 1. 导入 `starter-gateway` 包

路由是纯配置，无需任何应用 bean，匿名导入即可。参考 [example.go](example/example.go)。

```go
import _ "go-spring.org/starter-gateway"
```

### 2. 在配置中声明路由

在项目的[配置文件](example/conf/app.properties)中添加路由配置。每条路由包含一个 id、
一组断言、可选的过滤器链，以及一个上游目标：

```properties
spring.gateway.server.addr=:9440

spring.gateway.routes.api.predicates.path=/api/**
spring.gateway.routes.api.filters=stripPrefix(1),addRequestHeader(X-From,gw)
spring.gateway.routes.api.upstream.target=http://127.0.0.1:19000
```

### 3. 运行

网关以 `gs.Server` 形态在其监听端口启动。请求 `http://127.0.0.1:9440/api/echo` 会被
`api` 路由匹配，剥掉 `/api` 前缀，注入 `X-From: gw` 头，再转发给上游。未匹配的路径由网关
自身返回 `404`。

## 断言（Predicates）

一条路由上的所有断言以逻辑**与**组合；没有断言的路由即 catch-all。声明于
`spring.gateway.routes.<id>.predicates.*`：

| 键 | 含义 | 示例 |
| --- | --- | --- |
| `path` | ant 风格路径模式（`*` 单段，`**` 任意） | `/api/orders/**` |
| `methods` | 逗号分隔的 HTTP 方法列表 | `GET,POST` |
| `host` | 精确 host 或 `*.suffix` 通配 | `*.example.com` |
| `headers` | `K:V;K2:V2`，全部必须满足 | `X-Env:prod` |
| `queries` | `k=v;k2=v2`，全部必须满足 | `version=2` |
| `after` | RFC3339 时间，仅在此之后匹配 | `2026-01-01T00:00:00Z` |

## 过滤器（Filters）

过滤器按声明顺序由外向内包裹代理处理器，列于
`spring.gateway.routes.<id>.filters`，形如 `name(args...)`：

| 过滤器 | 作用 |
| --- | --- |
| `stripPrefix(n)` | 去掉前 `n` 个路径段 |
| `prefixPath(p)` | 追加固定路径前缀 |
| `addRequestHeader(k,v)` / `setRequestHeader(k,v)` / `removeRequestHeader(k)` | 修改请求头 |
| `addResponseHeader(k,v)` / `setResponseHeader(k,v)` / `removeResponseHeader(k)` | 修改响应头 |
| `rewriteHost(h)` | 覆盖发往上游的 Host 头 |
| `preserveHostHeader` | 保留入站 Host 而非替换为上游 Host |
| `requestId([header])` | 确保存在 `X-Request-Id`（或指定头） |
| `rateLimit(rate=..,burst=..,...)` | 经 resilience RateLimiter 限流 |
| `jwt-auth(<bean>)` / `lua(<bean>)` | 委派给 bean 形态的 `FilterWrapper`（见下） |

用 `StarterGateway.RegisterFilter(name, factory)` 注册自定义过滤器。

## 上游与负载均衡

* **直连**：`upstream.target=http://host:port` 直接转发到该地址。
* **服务发现**：`upstream.target=lb://<service>` 经 `spring/discovery` 解析活跃实例，
  由 `spring/loadbalance` 选择其一。可设置 `upstream.balancer`（`round_robin`、
  `least_conn`、`consistent_hash`、`weighted`）与 `upstream.discovery`（后端名），
  或通过 `spring.gateway.discovery` 设置网关级默认发现后端。

## 韧性（Resilience）

`spring.gateway.resilience.<name>` 下的命名策略与 `resilience.Policy` 一一对应（限流速率、
突发、错误阈值、熔断打开时长、最大并发、重试次数、超时）。路由通过
`resilience.policy=<name>` 引用；引用同一策略的路由共享熔断／限流状态。

## 可观测

* `/gateway/metrics` 以 Prometheus 文本形式贡献到 actuator 管理端口：按状态类别统计的
  各路由请求数、在途请求数，以及路由重载错误数。
* 名为 `gateway` 的 `health.Indicator` 只要路由表已加载即报告 UP。

## 核心特性

[example.go](example/example.go) 演示并断言：

* **路由 + 断言匹配** —— `/api/**` 被正确路由，未匹配路径干净地返回 `404`，不接触任何上游。
* **过滤器链** —— 在上游看到请求之前剥掉 `/api` 前缀并注入 `X-From` 头。

## 高级特性

* **热更新** —— 路由通过 `gs.Dync` map 绑定；任何配置刷新都会在下一次请求时重建已编译路由
  表。编译失败的路由会被拒绝，上一份路由表继续服务。
* **TLS / mTLS** —— 设置 `spring.gateway.server.tls.enabled=true` 并配 `cert-file`/`key-file`；
  再加 `ca-file` 即开启双向 TLS（客户端必须出示证书）。
* **优雅停机** —— 网关是 `gs.Server`，因此提前监听（端口冲突在启动即失败）、就绪后才对外
  服务、停机时排空在途请求。
* **bean 形态过滤器** —— `jwt-auth(<bean>)` 与 `lua(<bean>)` 解析一个导出为
  `gateway.FilterWrapper` 的 bean（单方法 `Wrap(next) handler` 缝隙），让
  `starter-security-jwt` 与 `starter-lua-filter` 无耦合地作为过滤器接入。
