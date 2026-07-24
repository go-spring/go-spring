# starter-dubbo 配置模型与动态配置下发审计

> 审计日期: 2026-07-24 | 代码基线: `lvan100_dev` | dubbo-go: v3.3.1

## 一、配置模型

starter-dubbo 的配置模型（`dubbo_schema.go`）完全对齐 dubbo-go.json schema，全部绑定在 `${spring.dubbo}` 下：

| 节点 | 类型 | 说明 |
|---|---|---|
| `application` | `DubboApplication` | 进程级元数据 (name, org, module, version, owner, env, metadata-type) |
| `registries` | `map[string]DubboRegistry` | 全局注册中心，role 按 ID 引用 |
| `protocols` | `map[string]DubboProtocol` | 全局协议监听器，server 继承 |
| `metadata-report` | `DubboMetadataReport` | 元数据中心配置 |
| `provider` | `DubboProvider` | 提供者端全局默认 + `services` 按 service 覆盖 + `methods` 按方法调优 |
| `consumer` | `DubboConsumer` | 消费者端全局默认 + `references` 按引用覆盖 + `methods` 按方法调优 |
| `metrics` | `DubboMetric` | Prometheus metrics |
| `tracing` | `DubboTracing` | OTel tracing |
| `shutdown` | `DubboShutdown` | 优雅停机 |

**结论：配置模型准确。** `Instance` 持有完整 `DubboConfig`（单次绑定 `${spring.dubbo}`），`NewClient` / `NewSimpleDubboServer` 从 `Instance` 取配置。

---

## 二、动态配置下发链路

```
properties 变更 (Nacos/file/env)
  → go-spring RefreshProperties()
    → gs.Dync[T] 原子交换新值
      → dyncPoller (5s 轮询) 检测变化
        → consumerToOverrideRules() 转换为 override rules
          → mapconfig.RefreshOverrideRules() 写入内存 + 通知 dubbo-go listener
            → consumerConfigurationListener / referenceConfigurationListener
              → 更新 configurators → 线上 invoker URL 更新 → 下次调用生效
```

**状态：链路贯通。** mapconfig 已激活，consumer-level defaults 发布为 `appName.configurators`，各 reference 独立发布为 `<interface>.configurators`，retries=0 可正确下发。

---

## 三、相关文档

- `starter-dubbo/README.md` — 使用文档
- `starter-dubbo/dubbo-go.json` — dubbo-go 配置 schema
- `starter-dubbo/DESIGN.md` — 本文档
- dubbo-go v3.3.1 `registry/directory/directory.go` — config-center listener
- go-spring `spring/gs/internal/gs_dync/dync.go` — Dync 实现
