# httpx Design
[English](DESIGN.md) | [中文](DESIGN_CN.md)

`httpx` 为生成的声明式 HTTP 客户端提供运行时传输层。它位于 `cloud` 层:装配器与
它所组合的 `discovery` / `loadbalance` / `governance` 都是 cloud 抽象,可观测
(otelhttp trace、fault 注入、observe 包装)也在本包内实现——生成代码与本包永不
import 具体 starter。

## 1. 职责与边界

- 从声明式 `Config` 装配一个 `http.RoundTripper`,并返回一个 close 函数,负责释
  放 `discovery` watch 与 `resilience` executor。
- 三种寻址模式由 config 二选一:discovery + LB 池、直连 `Addr`、以及不干预
  (透传生成客户端设置的 host)。
- 可观测内置:`otelhttp` 包住底层传输(client span、trace 传播);启用 resilience
  时执行器默认包成 `observe(fault(raw))`——fault 注入在最内层,注入的故障看起来
  与真实下游失败一致,observe 记录最终结果。无 OTel SDK / 未注册 injector 时均
  为 no-op。
- 默认受治理保护:未显式给执行器时,从集中治理中心按 `Resource` 解析执行器
  (error-rate 熔断施加 `min-requests=5` 下限);治理未启用时为透明 pass-through。
- 构建 TLS 面:启用的 `tls.*` 块接进底层传输的 TLS 配置;显式 `Base` 则完全接管
  拨号。
- 拒绝介入传输层以上。Cookie jar、请求体缓冲以便重试、为追踪加 header 都不在这
  里做;交给 client 或调用方传入的 `Base` 传输层。

## 2. 关键抽象与缝隙

- **`http.RoundTripper`**——唯一缝隙。`otelhttp`、`resilience`、`httpx` 都以它
  为契约,埋点与保护无胶水即可环绕负载均衡。
- **`Config.ServiceName` 与 `Config.Addr`**——两种寻址模式内部收敛到同一形状
  (`balancedTransport` vs `fixedHostTransport`),让上层代码路径完全一致。
- **`Base`**——裸底层传输(TLS 定制 clone、自定义 dialer);`httpx` 自己在其上
  叠 otelhttp trace。
- **`WrapExec`**——替换默认 `observe(fault(...))` 执行器包装的逃生舱,供需要自
  定顺序或额外层的调用方使用。

## 3. 约束

- Resilience 包住整条链,处于 balancer **之外**。故重试会重新挑端点(failover
  需要),熔断按逻辑服务名(生成客户端设置的 `Host`)归键,而非按物理端点。永远
  不要把 resilience 层挪到 balancer 之内。
- `balancedTransport.RoundTrip` 改写 `URL.Host` / `Host` 前必须 Clone 请求:
  net/http 可能重试,上层 resilience 也会跨尝试复用原请求。就地改写调用方请求
  是正确性 bug。
- `NewTransport` 对未知 discovery/balancer 名字立即失败。让首次请求时才发现
  wiring 错误远比启动期报错更糟。

## 4. 取舍与被否决方案

- **不做 YAML / 反射 / 注解。** 声明式行为由 `gs-http-gen` 生成;运行时魔法有
  意规避,因为 Go 没有代理机制,基于反射的客户端每次调用都付代价。
- **可观测内置而非由 starter 注入。** cloud 本就依赖 otel(observe kit),把
  trace/metric/log 挡在装配器之外没有任何收益,反而迫使每个消费方重复接同一套
  栈。starter 只做配置,cloud 负责实现。
- **不暴露可自由组合的 "chain" API。** 四段顺序(resilience → balancer →
  otel-base → net/http)是有意固定的;暴露成用户可拼装的 chain 会诱使把
  resilience 装到 balancer 之下,重试时失去 failover。
