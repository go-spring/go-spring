# dubbo-go — Dubbo/Hessian2(Go-Spring 风格)

[English](README.md) | [中文](README_CN.md)

一个使用 **经典 Dubbo 协议** —— TCP 传输 + **Hessian2** 序列化 —— 的
[Dubbo-go](https://dubbo.apache.org/zh-cn/overview/mannual/golang-sdk/)
`GreetService` 示例,并通过可复用的 **starter-dubbo** 模块以 Go-Spring 的方式装配:
由它提供 `gs.Server` 适配器,`gs.Run()` 驱动生命周期,provider 只是一个
`ServiceRegister` bean,协议与注册中心都取自 `conf/app.properties`,而不是
写死在 `main()` 里。

与 [`../triple`](../triple) 中的 Triple 版本不同:经典 Dubbo 协议在 dubbo-go v3
里**没有 protobuf IDL,也没有代码生成器** —— 服务就是纯 Go 结构体,注册时通过
反射读取其导出方法签名,通信线上用 Hessian2 序列化。因此,经典 Dubbo 协议是与
Java Dubbo 服务互通的最佳路径(Java Dubbo 原生就用这一协议);对纯 Go 的新业务
仍推荐 Triple。

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
  │ → dubbo://<host>:20001                   │
┌─┴──────────┐                        ┌──────┴─────┐
│  provider  │◀── Dubbo (Hessian2) ───│  consumer  │
│ gs.Run()   │      Greet(name)       │  一次性调用 │
│ :20001     │──────────────────────▶│  断言后退出 │
└────────────┘       echo name        └────────────┘
```

## 目录结构

```
contrib/dubbo-go/dubbo/
├── idl/greet.go             # 「IDL」:接口名与方法名常量
├── idl/gen-code.sh          # no-op —— 经典 Dubbo 无 IDL codegen
├── provider/handler.go      # GreetProvider + StarterDubbo.ServiceRegister bean(server 由 starter-dubbo 提供)
├── provider/main.go         # gs.Run(),长驻并注册到 etcd
├── provider/conf/app.properties  # provider 配置(server 角色 + 注册中心 + 可观测,metrics :9090)
├── consumer/main.go         # 通过 etcd 发现 provider,调用并断言后退出(Go-Spring 风格:client bean + gs.Run())
├── consumer/conf/app.properties  # consumer 配置(client 角色 + 注册中心 + 可观测,metrics :9091)
├── docker/                  # 后端栈的 Prometheus 与 Promtail 配置
├── docker-compose.yml       # 本地 etcd + Prometheus + Jaeger + Loki + Promtail
└── scripts/smoke-test.sh    # 冒烟脚本:起后端+provider,跑 consumer,断言后自动清理
```

## 如何生成

**什么都不用生成**。经典 Dubbo/Hessian2 在 dubbo-go v3 里没有 protobuf/thrift
IDL,也没有代码生成器 —— 服务表面就是一份手写的 Go 文件(`idl/greet.go`),
固定 Java 风格接口名与方法名,再加一份匹配签名的手写 provider 结构体。
执行 `./idl/gen-code.sh` 只会打印一行 “nothing to do”,只是为了与 Triple 兄弟目录
保持一致的入口。

如果你的服务用到非基本类型,需要通过 `hessian.RegisterPOJO(&MyStruct{})`
把类型注册进 Hessian2,让 Go↔Java 的类型映射在启动时就绪。本示例只用
`string`,不需要额外注册。

## 与 Triple 的对比

| 关注点        | Triple(`../triple`)              | Dubbo/Hessian2(本模块)                                |
| ------------- | -------------------------------- | ------------------------------------------------------ |
| 传输          | HTTP/2                            | 原始 TCP                                              |
| 负载          | protobuf                          | Hessian2                                               |
| IDL           | `.proto` + `protoc-gen-go-triple` | 无 —— 手写 Go 结构体                                   |
| 跨语言可达    | 任何 gRPC/Triple 客户端           | Java Dubbo(原生)、任何支持 Hessian2 的运行时         |
| 客户端调用    | 类型化桩(`svc.Greet(ctx, req)`) | 反射式(`conn.CallUnary(ctx, args, resp, "Greet")`)   |
| 何时选它      | 纯 Go 新业务                       | 与既有 Java Dubbo 服务互通                             |

## 配置

```properties
# 关闭内置 HTTP server:provider 只暴露 Dubbo 端点,consumer 无 server 运行。
spring.http.server.enabled=false

# 注册中心只在这一处定义(${spring.dubbo.registries})。map 驱动:key 是逻辑
# 注册中心 ID;未给 `protocol` 时类型默认取 key。角色不再内联定义注册中心,而是
# 通过 ${...registry-ids} 按 ID 引用。此处只定义一个注册中心,两个角色都不设
# registry-ids,于是 provider(server)与 consumer(client)默认都用它。
# 与 docker-compose.yml 一致。
spring.dubbo.registries.etcdv3.address=127.0.0.1:2379

# Provider 协议监听;${spring.dubbo.server.protocols} 下的 key 即 dubbo-go 协议名。
# 经典 Dubbo 在 20001(20000 留给 Triple 兄弟,便于两者同机共存)。
spring.dubbo.server.protocols.dubbo.port=20001
```

Dubbo **client** 由 starter-dubbo 作为默认 bean(`__default__`)提供,由
`${spring.dubbo.client}` 加顶层 `${spring.dubbo.registries}` 构建;consumer 直接
autowire 它并 dial 服务。可在 `${spring.dubbo.client.instances}` 下声明多个命名
client(bean 名 = map key)。若要运行两个同类型注册中心,给各自一个不同的 map-key
ID 并显式设置 `protocol`,例如 `spring.dubbo.registries.bj.protocol=etcdv3` /
`...sh.protocol=etcdv3`,再让各角色用 registry-ids 挑选(如
`spring.dubbo.client.registry-ids=bj`)。

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

或一键冒烟(自动起后端 + provider、跑 consumer、断言、清理):

```bash
bash scripts/smoke-test.sh
```

## 可观测

starter-dubbo **内置 metrics 与 tracing 且默认开启**;本示例在各角色各自的
`conf/app.properties` 里把它们显式打开,并在 `docker-compose.yml` 里配好一整套
本地后端栈,让 metrics、traces、logs 三种信号都能端到端看到。这一切**无需任何
代码**,只是在共享的 `Instance` bean 之上做配置。

| 信号    | 产生方                                       | 后端            | 在哪看                                             |
| ------- | -------------------------------------------- | --------------- | ------------------------------------------------- |
| Metrics | dubbo-go Prometheus exporter,端口 `:9090`    | Prometheus      | UI http://127.0.0.1:9099(查 `up`、`dubbo_*`)     |
| Traces  | dubbo-go OTel → OTLP/HTTP `127.0.0.1:4318`   | Jaeger          | UI http://127.0.0.1:16686(服务 `dubbo-demo`)     |
| Logs    | go-spring `log` → `logs/` 下 JSON 文件        | Loki(Promtail) | Loki HTTP API,端口 `:3100`(见下方查询)          |

### 架构与原理

provider 与 consumer 跑在**宿主机**上(由 scripts/smoke-test.sh 构建并运行);所有后端跑在
**容器**里(docker-compose.yml)。正是这条宿主机↔容器的边界,让三种信号各自走了
略微不同的路径:

```
        宿主机 (provider + consumer)             DOCKER (docker-compose.yml)
  ┌────────────────────────────────┐        ┌──────────────────────────────┐
  │ provider                       │  注册/ │ etcd            :2379         │
  │   dubbo/Hessian2   :20001      │◀─发现─▶│   服务注册中心                 │
  │   /metrics (HTTP)  :9090       │        │                               │
  │   OTel SDK ─┐                  │        │ Prometheus      :9099 (UI)    │
  │   日志文件  │ ─┐               │        │ Jaeger    :4318 / :16686 (UI) │
  │ consumer    │  │               │        │ Loki            :3100         │
  │   /metrics  │  │    :9091      │        │ Promtail (采集 /var/log/app)  │
  │   OTel SDK ─┤  │               │        └──────────────────────────────┘
  │   日志文件  │  │               │
  └─────────────┼──┼───────────────┘
                │  │
   (1) METRICS — 拉:   Prometheus ──每 5s GET /metrics──▶ provider :9090
                       (经 host.docker.internal 访问宿主机)
   (2) TRACES  — 推:   OTel SDK ──OTLP/HTTP 上报 span──▶ Jaeger :4318 ─▶ :16686 UI
   (3) LOGS    — 采集+推: 进程 ─写▶ ../logs/*.log ◀─bind-mount─ Promtail
                       Promtail ──HTTP push──▶ Loki :3100 ─▶ 查询 API
```

**(1) Metrics —— 拉(pull)模型。** starter-dubbo 内置的 Prometheus registry(由
`spring.dubbo.metrics.*` 开启)起了一个纯 HTTP 端点,按需渲染当前的
counter/gauge 值 —— 它自己**不主动推**。provider 在 `:9090` 提供,consumer 在
`:9091`。主动方是 Prometheus:按 `scrape_interval`(这里 5s)发 `GET /metrics`,
解析文本格式响应,给每个样本打上时间戳存下来。因为 Prometheus 在容器里而目标在
宿主机上,`docker/prometheus.yml` 把目标写成 `host.docker.internal:9090`
(Linux 经 `extra_hosts` 映射,macOS/Windows 原生支持)。指标是**懒注册**的,
`dubbo_*` 只有在首次 RPC **之后**才出现 —— 所以冒烟脚本先调用再断言。

**(2) Traces —— 推(push)模型。** dubbo-go 的 OTel 集成(由
`spring.dubbo.tracing.*` 开启)给每次 RPC 包一个 span。进程内的 batch span
processor 缓冲后,经 **OTLP/HTTP** 上报到配置里的 endpoint(`127.0.0.1:4318`),
也就是 Jaeger 映射出来的 OTLP/HTTP collector 端口。这里主动方是**应用** —— 它把 span 推给
collector;Jaeger 存下并在 `:16686` UI 以服务 `dubbo-demo` 展示。
`mode=always`/`ratio=1.0` 对每个 span 全采样,单次调用也能看到。

> **为什么用 OTLP/HTTP 而不是 OTLP/gRPC?** dubbo-go v3.3.1 的 otlp-grpc 导出器
> (`otel/trace/otlp/exporter.go` 的 `newGrpcExporter`)**忽略了 `insecure` 配置**,
> 总是以 TLS 拨号,而 Jaeger all-in-one 的 collector 是明文的 —— 握手会被判为
> "bogus greeting" 而断开,span 永远送不到。otlp-http 导出器正确处理 `insecure`,
> 所以本示例走 OTLP/HTTP(`:4318`)。另外,若你的 shell 导出了
> `OTEL_EXPORTER_OTLP_ENDPOINT`,OTel SDK 会把它并入导出 URL(可能得到 404),
> 跑之前请先 `unset` 它。

**(3) Logs —— 先采集再推。** 应用侧的日志**从不走网络**。go-spring 的 `log`
模块(`FileLogger` + `JSONLayout`)把结构化 JSON 行写到宿主机的
`../logs/<role>.log`。这个 `logs/` 目录被**只读 bind-mount**进 Promtail 容器的
`/var/log/app`(docker-compose.yml)。Promtail(`docker/promtail-config.yml`)
采集 `*.log`,用 positions 文件记录读取偏移,给每行打上 `job="dubbo-demo"` 和来源
`filename` 标签,再把批次**推**给 Loki 的 `:3100` HTTP API。Loki 只索引标签;
JSON 正文通过手动步骤里展示的标签选择器可查。

以上全部只是**配置** —— 没有任何应用代码直接碰 Prometheus、OTel 或 Loki,都是
叠加在 starter-dubbo 从 `spring.dubbo.*` 构建出的那个共享 `Instance` bean 之上。

```properties
# metrics(Prometheus)—— 独立于已关闭的 HTTP server 单独提供
spring.dubbo.metrics.enable=true
spring.dubbo.metrics.port=9090
spring.dubbo.metrics.path=/metrics

# tracing(OTel → Jaeger,走 OTLP/HTTP);mode=always,单次调用也会被采样
spring.dubbo.tracing.enable=true
spring.dubbo.tracing.exporter=otlp-http
spring.dubbo.tracing.endpoint=127.0.0.1:4318
spring.dubbo.tracing.insecure=true

# logging —— 结构化 JSON 写到 logs/provider.log,由 Promtail 采集进 Loki
logging.logger.root.type=FileLogger
logging.logger.root.layout.type=JSONLayout
logging.logger.root.dir=../logs
logging.logger.root.file=provider.log
```

**两个进程、两份配置。** provider 与 consumer 各读**自己**的
`conf/app.properties`(分别在 `provider/`、`consumer/` 下),因此会冲突的值直接
写成不同字面量即可:provider 的 metrics 在 `:9090`、日志写 `provider.log`,
consumer 的 metrics 在 `:9091`、日志写 `consumer.log`,两者都写进同一个模块根
`logs/`(经 `../logs`)供 Promtail 采集。无需任何环境变量覆盖,直接跑即可:

```bash
go run ./consumer
```

### 手动验证(逐步)

`scripts/smoke-test.sh` 只断言**端点存活**(provider 暴露 `dubbo_*` 指标、RPC 打通、无后端
容器崩溃),并**不**等待数据真正落到 Prometheus/Jaeger/Loki。下面的步骤让你逐个
信号手动复核,并写清每一步应当看到什么。请在**栈仍运行**且**至少发起过一次
RPC**之后执行。

> 所有 `curl`/`open` 目标都是 `127.0.0.1`。若你的 shell 导出了 HTTP 代理,请先把
> `127.0.0.1,localhost` 加进 `no_proxy`/`NO_PROXY`,否则本地请求会被路由到代理而
> 返回空。

**Step 0 —— 起栈并发起调用。**

```bash
docker compose up -d          # 或 docker-compose up -d
go run ./provider &           # 终端 A:长驻
go run ./consumer             # 终端 B:发起 21 次 Greet 调用(1 次规范 + 20 次批量)
```

预期:consumer 先打印规范行,再打印批量汇总,然后退出。

```
Response from discovered provider: Hello, Dubbo-Go!
Sent 21 greetings (1 canonical + 20 batch)
```

**Step 1 —— provider 暴露 `dubbo_*` 指标。** 指标是懒注册的,所以只有在 Step 0
的调用**之后**才会有数据行。

```bash
curl -s http://127.0.0.1:9090/metrics | grep '^dubbo_provider_requests_total{'
```

预期 —— `Greet` 方法的计数器,值为 `21`(Step 0 的全部调用):

```
dubbo_provider_requests_total{application_name="dubbo-demo",group="",interface="com.example.GreetService",method="Greet",version="",...} 21
```

**Step 2 —— Prometheus 已抓到 provider。** 查 Prometheus 的 HTTP API(UI 在
`:9099`,其容器端口 `9090` 被重映射以避免与 provider 的 `:9090` 冲突)。

```bash
# a) 抓取目标健康(值 "1" = up)
curl -s -G 'http://127.0.0.1:9099/api/v1/query' \
  --data-urlencode 'query=up{job="dubbo-provider"}'

# b) dubbo 指标已进入 Prometheus
curl -s -G 'http://127.0.0.1:9099/api/v1/query' \
  --data-urlencode 'query=dubbo_provider_requests_total'
```

预期 —— `"status":"success"`,且结果的 `"value"` 以 `"21"` 结尾(`up` 查询则以
`"1"` 结尾,即健康):

```json
{"status":"success","data":{"resultType":"vector","result":[{"metric":{"__name__":"up","job":"dubbo-provider","instance":"host.docker.internal:9090","role":"provider","service":"dubbo-demo"},"value":[...,"1"]}]}}
```

或打开 UI 查 `up` / `dubbo_*`:

```bash
open http://127.0.0.1:9099
```

**Step 3 —— trace 已到达 Jaeger。**

```bash
# a) 服务已注册
curl -s 'http://127.0.0.1:16686/api/services'

# b) 现在存在多条 trace,每条含一个 "Greet" span
curl -s 'http://127.0.0.1:16686/api/traces?service=dubbo-demo&limit=30'
```

预期 —— 服务列表含 `dubbo-demo`,traces 载荷里有多条 trace(每次 RPC 一条),
每条含一个 `operationName` 为 `Greet` 的 span:

```json
{"data":["dubbo-demo"],"total":1,"limit":0,"offset":0,"errors":null}
```

或打开 Jaeger UI,选服务 `dubbo-demo`,点 *Find Traces*:

```bash
open http://127.0.0.1:16686
```

**Step 4 —— 日志已到达 Loki(经 Promtail)。**

```bash
# a) Promtail 正在采集两个文件
curl -s -G 'http://127.0.0.1:3100/loki/api/v1/label/filename/values'

# b) 查询最近一小时的实际 JSON 日志行
END=$(date +%s)000000000; START=$(($(date +%s)-3600))000000000
curl -s -G 'http://127.0.0.1:3100/loki/api/v1/query_range' \
  --data-urlencode 'query={job="dubbo-demo"}' \
  --data-urlencode "start=$START" --data-urlencode "end=$END" \
  --data-urlencode 'limit=5'
```

预期 —— (a) 列出两个文件,(b) 返回 `"status":"success"`,含一条或多条 JSON 日志
行的 stream:

```json
{"status":"success","data":["/var/log/app/consumer.log","/var/log/app/provider.log"]}
```

**Step 5 —— 日志文件在磁盘上存在。** 两个进程都写进共享的模块根 `logs/`(各自经
`../logs`);这正是 Promtail 挂载的目录。

```bash
ls logs/
head -1 logs/provider.log
```

预期 —— 两个文件,以及结构化 JSON 行:

```
consumer.log  provider.log
{"level":"info","time":"...","fileLine":"...","tag":"_app_def","msg":"ready",...}
```

**Step 6 —— 无后端崩溃。**

```bash
docker compose ps        # 或 docker-compose ps
```

预期 —— 五个容器全部 `Up`:

```
contrib-dubbo-go-dubbo-etcd         Up
contrib-dubbo-go-dubbo-jaeger       Up
contrib-dubbo-go-dubbo-loki         Up
contrib-dubbo-go-dubbo-prometheus   Up
contrib-dubbo-go-dubbo-promtail     Up
```
