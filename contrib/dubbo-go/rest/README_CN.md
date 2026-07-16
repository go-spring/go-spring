# dubbo-go — REST(Go-Spring 风格)

[English](README.md) | [中文](README_CN.md)

一个使用 **REST 协议** —— HTTP/1.1 传输 + go-restful 逐方法(动词、路径、参数来源)路由
—— 的 [Dubbo-go](https://dubbo.apache.org/zh-cn/overview/mannual/golang-sdk/)
`GreetService` 示例,并通过可复用的 **starter-dubbo** 模块以 Go-Spring 的方式装配:
由它提供 `gs.Server` 适配器,`gs.Run()` 驱动生命周期,provider 只是一个
`ServiceRegister` bean,协议与注册中心都取自 `conf/app.properties`,而不是
写死在 `main()` 里。

与 [`../triple`](../triple) 的 Triple 版本不同:REST 没有 protobuf IDL,也没有
代码生成器;与 [`../dubbo`](../dubbo)/[`../jsonrpc`](../jsonrpc) 的兄弟示例也不同:
**REST 不能仅靠方法反射**驱动 —— dubbo-go 需要一份 `RestServiceConfig` 映射把每个
Go 方法钉到具体的 `(HTTP 动词、URL 路径、参数来源)` 三元组上,才能在 Serve 前完成
路由注册。provider 端在 `provider/handler.go` 里安装该映射,consumer 端在
`consumer/main.go` 里安装同名映射,两侧必须一致,且都必须在进程注册/拨号前就位。

它接入 **etcd 注册中心**做真实的**服务注册与发现**:provider 启动时把
`com.example.GreetService`(Java 风格的点分接口名)注册进 etcd;consumer 不知道
provider 的 host:port,而是从同一 etcd 解析出可用地址再发起调用。

这是一个**可运行示例**,并非可复用的 starter 模块。

## 拓扑

```
                ┌──────────────┐
    注册        │     etcd     │    发现
  ┌────────────▶│  :2379       │◀────────────┐
  │             └──────────────┘             │
  │ com.example.GreetService                 │ 解析 provider 地址
  │ → rest://<host>:20003                    │
┌─┴──────────┐                        ┌──────┴─────┐
│  provider  │◀──── REST (HTTP/1) ────│  consumer  │
│ gs.Run()   │  GET /greet?name=...   │  一次性调用 │
│ :20003     │──────────────────────▶│  断言后退出 │
└────────────┘   echo name (JSON)     └────────────┘
```

## 目录结构

```
contrib/dubbo-go/rest/
├── proto/greet.go           # 「IDL」:接口名、方法名、HTTP 动词/路径/查询键常量
├── scripts/gen-code.sh      # no-op —— REST 无 IDL codegen
├── provider/handler.go      # GreetProvider(Go 结构体)+ RestServiceConfig + StarterDubbo.ServiceRegister bean(server 由 starter-dubbo 提供)
├── provider/main.go         # gs.Run(),长驻并注册到 etcd
├── consumer/main.go         # 注册 RestServiceConfig,通过 etcd 发现 provider,调用并断言后退出(裸 dubbo-go 客户端,不走 gs.Run)
├── conf/app.properties      # 仅供 provider 使用的配置(server 角色 + 注册中心 + 可观测,metrics :9090)
├── docker/                  # 后端栈的 Prometheus 与 Promtail 配置
├── docker-compose.yml       # 本地 etcd + Prometheus + Jaeger + Loki + Promtail
└── scripts/smoke-test.sh    # 冒烟脚本:起后端+provider,跑 consumer,断言后自动清理
```

## 如何生成

**什么都不用生成**。REST 在 dubbo-go v3 里没有 protobuf/thrift IDL,也没有代码
生成器 —— 服务表面就是手写的 Go 文件(`proto/greet.go`)——固定 Java 风格接口名、
方法名、HTTP 动词/路径/查询键常量——再加一份匹配签名的手写 provider 结构体,以及
provider/consumer 各自手写的 `RestServiceConfig` 映射。执行 `./scripts/gen-code.sh` 只会打印
一行 "nothing to do",只是为了与 Triple 兄弟目录保持一致的入口。

## 与其他协议的对比

| 关注点        | Triple(`../triple`)              | Dubbo/Hessian2(`../dubbo`)      | JSON-RPC(`../jsonrpc`)             | REST(本模块)                                       |
| ------------- | -------------------------------- | ------------------------------- | ----------------------------------- | ---------------------------------------------------- |
| 传输          | HTTP/2                            | 原始 TCP                        | HTTP/1.1                            | HTTP/1.1                                             |
| 负载          | protobuf                          | Hessian2                        | JSON-RPC 2.0 信封                    | 纯 JSON,无信封                                       |
| URL 布局      | 协议固定                          | 协议固定                        | 协议固定(`POST /<interface>`)      | 每个方法自定义(动词 + 路径 + 参数来源)             |
| IDL           | `.proto` + `protoc-gen-go-triple` | 无 —— 手写 Go 结构体            | 无 —— 手写 Go 结构体                 | 无 —— Go 结构体 + 手写 RestServiceConfig 映射         |
| 客户端连接    | 类型化桩                          | 只需接口名                      | 只需接口名                          | 接口名 + 方法映射                                     |
| 跨语言可达    | 任何 gRPC/Triple 客户端           | Java Dubbo(原生)、Hessian2 运行时 | 任何能发 HTTP + JSON 的客户端       | 任何能发 HTTP 的客户端(curl、浏览器、网关等)         |
| 何时选它      | 纯 Go 新业务                       | 与既有 Java Dubbo 服务互通       | 调试 / 裸 HTTP / 兜底                | 对外 REST API、网关友好的端点                        |

## 配置

```properties
# 关闭内置 HTTP server,provider 只暴露 REST 端点。
spring.http.server.enabled=false

# REST 监听端口;${spring.dubbo.server.protocols} 下的 key 即 dubbo-go 协议名。
# REST 在 20003(20000/20001/20002 留给 Triple / Dubbo / JSON-RPC 兄弟,便于四者同机共存)。
spring.dubbo.server.protocols.rest.port=20003

# etcd 注册中心,只在 ${spring.dubbo.registries} 定义一次:key 是逻辑注册中心 ID
# (类型默认取 key)。角色通过 ${...registry-ids} 按 ID 引用;只有一个注册中心时
# 两个角色都不设。与 docker-compose.yml 一致。
spring.dubbo.registries.etcdv3.address=127.0.0.1:2379
```

## 运行

先起注册中心:

```bash
docker compose up -d      # 或 docker-compose up -d
```

终端 A —— 启动 provider(长驻,注册进 etcd):

```bash
go run ./provider
```

终端 B —— 启动 consumer(从 etcd 发现并调用):

```bash
go run ./consumer
```

consumer 预期输出:

```
Response from discovered provider: Hello, Dubbo-Go!
```

或一键冒烟(自动起 etcd + provider、跑 consumer、清理):

```bash
bash scripts/smoke-test.sh
```

## 可观测

starter-dubbo **内置 metrics 与 tracing 且默认开启**;本示例在
`conf/app.properties` 里把它们显式打开,并在 `docker-compose.yml` 里配好一整套
本地后端栈,让 metrics、traces、logs 三种信号都能端到端看到。这一切**无需任何
代码**,只是在共享的 `Instance` bean 之上做配置。

> **仅 provider 侧。** 与 Triple/Dubbo/JSON-RPC 兄弟不同,starter-dubbo 的
> client bean 不支持 REST 协议,所以本示例的 consumer 是**独立的裸 dubbo-go
> `client.NewClient` main()**,完全不接入 Go-Spring 装配。它没有被埋点:
> 没有 consumer `conf/`、没有 consumer metrics 端口、没有 consumer OTel 上报、
> 没有 consumer 日志文件。下面所有内容都是关于 **provider** 的。模块根下的
> 单份 `conf/app.properties` 也只有 provider 会加载(其 `init()` 在
> `gs.Run()` 之前 chdir 到模块根)。

| 信号    | 产生方                                       | 后端            | 在哪看                                             |
| ------- | -------------------------------------------- | --------------- | ------------------------------------------------- |
| Metrics | dubbo-go Prometheus exporter,端口 `:9090`    | Prometheus      | UI http://127.0.0.1:9099(查 `up`、`dubbo_*`)     |
| Traces  | dubbo-go OTel → OTLP/gRPC `127.0.0.1:4317`   | Jaeger          | UI http://127.0.0.1:16686(服务 `rest-demo`)      |
| Logs    | go-spring `log` → `logs/provider.log`         | Loki(Promtail) | Loki HTTP API,端口 `:3100`(见下方查询)          |

### 架构与原理

provider(和裸 consumer)跑在**宿主机**上(由 scripts/smoke-test.sh 构建并运行
provider);所有后端跑在**容器**里(docker-compose.yml)。正是这条宿主机↔容器
边界,让三种信号各自走了略微不同的路径:

```
        宿主机 (provider + 裸 consumer)         DOCKER (docker-compose.yml)
  ┌────────────────────────────────┐        ┌──────────────────────────────┐
  │ provider                       │  注册/ │ etcd            :2379         │
  │   REST (HTTP/1)    :20003      │◀─发现─▶│   服务注册中心                 │
  │   /metrics (HTTP)  :9090       │        │                               │
  │   OTel SDK ─┐                  │        │ Prometheus      :9099 (UI)    │
  │   日志文件  │                  │        │ Jaeger    :4317 / :16686 (UI) │
  │ consumer(裸客户端,未埋点)             │ Loki            :3100         │
  │   一次性调用后退出              │        │ Promtail (采集 /var/log/app)  │
  └─────────────┼──────────────────┘        └──────────────────────────────┘
                │
   (1) METRICS —— 拉:   Prometheus ──每 5s GET /metrics──▶ provider :9090
                       (经 host.docker.internal 访问宿主机)
   (2) TRACES  —— 推:   OTel SDK ──OTLP/gRPC 上报 span──▶ Jaeger :4317 ─▶ :16686 UI
   (3) LOGS    —— 采集+推: provider ─写▶ logs/provider.log ◀─bind-mount─ Promtail
                       Promtail ──HTTP push──▶ Loki :3100 ─▶ 查询 API
```

**(1) Metrics —— 拉(pull)模型。** starter-dubbo 内置的 Prometheus registry
(由 `spring.dubbo.metrics.*` 开启)起了一个纯 HTTP 端点,按需渲染当前的
counter/gauge 值 —— 它自己**不主动推**。provider 在 `:9090` 提供。主动方是
Prometheus:按 `scrape_interval`(这里 5s)发 `GET /metrics`,解析后按时间戳
存下。因为 Prometheus 在容器里而目标在宿主机上,`docker/prometheus.yml` 把
目标写成 `host.docker.internal:9090`(Linux 经 `extra_hosts` 映射,macOS/Windows
原生支持)。指标是**懒注册**的,`dubbo_*` 只有在首次 RPC **之后**才出现 ——
所以冒烟脚本先调用再断言。

**(2) Traces —— 推(push)模型。** dubbo-go 的 OTel 集成(由
`spring.dubbo.tracing.*` 开启)在 provider 侧给每次 RPC 包一个 span。进程内
的 batch span processor 缓冲后经 **OTLP/gRPC** 上报到配置里的 endpoint
(`127.0.0.1:4317`),也就是 Jaeger 映射的 collector 端口。主动方是**应用** ——
Jaeger 存下并在 `:16686` UI 以服务 `rest-demo` 展示。
`mode=always`/`ratio=1.0` 全采样,单次调用也能看到。(裸 consumer 不产生
客户端 span。)

**(3) Logs —— 先采集再推。** 应用侧日志**从不走网络**。go-spring 的 `log`
模块(`FileLogger` + `JSONLayout`)把结构化 JSON 行写到 provider 进程工作目录
下的 `logs/provider.log` —— provider 在 `gs.Run()` 之前 chdir 到模块根,因此这个
相对 `logs/` 解析为 `contrib/dubbo-go/rest/logs/`。该 `logs/` 目录被**只读
bind-mount** 进 Promtail 容器的 `/var/log/app`(docker-compose.yml)。
Promtail(`docker/promtail-config.yml`)采集 `*.log`,用 positions 文件记录读取
偏移,给每行打上 `job="rest-demo"` 与 `filename` 标签,再把批次**推**给
Loki 的 `:3100` HTTP API。

以上全部只是**配置** —— 没有任何应用代码直接碰 Prometheus、OTel 或 Loki。

```properties
# metrics(Prometheus)—— 独立于已关闭的 HTTP server 单独提供
spring.dubbo.metrics.enable=true
spring.dubbo.metrics.port=9090
spring.dubbo.metrics.path=/metrics

# tracing(OTel → Jaeger,走 OTLP/gRPC);mode=always,单次调用也会被采样
spring.dubbo.tracing.enable=true
spring.dubbo.tracing.exporter=otlp-grpc
spring.dubbo.tracing.endpoint=127.0.0.1:4317
spring.dubbo.tracing.insecure=true

# logging —— 结构化 JSON 写到 logs/provider.log,由 Promtail 采集进 Loki
logging.logger.root.type=FileLogger
logging.logger.root.layout.type=JSONLayout
logging.logger.root.dir=logs
logging.logger.root.file=provider.log
```

### 手动验证(逐步)

`scripts/smoke-test.sh` 只断言**端点存活**(provider 暴露 `dubbo_*` 指标、
RPC 打通、无后端容器崩溃),并**不**等待数据真正落到 Prometheus/Jaeger/Loki。
下面的步骤让你逐个信号手动复核。请在**栈仍运行**且**至少发起过一次 RPC**之后
执行。

> 所有 `curl`/`open` 目标都是 `127.0.0.1`。若你的 shell 导出了 HTTP 代理,请先把
> `127.0.0.1,localhost` 加进 `no_proxy`/`NO_PROXY`,否则本地请求会被路由到代理而
> 返回空。

**Step 0 —— 起栈并发起调用。**

```bash
docker compose up -d          # 或 docker-compose up -d
go run ./provider &           # 终端 A:长驻
go run ./consumer             # 终端 B:发起一次 Greet 调用后退出
```

预期:

```
Response from discovered provider: Hello, Dubbo-Go!
```

**Step 1 —— provider 暴露 `dubbo_*` 指标。** 指标是懒注册的,所以只有在
Step 0 的调用**之后**才会有数据行。

```bash
curl -s http://127.0.0.1:9090/metrics | grep '^dubbo_provider_requests_total{'
```

**Step 2 —— Prometheus 已抓到 provider。** UI 在 `:9099`(其容器端口
`9090` 被重映射以避免与 provider 的 `:9090` 冲突)。

```bash
# 抓取目标健康(值 "1" = up)
curl -s -G 'http://127.0.0.1:9099/api/v1/query' \
  --data-urlencode 'query=up{job="rest-provider"}'

# dubbo 指标已进入 Prometheus
curl -s -G 'http://127.0.0.1:9099/api/v1/query' \
  --data-urlencode 'query=dubbo_provider_requests_total'
```

或打开 UI:

```bash
open http://127.0.0.1:9099
```

**Step 3 —— trace 已到达 Jaeger。**

```bash
curl -s 'http://127.0.0.1:16686/api/services'
curl -s 'http://127.0.0.1:16686/api/traces?service=rest-demo&limit=10'
```

或打开 Jaeger UI,选服务 `rest-demo`,点 *Find Traces*:

```bash
open http://127.0.0.1:16686
```

**Step 4 —— 日志已到达 Loki(经 Promtail)。**

```bash
# Promtail 正在采集 provider.log
curl -s -G 'http://127.0.0.1:3100/loki/api/v1/label/filename/values'

# 查询最近一小时的实际 JSON 日志行
END=$(date +%s)000000000; START=$(($(date +%s)-3600))000000000
curl -s -G 'http://127.0.0.1:3100/loki/api/v1/query_range' \
  --data-urlencode 'query={job="rest-demo"}' \
  --data-urlencode "start=$START" --data-urlencode "end=$END" \
  --data-urlencode 'limit=5'
```

预期 —— filename 列表含 `/var/log/app/provider.log`,查询返回
`"status":"success"`,含一条或多条 JSON 日志行的 stream。

**Step 5 —— 日志文件在磁盘上存在。**

```bash
ls logs/
head -1 logs/provider.log
```

预期 —— 只有 `provider.log` 一个文件(裸 consumer 未埋点,无 `consumer.log`),
以及结构化 JSON 行。

**Step 6 —— 无后端崩溃。**

```bash
docker compose ps        # 或 docker-compose ps
```

预期 —— 五个容器全部 `Up`:

```
contrib-dubbo-go-rest-etcd         Up
contrib-dubbo-go-rest-jaeger       Up
contrib-dubbo-go-rest-loki         Up
contrib-dubbo-go-rest-prometheus   Up
contrib-dubbo-go-rest-promtail     Up
```
