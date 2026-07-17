# 可观测方案组合(Go-Spring 风格)

[English](README.md) | [中文](README_CN.md)

**一个应用,多种可观测后端。** 本示例用**同一个、零改动**的 dubbo-go **Triple**
服务(与 [`../dubbo-go/triple`](../dubbo-go/triple) 是同一个 `GreetService`),
接到多套可互换的 **MTL**(Metrics / Traces / Logs)后端上,让你并排对比主流的
可观测组合方案。

应用在所有 stack 里**逐字节相同**,三支柱输出固定:

- **Metrics** —— provider 在 `:9090` 暴露 Prometheus 抓取端点(consumer 在
  `:9091`),`dubbo_*` 指标。
- **Traces** —— OTel span 通过 **OTLP/gRPC** 导出到 `127.0.0.1:4317`。
- **Logs** —— 结构化 JSON 行写入 `logs/*.log`(有 span 时带 `trace_id` /
  `span_id` 字段)。

stack 之间**唯一变化**的,是**谁监听 `:4317`、谁抓 `:9090`、谁收 `logs/`**——
也就是后端管道,而不是代码。这正是核心思想:**埋点不动,后端可插拔。**

## M/T/L 可分割吗?

技术上可分——你完全可以只上报 metrics。但它们的**价值在关联**:从一条慢
trace 跳到它的日志(日志行里的 `trace_id`),或从延迟直方图跳到样本 trace
(exemplar)。所以这里每个 stack 都带齐三支柱,而 stack 3 专门演示它们如何被
"串起来"。

## 四种 stack

| # | Stack | 管道 | Metrics | Traces | Logs | 亮点 |
|---|-------|------|---------|--------|------|------|
| **1** | [`1-classic`](stacks/1-classic) | 应用 **直连** 后端(无 collector) | Prometheus(抓取) | Jaeger(OTLP) | Loki(Promtail) | 最常见的起点;每个信号各自独立后端 |
| **2** | [`2-collector`](stacks/2-collector) | 应用 → **OTel Collector** → 扇出 | Prometheus | Jaeger | Loki | 厂商中立的统一管道;埋点与后端解耦 |
| **3** | [`3-lgtm`](stacks/3-lgtm) | 应用 → OTel Collector → **LGTM** | Prometheus(exemplar) | **Tempo** | Loki | **关联**:Grafana 里 trace↔日志↔指标 exemplar 互跳 |
| **5** | [`5-elastic`](stacks/5-elastic) | 应用 → OTel Collector → **Elasticsearch** | Elasticsearch | Elasticsearch | Elasticsearch | 三信号同存一库,Kibana 查看(OTel 原生,无 Beats) |

> 编号沿用设计本项目时的调研(stack 4 = SigNoz/Uptrace 式单一 ClickHouse 存储,
> stack 6 = VictoriaMetrics,都有意略去)。stack 5(Elastic)为可选——镜像最重、
> 预热最久。

### 其他组合,以及为什么没做

上面四种沿三个轴覆盖了整个设计空间——**埋点方式**(框架原生 / 手工 OTel SDK /
eBPF 自动)、**管道**(直连 / collector)、**存储**(每信号一套 / 全合一)。
考虑过但略过的:

- **ClickHouse 全合一**(SigNoz / Uptrace)—— 单库单 UI,和 Elastic 概念重叠。
- **VictoriaMetrics + VictoriaLogs** —— Prometheus/Loki 的轻量替代。
- **eBPF 无侵入埋点**(Grafana Beyla)—— 零代码,属于*埋点*轴而非后端轴。
- **SaaS 单面板**(Datadog / New Relic / Honeycomb / Grafana Cloud)——
  底层通常也是 OTLP,需要账号。

## 目录结构

```
contrib/observability/
├── proto/                     # 共享 Triple IDL + 生成的桩(*.go 请勿手改)
├── provider/                  # GreetService provider(与 dubbo-go/triple 相同)
│   └── conf/app.properties    # metrics :9090, OTLP → :4317, JSON 日志 → ../logs
├── consumer/                  # 经 etcd 发现、调用、断言、退出
│   └── conf/app.properties    # metrics :9091, JSON 日志 → ../logs
├── stacks/
│   ├── 1-classic/             # docker-compose + prometheus/promtail/grafana 配置
│   ├── 2-collector/           # + otel-collector-config.yml
│   ├── 3-lgtm/                # + otel-collector-config.yml、tempo.yml、关联数据源
│   └── 5-elastic/             # + otel-collector-config.yml(elasticsearch exporter)
└── scripts/
    ├── gen-code.sh            # 从 IDL 重新生成 proto/*.go
    └── smoke-test.sh <stack>  # 拉起某个 stack + 应用,跑 consumer,再拆掉
```

