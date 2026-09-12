# starter-registry-consul 使用说明

精简版(client 家族格式)。概览见 [README.md](README.md)。锚定 [example/](example/)(docker-compose +
`check.sh`)。**Consul 语义(健康检查、TTL、catalog)见 [Consul 官方文档](https://developer.hashicorp.com/consul/docs)**
——本文只写绑定面。

**模型**:配置是**命名块**——每个 `spring.registry.consul.<name>.*` 块描述一个 Consul
agent,成为名为 `consul.<name>` 的后端 bean(`center.go`)。该 bean 同时实现命名体系的
两半:`discovery.Registrar`(写侧——由传递依赖自动引入的
[starter-registry](../starter-registry) 核心之 `registryServer` 收集,跨后端注册进
**每一个**已配置中心)与 `discovery.Discovery`(读侧——消费方按 bean 名引用,如
`discovery=consul.main`;bean 惰性)。读写共享该块的客户端,永不分裂。没有默认/无名块。
注册仅在设置 `spring.registry.service-name` 后激活——纯消费方应用只配连接块、不注册任何
实例。多中心(双注册、跨后端混搭)就是多配几个块。

## 1. 快速开始

前置依赖:一个 Consul agent(example 自带 docker-compose)。

```properties
spring.registry.consul.main.address=127.0.0.1:8500
spring.registry.service-name=orders
spring.registry.addr=10.0.0.1:8080
```

blank-import + `gs.Run()`。注册器用 TTL 心跳保活;`PreStop` 反注册。

## 2. 全量配置参考

### `spring.registry.consul.<name>.*`(连接,每块 7 key)

块名自选,成为后端 bean `consul.<name>`;任一块存在即激活模块。

| Key | 类型 | 默认值 | 说明 |
|-----|------|--------|------|
| `address` | string | — | 每块必填 |
| `scheme` | string | http | |
| `datacenter` | string | ""(用 agent 的) | |
| `token` | string | "" | |
| `namespace` | string | ""(Enterprise) | |
| `ttl` | duration | 15s | 心跳间隔 |
| `deregister-critical-after` | duration | 1m | 0 = 关闭;agent 卡死时可见性很快消失 |

### `spring.registry.*`(实例,5 key)

| Key | 类型 | 默认值 | 说明 |
|-----|------|--------|------|
| `service-name` / `addr` | string | "" | 注册时启动期必填;`service-name` 即注册意图信号 |
| `id` | string | 派生 `ServiceName-Addr` | ⚠ 同 addr 不同服务会撞 ID |
| `weight` | int | 100;负数存为 1 | 0 = 摘流,Register 与 `UpdateWeight` 同语义 |
| `metadata` | map | 空 | |

### 发现 —— 按块 bean 名引用(零配置)

发现**无需任何自有配置**:每个块的后端 bean 本身就是 `cloud/discovery.Discovery`,名为
`consul.<name>`(`center.go`),client 按 bean 名引用(`discovery=consul.main`)。
该 bean 惰性——纯 provider 从不解析、从不付费;纯消费方只配连接块(不设
`service-name`/`addr`),不注册任何实例。多 agent 发现就是多个块:引用哪个块就从哪个
agent 读。调用侧 `discovery.WithTag` 仍可逐次按 Consul 服务标签收窄查询。

解析语义:只查 **passing** 实例(不健康不进快照);每个已解析服务一条后台 blocking
query(index 长轮询)保鲜,后续 Resolve 读缓存;`Meta["scheme"]` → `Endpoint.Scheme`,
`Weights.Passing` → 权重,无服务地址时回退节点地址。

## 3. bean 与运维

- 每块一个后端 bean `consul.<name>`(共享该块 `*api.Client`;Consul 客户端无需关闭,
  Destroy 只停 discovery 半边的后台查询)。
- `registryServer` 来自 [starter-registry](../starter-registry) 核心(传递依赖),
  `Registrars []discovery.Registrar` 切片收集所有后端的 registrar。
- `UpdateWeight(ctx, w)` 以 `Weights.Passing = w` 逐中心 upsert(0 = 摘流透传;未注册时报错)。
- 运行期日志带专属 tag `_app_registry_consul`（`log.RegisterAppTag("registry_consul", "")`），
  经 `logger.<name>.tag=_app_registry_consul` 单独调级（与 registry-etcd/nacos 一致）。
- 可观测性：注册与发现经 OTel 全局产出指标——未引入 `starter-otel` 时全部 no-op。`register`、`deregister`、`update_weight` 各有一个 client span 与一条 `registry.operation.duration`（标签 `system`/`operation`/`service`/`status`）；`registry.registration.attempts_total` 按 `reason` 与 `status` 计数；`registry.instance.registered` gauge 在发布中为 1、否则为 0——自愈失败会落在这里，而不只是出现在日志里。发现半边把每次后台缓存同步上报到 `discovery.sync_total`，并维持 `discovery.cache.age_seconds`（距上次确认新鲜的秒数），watch 死掉时表现为持续爬升，而不是静默返回陈旧地址。 `reason` 取 `initial`（初次发布）与 `self_heal`（后台重注册）。
- 启动探测:每块构建时 `Catalog().Services`(5s 超时),`address` 配错启动即失败。
- 心跳 `UpdateTTL` 失败会升级:连续失败记日志并重注册(upsert)自愈。

## 4. 设计体检表

| 指标 | 数值 |
|------|------|
| 配置 key | 每块 7(连接)+ 实例 5(`spring.registry.*`) |
| 必填 | 每块 1(`address`)+ Run 期 2(`service-name`、`addr`,仅注册需要) |
| quickstart 前置外部依赖 | 1(consul;docker 门控) |
| "注意/坑"条数 | 5 |

设计嫌疑:无日志 tag(与 nacos/etcd 不一致);派生 ID 撞号语义;`Weights.Warning: 1`
硬编码;blocking query 的 `WaitTime`(5m)与失败重试间隔(5s)不可配。
