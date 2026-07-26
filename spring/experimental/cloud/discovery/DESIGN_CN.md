# discovery 设计
[English](DESIGN.md) | [中文](DESIGN_CN.md)

`discovery` 位于 stdlib(零依赖基础层),定义每家命名服务需要满足的最小契约,
并沉淀所有基础设施客户端共用的 dialer 机制:一次适配、Redis / MySQL /
MongoDB / Kafka / gRPC 通吃。

## 1. 职责与边界

- **做:** 定义 `Discovery`(Resolve / Watch)、`Endpoint`、`LiveDialer`
  (冷启动快照 + 后台 Watch + 轮询拨号)、服务网格降级开关、以及配套的
  `Registrar`(本进程注册)。
- **不做:**
  - 不做 RPC 框架 provider 侧注册(kitex / kratos / dubbo-go 等)。每个框架
    已有各自的注册模型,再套一层只会变成翻译层 —— 是负价值;详见
    `starter/DESIGN.md` §3。
  - 不做负载均衡策略。`LiveDialer.Pick` 仅是最简 round-robin;真正的策略
    (weighted / least-conn / consistent-hash / zone-aware)在
    `go-spring.org/spring/loadbalance`,它位于 discovery 之上。
  - 不做具体后端。Nacos / Consul / etcd / DNS / Kubernetes 各自在 starter
    里实现并按名注册。

## 2. 关键抽象与缝隙

- **两方法 `Discovery` 接口**(Resolve + Watch)让适配面尽量小。不支持 streaming
  的后端可在内部轮询,契约仍统一。
- **包级后端注册表 + init 期 panic**,与 `resilience`、`cache` 同构。重复 /
  空名 / nil 注册是接线 bug,不是运行期状态。
- **`LiveDialer` 是共享 dialer 表面。** `DialContext(ctx, network, addr)` 匹配
  Redis 和 pgx;`Dial(ctx, addr)` 匹配 go-sql-driver 和 ClickHouse;它也直接
  满足 mssql `Dialer` 接口。客户端把服务名当 `Addr` 传进去,dialer 忽略并连接
  当前选中的 endpoint。
- **`Registrar` 是 `Discovery` 的写侧对偶。** 面向平台不替你注册的场景
  (VM / 裸机 / 混合部署);Kubernetes 已替每个 Pod 注册,故那里不用
  `Registrar`。`Registrar` 与 `Discovery` 注册表共享同一把锁 —— 两者都在 init
  期填充、查找之前不会并发写。
- **网格开关在中心处一次读取:** `NewLiveDialer` / `NewClientDialer` /
  `loadbalance.Pool.Pick`,而不是每个 starter 各自判断,降级动作全局统一。
  mesh 模式下 dialer 暴露单一稳定端点(服务名 / ClusterIP),把 discovery 与
  LB 交给 sidecar。

## 3. 不变量

- 后端与 dialer 必须并发安全;`LiveDialer` 用 `atomic.Pointer[[]Endpoint]`
  存快照、`sync.Once` 保护 `Stop`,支持 Stop 与 dial 并发。
- `Pick` 优先返回 `Healthy`;若**无**任何 endpoint 标 healthy(后端不上报健康
  时),全部视为可选 —— discovery 绝不能因后端没上报健康就黑洞流量。
- `Watcher.Next` 阻塞到下一份快照或 stop/error;错误后 watch loop 退出 ——
  调用方靠上一份快照兜底,不要在 loop 内重试。
- 网格开关只在启动时设一次,先于任何 dialer 建立;运行期切换不支持。
- 这里不加 provider 注册 seam。`Registrar` 面向传输无关的实例注册;RPC 框架
  自己的 provider 注册留在框架各自的配置里。

## 4. 权衡与放弃的方案

- **只做 client 侧。** 拒绝为 RPC 框架统一一个 `Registrar`:kitex
  `registry.Registry`、kratos `registry.Registrar`、dubbo-go 配置化注册、
  go-zero `discov.EtcdConf` 差异足够大,再套一层就是翻译;能用框架原生的
  就用原生。只有当框架完全没有原生机制(裸 gRPC / thrift / HTTP)才启用
  `Registrar`。
- **`LiveDialer.Pick` 只做最简 round-robin,不做 weighted / 一致性哈希。**
  策略归上一层;discovery 保持窄职责,避免与 `loadbalance`(策略 + 摘除)
  职责重叠。
- **网格开关是进程级 atomic,不是每客户端各配一遍。** 网格是基础设施决定
  ——有没有 sidecar,而不是每客户端配置项 —— 故一次开、所有客户端一起降级。
