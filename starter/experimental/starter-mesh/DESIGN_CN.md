# starter-mesh Design

[English](DESIGN.md) | [中文](DESIGN_CN.md)

`starter-mesh` 属于 global / infrastructure 形态(见
[starter/DESIGN.md](../DESIGN.md) §2.4),仅用于翻转客户端 discovery + 负载
均衡使用的**进程级 mesh 开关**。它落在 *starter* 层是为了让"接线动作"位于
集成层;真正的退化逻辑与开关的**唯一真源**留在 `spring/discovery`。

## 1. 职责与边界

- **在范围内:** 绑定 `${spring.mesh}`,在 `RefreshPrepare` 阶段调用
  `discovery.SetMeshMode(cfg.Enabled)`,确保任何 bean 构造前开关就位。
- **不在范围内:** 控制面对象(VirtualService、DestinationRule)与流量治理
  API——那属于部署脚手架,不该在本 starter。Trace、metrics、readiness 语义
  也不动;mesh 模式只影响客户端拨号与 Pick 路径。

## 2. 关键决策——单开关,集中退化

开关是 `spring/discovery` 内的单个进程级 `atomic.Bool`,不做逐 starter 分支。
每一处客户端 seam 只在两个位置读取它:

- `discovery.NewClientDialer` / `NewLiveDialer`:mesh 开 → 构造 `meshDialer`,
  仅返回单个稳定端点 `{Addr:name, Healthy:true}`,不 Resolve、不 Watch、也不
  开后台 goroutine。Kubernetes DNS 把 name 解析成 Service ClusterIP,由 sidecar
  拦截。
- `loadbalance.Pool.Pick`:mesh 开 → 直接返回 `eps[0]`,`Done` 是空函数。
  **有意跳过 Tracker**:唯一的 mesh 端点绝不能被剔除为不健康——否则会把全部
  流量打黑洞。

starter 本体不含退化逻辑,只是把配置桥接到 `SetMeshMode` 的一根线。

## 3. 约束

- 用 `gs.Module(nil, setup)`,让 `RefreshPrepare` 早于 bean 构造运行——每个
  client starter 的构造函数都必须看到已生效的开关。
- 无论 `enabled=true` 还是 `false` 都 apply,以便配置刷新时能清干净遗留"开态"。
- Client starters 应通过 `discovery.NewClientDialer`(如 `starter-go-redis`)取
  拨号器,才能在不触碰 registry 的情况下响应开关。仍直接调 `NewLiveDialer` 的
  starter 也会退化,但 mesh 下仍需要已注册的后端。
- `go.mod` **有意不跑 `go mod tidy`**:spring/log/stdlib 内部依赖靠 `go.work`
  解析,tidy 会去 proxy 404。

## 4. 取舍 / 弃选方案

- **每 starter 各自开关——弃选。**N 个开关需要同步,且 redis 与 gorm 同时
  看错开关时的协调 bug 真实存在;集中一处开关翻一次即可。
- **控制面集成——弃选于此。**VirtualService 生成属 `gs k8s` 部署脚手架;混在
  一起会把 Kubernetes API 依赖拖进每个 client starter。
- **mesh 开时删掉 LB 代码——弃选。**退化仅是运行时行为;关掉开关即恢复完整
  客户端 discovery + LB,无需重新编译。

## 5. 何时启用

- **开启**:pod 中注入了 Istio/Envoy 或 Linkerd sidecar——否则应用会在 sidecar
  之上做二次负载均衡,干扰 locality 与离群剔除。
- **关闭**(默认):VM、裸机、无 mesh 的 Kubernetes;继续走客户端 discovery+LB
  作为主路径。
