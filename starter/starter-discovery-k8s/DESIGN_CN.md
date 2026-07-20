# starter-discovery-k8s 设计

[English](DESIGN.md) | [中文](DESIGN_CN.md)

`starter-discovery-k8s` 属于 Config-provider 形态（`starter/DESIGN.md` §2.5）
的 `spring/discovery` 贡献者：把 K8s Service 名解析成活 Pod 端点的
`discovery.Discovery` 命名后端。

## 1. 职责与边界

- 把 `spring.discovery.k8s.<name>` 条目绑定到 `discovery.Discovery` 后端，
  每条一个，注册到进程级 `spring/discovery` 注册表下 `<name>` 名字。
- Discovery 后端**不是可注入 bean**。客户端 starter（redis / gorm / grpc）
  用 `discovery: <name>` 字段引用，靠 `discovery.MustGet` 反查。
- 注册一个 lifecycle bean（`manager`），仅为在容器关闭时清理后台 informer
  goroutine。
- 有意只做 client 侧：无 controller、无 CRD、不往注册中心推。K8s 本身是
  权威。

## 2. 关键抽象与缝隙

- **在任何 client bean 前注册。** 注册在 `gs.Module` 回调（bean 注册阶段）
  执行，框架保证它在任一 client bean 构造函数之前跑——所以 redis/gorm
  客户端调 `discovery.MustGet` 时不会跟注册表打架。
- **两种模式共用同一缝隙。** `Mode=dns` 走 headless Service DNS（SRV/A）
  ——零依赖、无 RBAC——由于 DNS 无 push 通道，靠周期重解析。
  `Mode=endpointslice` 用 client-go informer 监听 EndpointSlices，实时更新
  且带端点元数据（zone/ready），代价是 endpointslices 的 get/list/watch
  RBAC。
- **名字冲突启动失败。** 若某名字已被占用（例如公司自己的
  `discovery.Register`），启动即拒绝，不静默覆盖。
- **`Close` 可选。** `manager.Stop` 仅对实现 `io.Closer` 的后端调 `Close`；
  DNS 模式无资源需释放，只有 informer 后端需要 shutdown。

## 3. 约束

- **DNS 模式需要端口信息。** SRV 模式需 `port-name`；A 记录模式需
  `port`（A 记录不带端口）。两者都缺在 `validate` 被拒绝。
- **cluster-domain 仅 DNS 生效。** `cluster-domain`（默认 `cluster.local`）
  塑造 DNS 模式 Service FQDN，endpointslice 模式忽略。
- **集群内 vs kubeconfig。** `Kubeconfig` 为空时用 in-cluster ServiceAccount
  配置（K8s 部署路径）；提供 kubeconfig 路径时通过该文件拨号（本地
  开发/测试）。
- **本地单测不接真集群。** endpointslice 冒烟走 fake clientset；DNS 模式
  注入 resolver。

## 4. 权衡 / 已否决方案

- **推入 K8s 服务端注册——否决。** 平台已经把每个 Pod 通过 EndpointSlices
  登记好；再加一个 registrar 只会重复且失步。
- **发现能力烧进每个 client starter——否决。** 缝隙是 `spring/discovery`
  注册表；客户端按名查表，DNS/EndpointSlice/Nacos 之间互换不动 client。
