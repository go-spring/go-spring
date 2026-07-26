# starter-otel

[English](README.md) | [中文](README_CN.md)

`starter-otel` 是 Go-Spring 的统一可观测核心。它在框架层定义唯一的可观测配置
（`${spring.observability}`），构建共享的 OpenTelemetry `TracerProvider` /
`MeterProvider`，并在启动时将其安装为 OTel 进程全局。

各个已埋点的组件（gorm，后续更多）通过自身的 OTel 插件读取这些全局，因此你只需
**在这里配置一次可观测**，无需逐个适配组件。引入本 starter，整条链路即打通；不引入
时组件回落到 OTel 的 no-op 全局——零开销、无报错。

## 工作原理

```
组件（如 gorm 插件）  ──读取──▶  OTel 全局 (otel.GetTracerProvider/GetMeterProvider)
                                        ▲
starter-otel  ──启动时写入─────────────┘  otel.SetTracerProvider / SetMeterProvider
```

- **组件依赖 OTel API，而非本 starter。** 两者通过 OTel 进程全局解耦。
- **启用是全局隐式的。** 引入 `starter-otel` 即安装真实 provider；不引入则保留
  no-op 全局。
- **时机有保证。** provider 在模块 setup 阶段被提前（eager）构建，该阶段早于任何
  组件 bean 的实例化，因此组件安装插件时读到的一定是已就绪的真实 provider。

## 安装

```bash
go get go-spring.org/starter-otel
```

## 快速开始

### 1. 引入 `starter-otel` 包

```go
import _ "go-spring.org/starter-otel"
```

与一个已埋点的组件一起引入，这就是全部接线：

```go
import (
    _ "go-spring.org/starter-otel"
    _ "go-spring.org/starter-gorm-mysql"
)
```

### 2. 配置可观测

在项目的配置文件中添加可观测配置，比如通过 OTLP/gRPC 把 trace 和 metrics 同时导出
到 OTel Collector：

```properties
spring.observability.enable=true
spring.observability.service-name=my-service

spring.observability.trace.exporter=otlp-grpc
spring.observability.trace.endpoint=127.0.0.1:4317
spring.observability.trace.insecure=true

spring.observability.metrics.exporter=otlp-grpc
spring.observability.metrics.endpoint=127.0.0.1:4317
spring.observability.metrics.insecure=true
```

到此为止。此后你引入的任何已埋点组件都会通过这些 provider 产出 span 和 metrics。

## 内置 Exporter

以下 exporter 均已编译进 `starter-otel`,每种信号通过 `exporter` 配置项选择其一
即可,无需额外依赖或代码。

Trace(`spring.observability.trace.exporter`):

| 取值 | 后端 | 说明 |
| --- | --- | --- |
| `otlp-grpc` | OTLP over gRPC | 默认。发往 `endpoint`(`:4317`)上的 Collector / 原生支持 OTLP 的后端。 |
| `otlp-http` | OTLP over HTTP | 同上,走 HTTP(`:4318`)。 |
| `stdout` | 标准输出 | 以 JSON 打印 span,便于本地调试。 |
| `none` | (禁用) | 不构建 `TracerProvider`。 |

Metrics(`spring.observability.metrics.exporter`):

