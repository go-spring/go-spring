# starter-registry-k8s 设计

[English](DESIGN.md) | [中文](DESIGN_CN.md)

`starter-registry-k8s` 属于 Config-provider 形态（`starter/DESIGN.md` §2.5）
的 `cloud/discovery` 贡献者：把 K8s Service 名解析成活 Pod 端点的
`discovery.Discovery` 命名后端。它是 `starter-registry-*` 家族里的只读成员。

## 1. 职责与边界

- 把一个 `spring.registry.k8s.<name>` 命名块绑定成一个 `discovery.Discovery`
  bean，bean 名为 `k8s.<name>`，与家族其他后端（`consul.<name>`、
  `etcd.<name>`……）一致。跨后端的块永不冲突，因为 bean 名带着后端类型。
- Discovery 后端**按 bean 名引用**。客户端 starter（redis / gorm / grpc）用
  `discovery: k8s.<name>` 字段引用其中一个，容器按名注入该 bean；不存在另一张
  需要查的注册表。
- **不提供 registrar。** 与家族其他成员不同，本后端从不把本进程发布到任何地方，
  因此家族级注册键（`${spring.registry.service-name}` / `.addr`）在这里没有意义，
  也不被读取。只配了 k8s 块却又设置它们，会在 registry 核心处以
  “no registry center is configured” 启动失败——这正是期望中的响亮信号。
- 有意只做 client 侧：无 controller、无 CRD、不往注册中心推。K8s 本身是权威。

## 2. 关键抽象与缝隙

- **早于任何 client bean 声明。** 块到 bean 的映射在 `gs.Module` 回调（bean
  注册阶段）里建立，框架保证它在任一 client bean 构造函数之前跑——所以
  redis/gorm 客户端可以在配置里直接引用 `k8s.<name>`，不会与声明打架。构造本身
  推迟到注入时，因此没人引用的后端永远不会被实例化。
- **两种模式共用同一缝隙。** `Mode=dns` 走 headless Service DNS（SRV/A）
  ——零依赖、无 RBAC——由于 DNS 无 push 通道，靠周期重解析。
  `Mode=endpointslice` 用 client-go informer 监听 EndpointSlices，实时更新
  且带端点元数据（zone/ready），代价是 endpointslices 的 get/list/watch
  RBAC。
- **名字冲突启动失败。** 两个块解析出同一 bean 名时由容器拒绝，不静默覆盖。
- **关闭即 bean 析构。** 后端以 `Destroy` 提供；仅对实现 `io.Closer` 的后端调
  `Close`。DNS 模式无资源需释放，只有 informer 后端需要 shutdown。

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
- **发现能力烧进每个 client starter——否决。** 缝隙是命名后端 bean；客户端按名
  引用，DNS/EndpointSlice/Nacos 之间互换不动 client。
- **自占 `spring.discovery.*` 命名空间——否决。** 这会让同一概念在家族里有两套
  配置根和两套 bean 命名法；本后端是 registry 家族里的只读成员，不是独立家族。
