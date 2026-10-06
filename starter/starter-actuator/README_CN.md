# starter-actuator

[English](README.md) | [中文](README_CN.md)

> 项目已正式发布，欢迎使用！

给服务加一个运维端口：Kubernetes 存活/就绪/启动探针、版本信息，配上 starter-otel
还能把 Prometheus `/metrics` 挂上来——全在一个端口上，和业务端口分开。引包、配一行，
就能用。

有个地方和别的 server 不一样，是故意的：这个 server 在应用**还没启动完**的时候就开始
应答。K8s 需要这样——就绪探针要能看到「没就绪 → 就绪」的翻转，存活探要在漫长启动期间
一直有人应声，Pod 才不会被白白重启。

## 安装

```bash
go get go-spring.org/starter-actuator
```

## 快速开始

### 1. 引入包

参考 [main.go](example/main.go)。

```go
import _ "go-spring.org/starter-actuator"
```

### 2. 打开开关

在[配置文件](example/conf/app.properties)里加一行：

```properties
spring.actuator.addr=:9370
# 可选：给整个端口加 token。都不配鉴权又绑了非 loopback 地址，启动会打 WARN。
# spring.actuator.token=s3cret
```

不配 `addr` 就没有 actuator——这个 key 就是开关。

### 3. 探一下

```bash
curl http://127.0.0.1:9370/healthz     # 进程还活着吗？
curl http://127.0.0.1:9370/readyz      # 能接流量吗？
curl http://127.0.0.1:9370/startupz    # 启动完了吗？
curl http://127.0.0.1:9370/info        # 这是哪个版本？
```

`/health`、`/readiness`、`/startup` 也能用——同样的端点，旧名字。

直接写进 Pod 定义：

```yaml
startupProbe:
  httpGet: { path: /startupz, port: 9370 }
  failureThreshold: 30        # 预算 = failureThreshold × periodSeconds；
  periodSeconds: 2            # 必须覆盖最坏启动时长（见下）
livenessProbe:
  httpGet: { path: /healthz, port: 9370 }
readinessProbe:
  httpGet: { path: /readyz, port: 9370 }
```

## 工作模型

actuator 和所有 `gs.Server` 一样，应用就绪后才开始服务——等的是同一道就绪屏障。
三个探针的分工只有一种：

- **startupProbe → `/startupz`**：启动期唯一在工作的探针。应用就绪前它探的是
  一个还没人应答的端口，每次 connection refused 都在烧预算
  （`failureThreshold × periodSeconds`）——这是设计内机制，所以预算**必须**
  覆盖最坏启动时长：部署的必配项，不是可选项。首次成功之前，liveness 和
  readiness 根本不被轮询。
- **livenessProbe → `/healthz`**：只判"进程活没活"。超阈值失败会重启容器；
  绝不能因为启动慢或依赖挂而触发。优雅排空期间和其他探针一样回 `503`
  OUT_OF_SERVICE——实例要下线了。
- **readinessProbe → `/readyz`**：只判"该不该接流量"。失败只是把 pod 摘出
  Service 端点，永远不杀进程。

唯一合理的工作模型就是上面这套：微服务必须配 startupProbe，预算覆盖最坏启动时长；
liveness 只判死、readiness 只判流量，启动期探 readiness 无消费者。所以**没有**任何
"提前应答 / 早期监听"逻辑：actuator 的 `Run` 绑定监听后 `<-sig.TriggerAndWait()` 再
`Serve(ln)`，与 `SimpleHttpServer` 完全同构（旁路 goroutine 与 `ready` 标志已删——
`Serve` 即就绪，且没有消费者要更早的信号）（2026-10-03）。排空期三个探针统一 `503`
OUT_OF_SERVICE（liveness 也判 draining）；非排空期 liveness/startup 无门槛，readiness
才看 indicator。


## 端点

| 端点 | 它回答什么 |
| --- | --- |
| `/healthz`（别名 `/health`） | 进程在就返回 `200`；优雅排空期间 `503` OUT_OF_SERVICE。它**故意不**检查依赖——数据库挂了该摘流量，不该重启 Pod。 |
| `/readyz`（别名 `/readiness`） | 关键依赖都通过则 `200`；关键依赖挂了或优雅排空期间 `503`。只是非关键依赖挂了：`200`、状态 `DEGRADED`——还在服务，问题写在响应体里。 |
| `/startupz`（别名 `/startup`） | startup 组 indicator 的聚合。应用就绪前端口无人应答（connection refused 烧 startupProbe 预算——见上面的工作模型）；开始服务后由它报告启动健康度。 |
| `/info` | 编译进二进制的版本信息（模块路径/版本、Go 版本、从代码库构建时的 git 版本/时间）。 |
| `/metrics` | 不是本 starter 提供的。引入 `starter-otel` 并配 Prometheus exporter 后，它的抓取 handler 会挂到这里——同一个端口，监控只看一处。 |

健康指示器就是一个 bean：把任何东西（redis 客户端、db 连接池）导出为
`health.Indicator`，它就自动进 `/readyz` 的聚合——零接线，组件也不用 import 本
starter。写法见 [example](example/main.go)。

### 指标挂载

想让 `/metrics` 上这个端口，配 `starter-otel`：

```properties
spring.observability.metrics.exporter=prometheus
spring.observability.metrics.port=0        # 不另起指标服务器，只走 actuator
```

### Prometheus 发现

Pod 注解：

```yaml
metadata:
  annotations:
    prometheus.io/scrape: "true"
    prometheus.io/port: "9370"
    prometheus.io/path: "/metrics"
```

或者在 Service 上暴露端口，用 `ServiceMonitor` 选中：

```yaml
apiVersion: monitoring.coreos.com/v1
kind: ServiceMonitor
metadata:
  name: my-app
spec:
  selector:
    matchLabels:
      app: my-app
  endpoints:
    - port: management   # 映射到 9370 的 Service 端口
      path: /metrics
      interval: 15s
```

## 配置

| 属性 | 默认值 | 干什么的 |
| --- | --- | --- |
| `spring.actuator.addr` | — | 监听地址。必填——配上它 starter 才生效。`:9370` 绑所有网卡，集群内探针够得着；和业务端口（`:9090`）、pprof（`127.0.0.1:9981`）分开。 |
| `spring.actuator.check-timeout` | `3s` | 一次探针扫描（全部 indicator 并发）的总预算，共享。应低于 kubelet 探针的 `timeoutSeconds`。 |
| `spring.actuator.read-header-timeout` | `5s` | 读请求头的期限（slowloris 防护）。`0` 关闭。 |
| `spring.actuator.read-timeout` / `write-timeout` / `idle-timeout` | `0` | 对应 `http.Server` 同名字段；`0` = 不限时。管理端口都是短的探针/指标请求，默认全关——端口能被不可信网络触达时再设 `write-timeout`。 |
| `spring.actuator.token` | `""` | 整个端口要求 `Authorization: Bearer <token>`。优先于 Basic。 |
| `spring.actuator.username` / `spring.actuator.password` | `""` | HTTP Basic 凭据（两个都配才生效）。什么鉴权都不配又绑非 loopback 地址 → 启动打 WARN。 |

## 许可证

本项目基于 Apache License 2.0 许可证。
