# starter-registry-consul 使用说明

精简版(client 家族格式)。概览见 [README.md](README.md)。锚定 [example/](example/)(docker-compose +
`check.sh`)。**Consul 语义(健康检查、TTL、catalog)见 [Consul 官方文档](https://developer.hashicorp.com/consul/docs)**
——本文只写绑定面。

**仅注册侧**:本 starter 不提供 Consul discovery 后端(README 已同步修正)。

## 1. 快速开始

前置依赖:一个 Consul agent(example 自带 docker-compose)。

```properties
spring.registry.consul.address=127.0.0.1:8500
spring.registry.service-name=orders
spring.registry.addr=10.0.0.1:8080
```

blank-import + `gs.Run()`。注册器用 TTL 心跳保活;`PreStop` 反注册。

## 2. 全量配置参考

### `spring.registry.consul.*`(连接,8 key)

| Key | 类型 | 默认值 | 说明 |
|-----|------|--------|------|
| `address` | string | — | 必填;存在即激活模块 |
| `scheme` | string | http | |
| `datacenter` | string | ""(用 agent 的) | |
| `token` | string | "" | |
| `namespace` | string | ""(Enterprise) | |
| `ttl` | duration | 15s | 心跳间隔 |
| `deregister-critical-after` | duration | 1m | 0 = 关闭;agent 卡死时可见性很快消失 |

### `spring.registry.*`(实例,5 key)

| Key | 类型 | 默认值 | 说明 |
|-----|------|--------|------|
| `service-name` / `addr` | string | "" | 启动期必填 |
| `id` | string | 派生 `ServiceName-Addr` | ⚠ 同 addr 不同服务会撞 ID |
| `weight` | int | 0 → 存为 1 | 只有 `UpdateWeight` 能存 0(摘流) |
| `metadata` | map | 空 | |

## 3. bean 与运维

- `registryServer`(`gs.Server`);`UpdateWeight(ctx, w)` 以 `Weights.Passing = w` upsert
  (0 = 摘流透传;未注册时报错)。
- 没有专属日志 tag——全部走 `log.TagAppDef`(与 registry-nacos/etcd 不一致)。
- 无快速失败探测:`address` 配错只在首次 Register 时暴露(nacos/etcd 启动期探测)。
- 心跳 `UpdateTTL` 错误被**丢弃**——agent 故障时实例静默变 critical 并被自动反注册。

## 4. 设计体检表

| 指标 | 数值 |
|------|------|
| 配置 key | 13(8+5) |
| 必填 | 3(`address`、`service-name`、`addr`) |
| quickstart 前置外部依赖 | 1(consul;docker 门控) |
| "注意/坑"条数 | 5 |

设计嫌疑:TTL 心跳失败静默(错误丢弃、无日志);无启动探测(starter.go 注释自己承认);
无日志 tag(与 nacos/etcd 不一致);派生 ID 撞号语义;`Weights.Warning: 1` 硬编码;
无 discovery 半边(与 nacos/etcd starter 不对称)。
