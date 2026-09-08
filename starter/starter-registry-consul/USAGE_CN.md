# starter-registry-consul 使用说明

精简版(client 家族格式)。概览见 [README.md](README.md)。锚定 [example/](example/)(docker-compose +
`check.sh`)。**Consul 语义(健康检查、TTL、catalog)见 [Consul 官方文档](https://developer.hashicorp.com/consul/docs)**
——本文只写绑定面。

**两半齐备**:同一份 `${spring.registry.consul}` 中心配置派生注册端(registrar)与
消费端(discovery 后端,`discovery-name` 默认 `consul`),共享一个客户端连接;也可用
`${spring.discovery.consul.<name>}` 独立块接其他 agent。

## 1. 快速开始

前置依赖:一个 Consul agent(example 自带 docker-compose)。

```properties
spring.registry.consul.address=127.0.0.1:8500
spring.registry.service-name=orders
spring.registry.addr=10.0.0.1:8080
```

blank-import + `gs.Run()`。注册器用 TTL 心跳保活;`PreStop` 反注册。

## 2. 全量配置参考

### `spring.registry.consul.*`(连接,9 key)

| Key | 类型 | 默认值 | 说明 |
|-----|------|--------|------|
| `address` | string | — | 必填;存在即激活模块 |
| `scheme` | string | http | |
| `datacenter` | string | ""(用 agent 的) | |
| `token` | string | "" | |
| `namespace` | string | ""(Enterprise) | |
| `ttl` | duration | 15s | 心跳间隔 |
| `deregister-critical-after` | duration | 1m | 0 = 关闭;agent 卡死时可见性很快消失 |
| `discovery-name` | string | consul | 为同一 agent 派生该标签的 discovery 后端 bean;空 = 关闭 |

### `spring.registry.*`(实例,5 key)

| Key | 类型 | 默认值 | 说明 |
|-----|------|--------|------|
| `service-name` / `addr` | string | "" | 启动期必填 |
| `id` | string | 派生 `ServiceName-Addr` | ⚠ 同 addr 不同服务会撞 ID |
| `weight` | int | 0 → 存为 1 | 只有 `UpdateWeight` 能存 0(摘流) |
| `metadata` | map | 空 | |

### `spring.discovery.consul.<name>.*`(发现块,6 key)

每个块一个命名 discovery 后端 bean(bean 名即 client 引用的标签)。`address` 留空 =
继承 `${spring.registry.consul}` 中心连接(共享客户端,无独立销毁);非空 = 自建客户端
并做启动探测。

| Key | 类型 | 默认值 | 说明 |
|-----|------|--------|------|
| `address` | string | "" | 留空继承中心连接 |
| `scheme` | string | http | |
| `datacenter` | string | ""(用 agent 的) | |
| `token` | string | "" | |
| `namespace` | string | ""(Enterprise) | |
| `tag` | string | "" | Consul 服务标签,收窄每次查询;调用侧 `discovery.WithTag` 可逐次覆盖 |

解析语义:只查 **passing** 实例(不健康不进快照);每个已解析服务一条后台 blocking
query(index 长轮询)保鲜,后续 Resolve 读缓存;`Meta["scheme"]` → `Endpoint.Scheme`,
`Weights.Passing` → 权重,无服务地址时回退节点地址。

## 3. bean 与运维

- `registryServer`(`gs.Server`)与 `discovery-name`(默认 `consul`)后端均派生自中心
  bean `consulCenter`(共享 `*api.Client`;Consul 客户端无需关闭,故无 Destroy)。
- `UpdateWeight(ctx, w)` 以 `Weights.Passing = w` upsert(0 = 摘流透传;未注册时报错)。
- 没有专属日志 tag——全部走 `log.TagAppDef`(与 registry-nacos/etcd 不一致)。
- 启动探测:center 构建时 `Catalog().Services`(5s 超时),`address` 配错启动即失败。
- 心跳 `UpdateTTL` 失败会升级:连续失败记日志并重注册(upsert)自愈。

## 4. 设计体检表

| 指标 | 数值 |
|------|------|
| 配置 key | 20(9 连接 + 5 实例 + 6 发现块) |
| 必填 | 3(`address`、`service-name`、`addr`) |
| quickstart 前置外部依赖 | 1(consul;docker 门控) |
| "注意/坑"条数 | 5 |

设计嫌疑:无日志 tag(与 nacos/etcd 不一致);派生 ID 撞号语义;`Weights.Warning: 1`
硬编码;blocking query 的 `WaitTime`(5m)与失败重试间隔(5s)不可配。
