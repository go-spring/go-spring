# starter-dubbo 配置模型与动态配置下发审计

> 首审: 2026-07-24 | 复审: 2026-08-28 | 代码基线: `lvan100_dev` | dubbo-go: v3.3.1
> 详细行为以 `USAGE.md` / `USAGE_CN.md` 与源码为准，本文只记架构结论。

## 一、配置模型

配置模型定义在 `config.go`（`DubboConfig` 及各节点结构体，无独立 schema 文件），全部
绑定在 `${spring.dubbo}` 下：

| 节点 | 类型 | 说明 |
|---|---|---|
| `application` | `DubboApplication` | 进程级元数据 (name, org, module, version, owner, env, metadata-type) |
| `registries` | `map[string]DubboRegistry` | 全局注册中心，role 按 ID 引用 |
| `protocols` | `map[string]DubboProtocol` | 全局协议监听器，server 继承 |
| `provider` | `DubboProvider` | 提供者端全局默认 + `services` 按 service 覆盖 + `methods` 按方法调优 |
| `consumer` | `DubboConsumer` | 消费者端全局默认 + `references` 按引用覆盖 + `methods` 按方法调优 |
| `metrics` | `DubboMetric` | Prometheus metrics |
| `tracing` | `DubboTracing` | OTel tracing |
| `shutdown` | `DubboShutdown` | 优雅停机 |

**结论：配置模型准确。** `Instance` 持有完整 `DubboConfig`（单次绑定 `${spring.dubbo}`），
`NewClient` / `NewSimpleDubboServer` 从 `Instance` 取配置。

2026-08-28 清理（USAGE_FINDINGS #35）：

- 删除死配置：`metadata-report.*`、`provider.proxy`、`consumer.proxy`、
  `consumer.max-wait-time-for-service-discovery`、`services.<n>.max_message_size`、
  `metrics.mode/namespace`、旧版 jaeger tracing 字段（name/serviceName/address/use-agent）。
- `retries` 语义统一：所有层级 `-1`（默认）= 未设置、`0` = 不重试、`>0` = 重试次数；
  低于 `-1` 在 `NewInstance` 校验报错（`validateRetries`）。
- `check` 默认值统一为两级 `true`（fail-fast）。dubbo-go v3 无 reference 级"关闭检查"
  选项，`references.<n>.check=false` 叠加 `consumer.check=true` 时启动 WARN。
- 分隔符混用（连字符 vs 点号）定位为兼容保留的规范化口径：动态层级点号对齐 dubbo
  URL 参数名，静态层级连字符为 starter 自身风格；仅精确匹配。

## 二、动态配置下发链路

```
properties 变更 (Nacos/file/env)
  → go-spring RefreshProperties()
    → gs.Dync[T] 原子交换新值
      → dyncPoller.OnChanged 变更回调（非轮询）触发 poll()
        → consumerToOverrideRules() 转换为 override rules
          → mapconfig.RefreshOverrideRules() 写入内存 + 通知 dubbo-go listener
            → consumerConfigurationListener / referenceConfigurationListener
              → 更新 configurators → 线上 invoker URL 更新 → 下次调用生效
```

**状态：链路贯通。** mapconfig 已激活，consumer-level defaults 发布为
`<appName>.configurators`，各 reference 独立发布为
`<interface>:<version>:<group>.configurators`（冒号分隔 key，见 dync.go
`colonSeparatedKey`），`retries=0` 可正确下发（统一语义 = 不重试）。

## 三、相关文档

- `starter-dubbo/USAGE.md` / `USAGE_CN.md` — 行为文档（权威）
- `starter-dubbo/schema.json` — 配置 schema（由配置模型生成口径维护）
- dubbo-go v3.3.1 `registry/directory/directory.go` — config-center listener
- go-spring `spring/gs/internal/gs_dync/dync.go` — Dync 实现