| 取值 | 后端 | 说明 |
| --- | --- | --- |
| `otlp-grpc` | OTLP over gRPC | 默认。每隔 `interval` 推送到 `endpoint`(`:4317`)上的 Collector。 |
| `otlp-http` | OTLP over HTTP | 同上,走 HTTP(`:4318`)。 |
| `prometheus` | Prometheus(pull) | 在 `port` 上暴露独立的 `/metrics` 端点供抓取；当引入 `starter-actuator` 时，同一抓取 handler 还会挂载到管理端口（见[通过 Actuator 暴露指标](#通过-actuator-暴露指标)）。 |
| `stdout` | 标准输出 | 每隔 `interval` 以 JSON 打印 metrics。 |
| `none` | (禁用) | 不构建 `MeterProvider`。 |

若要对接上表之外的后端,保持 `otlp-grpc`/`otlp-http`,交由 OpenTelemetry
Collector 转换/路由——参见[对接多种后端](#对接多种后端)。

## 配置项参考

所有配置均位于 `${spring.observability}` 下。

| 配置项 | 默认值 | 说明 |
| --- | --- | --- |
| `enable` | `true` | 总开关；为 `false` 时本 starter 不安装任何东西。 |
| `service-name` | `${spring.application.name:=go-spring-app}` | `service.name` 资源属性。 |

Trace，位于 `${spring.observability.trace}`：

| 配置项 | 默认值 | 说明 |
| --- | --- | --- |
| `enable` | `true` | 启用共享的 `TracerProvider`。 |
| `exporter` | `otlp-grpc` | `otlp-grpc` \| `otlp-http` \| `stdout` \| `none`。 |
| `endpoint` | （空） | Collector 地址；otlp exporter 必填。 |
| `insecure` | `true` | 对 otlp exporter 关闭 TLS。 |
| `sampler-ratio` | `1.0` | ParentBased 比例采样（`>=1` 全采，`<=0` 不采）。 |
| `propagator` | `w3c` | `w3c`（TraceContext + Baggage）\| `none`。 |

Metrics，位于 `${spring.observability.metrics}`：

| 配置项 | 默认值 | 说明 |
| --- | --- | --- |
| `enable` | `true` | 启用共享的 `MeterProvider`。 |
| `exporter` | `otlp-grpc` | `otlp-grpc` \| `otlp-http` \| `prometheus` \| `stdout` \| `none`。 |
| `endpoint` | （空） | Collector 地址；otlp exporter 必填。 |
| `insecure` | `true` | 对 otlp exporter 关闭 TLS。 |
| `port` | `9090` | 独立 `/metrics` server 的端口（prometheus exporter）。设为 `0` 则不启动独立 server，仅通过 actuator 管理端口暴露 `/metrics`（见[通过 Actuator 暴露指标](#通过-actuator-暴露指标)）。 |
| `path` | `/metrics` | prometheus 抓取端点的路径（独立 server 与 actuator 挂载共用）。 |
| `interval` | `10s` | otlp/stdout reader 的推送间隔。 |

## 通过 Actuator 暴露指标

同时引入 `starter-actuator` 时，Prometheus exporter 的抓取 handler 会挂载到
actuator 管理端口（`:9370`）上——可与自身的独立 server 并存，也可完全取而代之。
运维方由此**只抓一个端口**即可同时拿到探针与指标。

这通过零依赖的 `go-spring.org/spring/endpoint` 缝隙实现：`starter-otel` 把抓取
handler 导出为 `endpoint.Endpoint` bean，actuator 将所有此类 bean 挂载到管理
server。两个 starter 互不 import。

```go
import (
    _ "go-spring.org/starter-otel"
    _ "go-spring.org/starter-actuator"
)
```

```properties
# 仅通过 actuator 暴露 /metrics（不再另起独立指标 server）：
spring.observability.metrics.exporter=prometheus
spring.observability.metrics.port=0
spring.observability.metrics.path=/metrics
```

```bash
curl http://127.0.0.1:9370/metrics
```

`port` 保持非零则独立 server 与 actuator 挂载同时存在。Pod 注解与 `ServiceMonitor`
抓取示例见 actuator README 的*指标与 Kubernetes 抓取*一节。

## Trace ↔ Log 关联

启用 tracing 时，`starter-otel` 会安装一个 `log.FieldsFromContext` 钩子，把当前
span 的 `trace_id` 与 `span_id` 从 context 提取到每一条以该 context 写出的日志
记录上。一个请求的日志由此自动与其 trace 关联，无需任何逐调用点的代码：

```go
log.Infof(ctx, tag, "handling request")
// → ...||trace_id=29b84b8a2f4eaa447120bf01282c3930||span_id=bc6627138bf150f7||msg=handling request
```

当 context 不携带有效 span 时该钩子为 no-op，因此未埋点的代码路径零开销。

## 对接多种后端

你的应用永远只导出到**一个** OTLP 出口——通常是 OpenTelemetry Collector。向多个
后端扇出是 Collector 的职责，而非应用的职责：

```
app (starter-otel) ──OTLP──▶ Collector ──┬─▶ Jaeger / Tempo   (trace)
                                          ├─▶ Prometheus       (metrics)
                                          └─▶ Loki / ES        (log)
```

新增或替换后端只需改 Collector 配置，应用代码与配置文件一个字都不用动。若某后端原生
支持 OTLP（如 Jaeger 的 `:4317`），可将 `endpoint` 直接指向它；Prometheus 走 pull
模式，用 `exporter=prometheus` 在 `port` 上暴露抓取端点即可。

## 示例

参见 [example](example) 的可运行、自校验冒烟测试：引入 `starter-otel` +
`starter-actuator` 并配置一次 `${spring.observability}`，即可（1）通过 `trace_id`
将一个请求的日志关联到其 trace，（2）在 actuator 管理端口上暴露 otel 的 Prometheus
`/metrics`——无需任何外部服务（tracing 用 stdout exporter，metrics 进程内抓取）。

参见 [contrib/observability-gorm](../../contrib/observability-gorm) 的端到端示例，
它额外引入一个已埋点组件（`starter-gorm-mysql`）：无需任何逐组件埋点代码，即可在
Collector 侧拿到 GORM 查询 span 和连接池指标。

## 优雅关闭

provider 以带 destroy 钩子的 bean 形式注册，因此关闭时会 flush 缓冲的 span 和
metrics 并干净地关闭 exporter。
