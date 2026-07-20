# starter-http-client 设计

[English](DESIGN.md) | [中文](DESIGN_CN.md)

`starter-http-client` 属于 *client* 形态(见
[starter/DESIGN.md](../DESIGN.md) §2.2,HTTP 场景变体),为
`gs-http-gen` 生成的声明式 HTTP 客户端(Go-Spring 的 OpenFeign /
`@HttpExchange` 等价物)装配运行时传输栈。它**不注册默认单例**——每个
发布的 `*http.Client` 都在 `${spring.http-client}` 下具名,按名注入。

## 1. 职责与边界

- **在范围内:**每个配置项构造一个 `*http.Client`,其 `Transport` 组合
  tracing + 服务发现感知的负载均衡 + 韧性;注册 destroy 释放服务发现 watch
  与 resilience executor。
- **不在范围内:**接口定义与代码生成(`gs-http-gen`);具体负载均衡算法
  (`spring/loadbalance`);韧性策略(`spring/resilience`)。

## 2. Transport 组合——由外到内

`spring/httpx.NewTransport` 装配的链路:

```
resilience  →  discovery + LB(balancedTransport 改写 host)  →  otelhttp base
```

- **otelhttp base 位于最内。**让 span 传播覆盖完整下游路径(含改写后的
  host 与 LB 挑选)。
- **discovery + LB。**`service-name` 非空时:`discovery.MustGet` +
  `LiveDialer` + `loadbalance.Pool`——与其它基础设施客户端同款。否则走
  `fixedHostTransport`(`Addr` 模式)。两者都为空 → 用请求原 host。
- **resilience 位于最外。**让**重试可重挑端点**,且**熔断按逻辑服务名**
  (`req.URL.Host` = 生成客户端的 Target)聚合,而非按端点地址。

## 3. 关键决策

- **只有多实例。**没有默认单例 `*http.Client` bean;加一个实例只是配置
  变更。契合家族规则 §2.2 的 client starter 约束(HTTP 无独立 driver
  注册表)。
- **生成代码只依赖 stdlib。**`gs-http-gen` 输出只 import `net/http` +
  `spring/httpclt`,绝不 import 具体 starter。声明式通过代码生成(无运行
  时代理),代码带 `HTTPClient *http.Client` 缝隙供 DI 填充。
- **`managedTransport` 带 `closeFn`。**starter destroy 把 transport 断言
  为 `io.Closer` 并释放其内的服务发现 watch 与 `resilience.Executor`,
  以避免刷新时移除实例导致 goroutine 泄漏。
- **装配期 fail-fast。**若 ServiceName 无法解析到已注册后端、或 resilience
  driver 未知,`httpx.NewTransport` 返回错误;启动期就暴露,而非首次请求。
- **直连模式下 `Target` 是资源名。**生成客户端的 `Target` 作为熔断
  resource label 出现在日志中,但**不影响路由**——`httpx` 通过改写 host
  完全接管寻址。

## 4. 约束

- **重试要求 `GetBody`。**重试循环重放请求;仅支持一次读取的 body 无法
  重试。
- 内部依赖靠 `go.work`——不跑 `go mod tidy`。
- 顺序要紧:把 resilience 塞进 LB 内部(重试位于 LB 下)会破坏按服务聚合
  的熔断,并让同一端点承接所有重试。

## 5. 取舍 / 弃选方案

- **基于反射的运行时代理——弃选。**Go 没有 Java 动态代理等价物;声明式
  依赖代码生成,让运行时开销与手写客户端一致。
- **resilience 位于 LB 内部——弃选。**重试会粘同一端点;熔断按地址聚合,
  逻辑服务抽象崩塌。
- **共享默认 `http.Client` bean——弃选。**与所有 client starter 弃默认
  单例同理:双注册易错,条件单例语义晦涩。
