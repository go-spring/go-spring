# registration

[English](README.md) | [中文](README_CN.md)

`registration` 是服务发现的**写侧**,属于控制面:它把本进程发布到已配置的
naming center,并在生命周期节点上维护这份发布。读侧(`Discovery` / `Resolver` /
本地 endpoint 目录)在 [dataplane/kernel/discovery](../../dataplane/kernel/discovery/);业务
请求从不经过本包。

## 为什么在控制面

- 发布是**期望端点态**的生产:本实例的地址、权重、版本、可用区被写入中心,
  供其他进程发现——与 `governance.Source` 获取期望态是同一类动作。
- 它不参与请求路径,而由 `gs.Server` 生命周期驱动:应用 ready 后注册,停机开始
  时(PreStop)反注册,权重变更时广播。
- 各 naming center 的远端适配器在 `starter-discovery-*` 里实现 `Registry`,
  与 `Source` 适配器在 starter 中的模式一致。

## 契约

```go
type Registry interface {
    Register(ctx context.Context, inst Instance) error      // 就绪后调用一次
    Deregister(ctx context.Context, inst Instance) error    // 幂等
    UpdateWeight(ctx context.Context, inst Instance, weight int) error
}
```

`Server` 收集容器中**全部**后端贡献的 `Registry`,统一驱动:任一中心注册失败即
启动失败(消费方视图分裂比启动失败更糟);反注册对每个中心幂等;`UpdateWeight`
让实例在摘流时仍保持可发现。`discoveryServer` 是唯一 `gs.Server` bean,仅在
`${spring.discovery.service-name}` 有值时激活——有值即"发布本进程",无值即纯消费端。

实例身份(`Instance`,配置前缀 `${spring.discovery}.*`)、naming center 配置
(`${spring.discovery.<backend>.<name>}.*`)与消费端引用规则的完整说明见
[discovery README](../../dataplane/kernel/discovery/README_CN.md) 的“配置”一节。

## 可观测性

本包自带 `Observer`(注册操作 span/耗时/计数 + `discovery.instance.registered`
gauge),与读侧的 sync 观测**独立**:两侧各一个 observer,互不 import,因而
`dataplane/kernel/discovery` 不会反向依赖控制面。