每个 `stacks/<name>/` 是自包含的 docker-compose 工程(各自带 etcd)。同一时刻
只跑一个 stack,所以它们复用同一批宿主端口。

## 运行

选一个 stack 跑它的一次性冒烟测试——它会拉起后端与注册中心,在宿主上构建并
运行 provider,跑 consumer(1 次基准 + 20 次批量调用),断言,然后全部拆除:

```bash
bash scripts/smoke-test.sh 1-classic     # 或 2-collector | 3-lgtm | 5-elastic
```

也可以手动拉起某个 stack 并保持运行,自己去逛各 UI:

```bash
cd stacks/3-lgtm
docker compose up -d               # 或 docker-compose up -d
cd ../..
mkdir -p logs
go run ./provider &                # 终端 A:长驻,注册进 etcd
go run ./consumer                  # 终端 B:发起调用
```

consumer 预期输出:

```
Response from discovered provider: Hello, Dubbo-Go!
Sent 21 greetings (1 canonical + 20 batch)
```

> 若你的 shell 导出了 HTTP 代理,请先把 `127.0.0.1,localhost` 加进
> `no_proxy`/`NO_PROXY`,否则本地 `curl`/UI 请求会被代理带走。

## 各 stack 去哪儿看

所有 UI 都在 `127.0.0.1`。

### 1-classic / 2-collector
| 信号 | UI | 试试 |
|------|----|------|
| Metrics | Prometheus http://127.0.0.1:9099 | 查询 `dubbo_provider_requests_total` |
| Traces | Jaeger http://127.0.0.1:16686 | 服务 `obs-demo`,*Find Traces* |
| Logs | Grafana http://127.0.0.1:3000 | Explore → Loki → `{job="obs-demo"}`(stack 1)/ `{service_name="obs-demo"}`(stack 2) |

### 3-lgtm(关联)
打开 Grafana http://127.0.0.1:3000 → **Explore**:
- **Tempo**:找一条 trace → 每个 span 可**跳到 Loki**(按 `trace_id` 过滤)
  和**跳到 Prometheus**(相关指标)。
- **Prometheus**:在面板上开启 *Exemplars* → 点 exemplar 圆点 → **跳到 Tempo
  trace**。
- **Loki**:`{service_name="obs-demo"} | json` → `trace_id` 字段是可点击的
  **derived field**,直达 Tempo。

### 5-elastic
打开 **Kibana** http://127.0.0.1:5601 → *Discover* / *Observability*。三支柱
经 collector 的 `elasticsearch` exporter 全部落入 Elasticsearch
(`logs-obs-demo`、`traces-obs-demo`、`metrics-obs-demo`)。

## 应用为何零改动

provider/consumer 用的是 **starter-dubbo 内置的可观测能力**,完全由
`conf/app.properties` 驱动——`main.go` 里没有任何可观测代码:

```properties
# metrics —— 独立于 HTTP server 的 Prometheus 抓取端点
spring.dubbo.metrics.enable=true
spring.dubbo.metrics.port=9090

# tracing —— OTel span 经 OTLP/gRPC 发往当前 stack 里占据 :4317 的那个组件
spring.dubbo.tracing.enable=true
spring.dubbo.tracing.exporter=otlp-grpc
spring.dubbo.tracing.endpoint=127.0.0.1:4317

# logging —— 结构化 JSON 写入 ../logs,由 Promtail(stack 1)或 collector 的
# filelog receiver(stack 2/3/5)采集
logging.logger.root.type=FileLogger
logging.logger.root.layout.type=JSONLayout
logging.logger.root.dir=../logs
```

因为 tracing 端点恒为 `127.0.0.1:4317`,切 stack 纯粹取决于哪个容器在那儿监听
——应用毫不知情。这是一个可运行示例,**不是**可复用的 starter 模块。

## 重新生成 proto

与 Triple 示例同一套 IDL 和工具链:

```bash
bash scripts/gen-code.sh
```

> 在 `runtime.Version()` 带实验后缀(如 `go1.26.1-X:jsonv2`)的 go1.26 工具链上,
> `protoc-gen-go-triple` v3.0.3 会在解析版本号时崩溃——请从源码重编,把版本
> 字符串截断到纯数字部分。
