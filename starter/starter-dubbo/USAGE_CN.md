# starter-dubbo 使用说明 — 参考手册

详细使用文档，概览见 [README_CN.md](README_CN.md)。所有行为声明均已对照 starter 源码
（`config.go`、`client.go`、`server.go`、`dync.go`、`fault.go`、`loadtest.go`、`internal/logger`、
`internal/mapconfig`）与可运行的 [example/](example/) 核实。**Dubbo/Triple 自身语义（协议、
注册中心行为、cluster/loadbalance 策略、序列化、filter 语义）见
[dubbo-go 官方文档](https://dubbo-go.github.io/)**——下文只写 go-spring 的增量：绑定面、bean
装配、生命周期、热更新、治理与可观测联动。

**激活**：整个模块以 `spring.dubbo.registries` 属性的字面存在为开关（config.go:52）。这个 key
拼写错误会让 starter 整体静默失效——不报错、不出 bean。在此之上：server bean 还要求
`spring.dubbo.provider.enabled` 未设置或为 `true`（server.go:33-34），**并且**至少存在一个
`ServiceRegister` bean；client bean 跟随 `Instance`（client.go:29）。

---

## 1. 完整工程示例

单进程内 provider + consumer（即 example/ 的形态），带 metrics、tracing 与运行时故障注入。
外部前置：一个注册中心——example 用 etcd（`127.0.0.1:2379`），nacos / zookeeper / polaris 均可。

```
demo/
├── go.mod
├── main.go
├── provider.go
├── consumer.go
├── idl/greet.proto              # 生成 stub 的源头
├── idl/proto/greet.pb.go        # protoc 产物
├── idl/proto/greet.triple.go    # protoc-gen-go-triple 产物
└── conf/app.properties
```

**idl/greet.proto**：

```protobuf
syntax = "proto3";
package greet;
option go_package = "demo/idl/proto;greet";

message GreetRequest  { string name = 1; }
message GreetResponse { string greeting = 1; }

service GreetService {
  rpc Greet(GreetRequest) returns (GreetResponse);
}
```

用 `protoc --go_out=. --go-triple_out=. idl/greet.proto` 生成（dubbo-go 的 Triple 插件，见
[dubbo-go protobuf 指南](https://dubbo-go.github.io/)）。生成文件入库。

**main.go**：

```go
package main

import (
    _ "demo/consumer"
    _ "demo/provider"

    "go-spring.org/spring/gs"
    _ "go-spring.org/starter-dubbo" // 必需：blank import 激活 starter
)

func main() { gs.Run() }
```

**provider.go** —— 注意两条注册规则：直接在 `init()` 顶层调用，以及显式接口转换（Go 不会把
类型参数推断成接口）：

```go
package provider

import (
    "context"

    _ "dubbo.apache.org/dubbo-go/v3/imports" // 必需的 side-effect import，由用户代码引入
    greet "demo/idl/proto"
    StarterDubbo "go-spring.org/starter-dubbo"
)

type GreetProvider struct{}

func (s *GreetProvider) Greet(ctx context.Context, req *greet.GreetRequest) (*greet.GreetResponse, error) {
    return &greet.GreetResponse{Greeting: req.Name}, nil
}

func init() {
    // name "greet" 即 ${spring.dubbo.provider.services.greet} 的 key。
    // 必须直接在 init() 里调用：bean 的 file:line 调试信息取自调用方栈帧
    // （见 §2.2）——不要包进任何 helper 函数。
    StarterDubbo.RegisterService("greet", greet.RegisterGreetServiceHandler,
        greet.GreetServiceHandler(&GreetProvider{})) // 显式转换，不做推断
}
```

**consumer.go** —— 类型化 stub 可注入到任意位置：

```go
package consumer

import (
    "context"

    _ "dubbo.apache.org/dubbo-go/v3/imports"
    greet "demo/idl/proto"
    StarterDubbo "go-spring.org/starter-dubbo"
    "go-spring.org/spring/gs"
)

func init() {
    // 绑定 ${spring.dubbo.consumer.references.greet}，产出一个 *greet.GreetService bean。
    StarterDubbo.RegisterReference("greet", greet.NewGreetService)
}

type Caller struct {
    Greet *greet.GreetService `autowire:"?"` // 具名 bean；去掉 "?" 则变为必需注入
}
```

**conf/app.properties** —— 上面用到的完整配置面：

```properties
# 关闭 gs 内建 HTTP server；端口由 dubbo 掌管。
spring.http.server.enabled=false

# --- application（必需）------------------------------------------------------
spring.dubbo.application.name=demo-app

# --- registries（必需；模块激活 key）------------------------------------------
spring.dubbo.registries.etcd.protocol=etcdv3
spring.dubbo.registries.etcd.address=127.0.0.1:2379

# --- protocols（全局；全部省略则回退 Triple :20000）---------------------------
spring.dubbo.protocols.tri.name=tri
spring.dubbo.protocols.tri.port=20000

# --- provider 侧 --------------------------------------------------------------
spring.dubbo.provider.services.greet.interface=greet.GreetService
spring.dubbo.provider.services.greet.version=1.0.0
spring.dubbo.provider.services.greet.filter=loadtest,fault   # starter 注入的 filter（§2.4）

# --- consumer 侧 ---------------------------------------------------------------
spring.dubbo.consumer.references.greet.interface=greet.GreetService
spring.dubbo.consumer.references.greet.version=1.0.0
spring.dubbo.consumer.references.greet.timeout=3s

# --- 可观测（默认值；坑见 §3）--------------------------------------------------
spring.dubbo.metrics.enable=true
spring.dubbo.metrics.port=9090
spring.dubbo.tracing.enable=false        # 默认 true 且 exporter=stdout——见 §5
```

**验证**（纯 URL 直连的进程内冒烟不需要注册中心——example 即如此）：

```bash
cd example && ./check.sh           # 40s 看门狗；server 自测后对自己发 SIGTERM
# 或手动：
go run . -manual                   # 终端 1
go run check_client.go             # 终端 2 → "OK: Dubbo RPC verified"
curl -s 127.0.0.1:9090/metrics | grep dubbo   # dubbo-go Prometheus 指标
```

注意：example/conf/app.properties 指向 etcd，因此即便"进程内"冒烟也需要活的注册中心
（体检表嫌疑 10）。

---

## 2. 装配与时序

### 2.1 Bean 生命周期

```
import starter-dubbo
  ├─ init()：安装 dubbo-go 日志桥（internal/logger，双 facade 都装）
  ├─ init()：mapconfig 装为 dubbo-go DynamicConfiguration（config.go:47）
  ├─ gs.Provide(NewInstance)  — 条件：OnProperty("spring.dubbo.registries")          [config.go:49-52]
  ├─ gs.Provide(NewClient)    — 条件：OnBean[*Instance]                              [client.go:27-30]
  ├─ gs.Module(provider.enabled 默认 true) → gs.Provide(NewSimpleDubboServer)
  │     以 gs.Server 导出 — 条件：OnBean[ServiceRegister] + OnBean[*Instance]        [server.go:32-44]
  ├─ gs.Provide(newDyncPoller) — 无条件；绑定 ${spring.dubbo.application}            [dync.go:40-42]
  └─ init()："fault" 与 "loadtest" 两个 filter 注册进 dubbo-go filter registry

gs.Run()
  ├─ 静态绑定 ${spring.dubbo}（仅此一次）→ DubboConfig → NewInstance
  │     （把 protocols、metrics、tracing、shutdown 挂到同一个 *dubbo.Instance 上；
  │      config.go:391-513；application.name 为空或 0 个 registry → 快速失败）
  ├─ RegisterReference bean 绑定 ${spring.dubbo.consumer.references.<n>} → 经 NewClient 产类型化 stub
  ├─ 装配顺序注意：Rooter（dyncPoller）先于 Runner（治理引擎）装配
  │     ——见 dync.go:90-94 注释；governance.OnReady 会补一次 poll 弥合时差
  ├─ SimpleDubboServer.Run：buildOptions(provider, protocols, registries) → d.NewServer()
  │     → regAll()：逐个调用 ServiceRegister bean → <-sig.TriggerAndWait() → svr.Serve()  [server.go:297-327]
  ├─ 就绪：Run 触发就绪信号后生效；Stop 时排水在途 RPC
  └─ SIGTERM：StopContext 关闭 done channel → Run 返回 → gs 完成关停序列
        （dubbo-go 自身的优雅停机时序来自 ${spring.dubbo.shutdown.*}）
```

### 2.2 调用方栈帧规则（RegisterService / RegisterReference）

`gs.Provide` 用 `runtime.Caller(skip)` → `SetFileLine` 抓取 file:line 调试信息
（spring/gs/internal/gs_bean/bean.go:372-376）。两个 helper 都用 `.Caller(2)` 钉死取帧
（server.go:360、client.go:175）：帧 0 = `Caller`，帧 1 = helper 自身，帧 2 = **你**。请在
你自己的 `init()`/包代码顶层直接调用；包进 helper 函数或在闭包深处调用都会移栈，bean 的
调试信息就指向错误位置。这只影响诊断信息（bean 描述、错误归因），不影响绑定——但栈帧错了
容器 dump 就废了。

### 2.3 Instance 模型（2026-07-24 重构）

`${spring.dubbo}` 只做**一次**静态绑定进 `DubboConfig`，由 `Instance` 门面持有
（config.go:51、351-354）。`NewClient`（client.go:34）与 `NewSimpleDubboServer`
（server.go:190）都从 `Instance` 取配置——`Registries()`、`Protocols()`、`Consumer()`、
`Provider()`、`NewServer()`、`NewClient()`（config.go:357-386）。因此 registries 与
protocols 是进程级全局：顶层定义一次，各角色用 `registry-ids` / `protocol-ids` 按名选取
（空 = 全选；未知 ID 快速失败——config.go:592-605）。

### 2.4 Filter 链 —— starter 注册了什么、装在哪

init 期注册两个 dubbo-go filter；都是**按 service 显式启用**——把名字加进 provider 的
`filter` key（逗号分隔，其余语义归 dubbo-go）：

- **`loadtest`**（loadtest.go:38）：从入站 dubbo attachment 读压测标记（string 与 []byte
  都处理），给 context 打标，使后续 filter 与你的服务实现里 `traffic.IsLoadTest(ctx)` 可用。
  请放在链的**最前**（源码注释，loadtest.go:31-34），标记要先于任何依赖它的层落位。
- **`fault`**（fault.go:40）：**每次调用**从治理 seam 解析 `fault.InjectorFor()`——可运行期
  热切换，治理缺席时透明直通（fault.go:50-65）。注入的失败以
  `result.RPCResult{Err: fault.ErrInjected}` 呈现。

Filter 在 Refer/导出时**冻结**（dync.go:60）——改 `filter` key 要重启；timeout/retries 不用
（§4.2）。

### 2.5 一次调用逐层走读（consumer → provider）

1. 业务代码调类型化 stub（`*greet.GreetService`），它由 `RegisterReference` 用共享
   `*client.Client` + 该 reference 的 `ReferenceOption` 组装（client.go:172-176）。
2. 跑 dubbo-go 的 consumer filter 链（`consumer.filter` /
   `consumer.references.<n>.filter` 所装）；出站压测标记由 cloud/governance/traffic 的
   carrier 注入完成——starter 的 `loadtest` filter 是入站侧配套（loadtest.go:44-47 注释）。
3. Cluster 策略 / loadbalance 选实例（dubbo-go 语义；URL 参数——这正是热更新通道能在运行期
   改动的那组，§4.2）。
4. Triple（或 dubbo/jsonrpc）传输把调用送到 provider 端口。
5. 服务端 filter 链：`loadtest` 打标、`fault` 可能注入，然后执行你 `RegisterService` 注册的
   handler；响应沿同一批 filter 返回。

---

## 3. 逐 key 行为参考

`spring.dubbo.*` 下共 169 个 `value` tag / 103 个去重 key 写法（约 159 个叶子）。每个 key
**对 dubbo-go 的含义**至多一句话——详见
[dubbo-go 配置文档](https://dubbo-go.github.io/)。下表是我们的绑定面：前缀、默认值、校验、
耦合。Map id（`registries.<id>`、`protocols.<id>`、`provider.services.<n>`、
`consumer.references.<n>`、`...methods.<m>`）自由命名，校验 `^[_a-zA-Z][a-zA-Z\d_-]*$`。
所有时长均为**字符串**（`"3s"`、`"10m"`）；无法解析或非正的时长被**静默丢弃**，绝不报错
（如 config.go:565-569）。

### 3.1 application（8 个 key）——`name` 为必需节点

| Key | 默认值 | 绑定行为 |
|-----|--------|----------|
| `name` | `dubbo.io` | 唯一硬必填：为空 → `NewInstance` 快速失败（config.go:393-395）。同时是应用级治理 label 与 dyncPoller 的 override key（dync.go:77-83）。 |
| `metadata-type` | `local` | `remote` 时切换 `dubbo.WithRemoteMetadata`（config.go:419-421）。 |
| `organization`/`module`/`owner` | `dubbo-go`/`sample`/`dubbo-go` | 仅非空时下发。 |
| `group`/`version`/`environment` | — | 非空时下发。 |

### 3.2 registries.<id>（13 个 key）——**模块激活节点**

| Key | 默认值 | 绑定行为 |
|-----|--------|----------|
| *（节点存在性）* | — | 字面量 `spring.dubbo.registries` 属性门控整个 starter（config.go:52）。≥1 条 entry，否则 `NewInstance` 失败（config.go:396-398）。 |
| `protocol` | — | nacos/etcdv3/polaris/xds/zookeeper/service-discovery-registry；为空回退 map key id（config.go:544-547）。 |
| `address` | — | 实际必填（registry 需要它）。 |
| `timeout` / `ttl` | 5s / 10s | 时长字符串；非法 → 静默丢弃。 |
| `weight` | 100 | 总是下发（`>=0`，config.go:571-573）。 |
| `simplified` / `preferred` / `zone` | false / false / — | 注册中心侧旋钮。 |
| `group` / `namespace` / `username` / `password` / `params` | — | 有值即下发。 |

### 3.3 protocols.<id>（4 个 key）

| Key | 默认值 | 绑定行为 |
|-----|--------|----------|
| `name` | `dubbo` | dubbo/rest/grpc/filter/jsonrpc/tri/registry；为空回退 id（config.go:517-520）。 |
| `port` | 0 | 0 交给 dubbo-go 选。 |
| `ip` / `params` | — | `params` 是 `map[string]string`——值类型 map，不是 `map[string]any`（绑定器限制，config.go:123-129 注释）。 |

⚠ 完全不配 protocols → server 回退单个 `tri` 监听 `:20000`（server.go:276-282）。

### 3.4 metadata-report ——已删除

曾有绑定（protocol/address/username/password/group/namespace/timeout）但从未转成任何
dubbo option——死配置，2026-08 删除。现在配 `spring.dubbo.metadata-report.*` 没有绑定
目标；需要远程元数据用 `application.metadata-type=remote`（经 registries 通道）。

### 3.5 provider（27 个 key）+ provider.services.<n>（26）+ methods.<m>（12）

provider 级默认值被所有导出服务继承；service 级字段覆盖之；两者在 server.go:50-132 与
196-294 翻译为 `server.ServerOption` / `ServiceOption`。只有非零/非空值下发（其余交给
dubbo-go 默认）。

要点：`registry-ids`/`protocol-ids` 必须引用已存在的 map key（快速失败）；`retries`
默认 `-1` = 未设置（保留 dubbo-go 自身默认）、`0` = 不重试、`>0` = 重试次数——所有层级
（provider/service/consumer/reference/method）语义统一，低于 `-1` 启动即报错；
`warmup` 是时长字符串；
`not-register=true` 导出但不发布；`adaptive-service(-verbose)` 对应各自 ServerOption。

### 3.6 consumer（19 个 key）+ consumer.references.<n>（19）+ methods.<m>（12）

客户端同构（consumer 级见 client.go:34-98，reference 级见 101-168）。这棵树被绑定**两次**
——一次静态供 client bean，一次 `gs.Dync` 供热更新（§4.2）。

| 坑 | 细节 |
|----|------|
| `check` | consumer 与 reference 两级默认均为 **true**（无提供者时快速失败）。迁移：过去依赖 reference 级旧默认 **false** 的引用需改设 `spring.dubbo.consumer.check=false`——dubbo-go v3 没有 reference 级"关闭检查"的选项，`references.<n>.check=false` 叠加 consumer check=true 只会在启动时 WARN（无法生效）。 |
| `protocol` | consumer 接受 tri/triple/jsonrpc/dubbo（client.go:39-46）；reference 接受 dubbo-go 支持的全部。 |
| `url` | 直连模式——该 reference 绕过注册中心。 |
| 分隔符混用 | provider/consumer 级用连字符（`tps-limit-rate`），service/reference/method 级用点（`tps.limit.rate`、`force.tag`）——同一概念两种拼法。**定位为规范化文档口径**（兼容保留）：service/reference/method 级的点号拼法刻意对齐 dubbo 自身 URL 参数名（热更新路径 dync.go 下发的就是 `tps.limit.rate` 等 URL 参数）；provider/consumer 级连字符遵循 starter 自身 key 风格。仅精确匹配（无宽松绑定），两种拼法按文档保留。 |

### 3.7 metrics（6 个 key）/ tracing（12 个 key）——⚠ 默认开启

| Key | 默认值 | 绑定行为 |
|-----|--------|----------|
| `metrics.enable` | **true** | `true` 挂载 dubbo-go 的 Prometheus 导出器。 |
| `metrics.port` / `metrics.path` | **9090** / `/metrics` | 与 actuator 并存的第二个指标端口——注意端口规划（约定：server 端口须显式配置）。 |
| `metrics.push-gateway-address` | — | 有值即启用 pushgateway 通路。 |
| `metrics.mode` / `metrics.namespace` | — | 已删除（2026-08）：绑定但从未生效，无对应 v3 metrics.Option。 |
| `tracing.enable` | **true** | 挂载 dubbo-go OTel tracing，exporter 默认 **stdout**——依赖前先看 §5。 |
| `tracing.exporter` / `endpoint` / `propagator` / `mode` / `ratio` / `insecure` | stdout / — / w3c / — / 1.0 / false | 下发给 dubbo-go `trace.Option`（config.go:453-472）。 |
| `tracing.name`/`serviceName`/`address`/`use-agent` | — | 已删除（2026-08）：旧版 jaeger 字段，从未转译，无 v3 Option。 |

### 3.8 shutdown（6 个 key）

时长字符串，**仅当至少一个字段有值**时才翻译为 `graceful_shutdown.Option`
（config.go:475-506，`anySet` 在 536-540）。`reject-handler`：任意非空值只是打开拒绝开关——
值本身被忽略（config.go:497-499）。`internal-signal=true`（默认）让 dubbo-go 自行响应信号。

### 3.9 包装字段说明（绝对 key 与前缀 key）

有些 starter 提供 `value:"${observability:=}"` 式包装字段，解析为**顶层绝对** key、不带实例
前缀。starter-dubbo **没有这种包装字段**：上文所有 key 都带 `spring.dubbo.*` 前缀、相对绑定
（仅有的顶层引用是三处字面量 tag 表达式 `spring.dubbo`、`spring.dubbo.consumer`、
`spring.dubbo.application`，见 config.go:51、dync.go:41、dync.go:67）。

---

## 4. 验证与故障演练

### 4.1 验证装配

```bash
grep -ri "dubbo server starting" logs/    # 到达 SimpleDubboServer.Run（server.go:314）
curl -s 127.0.0.1:9090/metrics | head     # dubbo-go Prometheus 指标（启用时）
grep -ri "_rpc_dubbo" logs/ | head        # 经日志桥转发的 dubbo-go 框架日志（§4.6）
```

### 4.2 热更新演练（timeout/retries，免重启）

`${spring.dubbo}` 为 `Instance` 做一次静态绑定——在构建/Refer 期消费的字段全部**冻结**：
protocols、registries、filter（dync.go:60）、serialization、interface/group 路由（它们构成
override key）。`${spring.dubbo.consumer}` 会**再**绑定为 `gs.Dync[DubboConsumer]`
（dync.go:67），`dyncPoller` 把其中可动态生效的子集以扁平 dubbo URL 参数推进内存配置中心
（mapconfig）：`timeout`、`retries`、`loadbalance`、`cluster`、`group`、`version`、
`serialization`、`sticky`、`force.tag`、`weight`，以及按方法
`methods.<m>.{timeout,retries,loadbalance,weight,sticky,tps.limit.*,execute.limit*}`
（dync.go:57-59、154-228）。

链路（DESIGN.md，已对码确认）：

```
属性变更（file/nacos/env）→ gs RefreshProperties → gs.Dync 原子换值
  → dyncPoller.OnChanged → poll() → 与上次快照 diff（无变化刷新跳过）
  → mapconfig.RefreshOverrideRules → dubbo-go consumerConfigurationListener /
    referenceConfigurationListener → invoker URL 更新 → 下次调用生效
```

演练：

1. 以 `spring.dubbo.consumer.references.greet.timeout=3s` 启动。
2. 改成 `100ms` 并触发刷新（如接了 actuator 则 `curl -X POST :9370/actuator/refresh`，
   或走你配置源的刷新路径）。
3. 观察慢调用立刻失败；override 落在 key `greet.GreetService:1.0.0:.configurators` 下——
   key 格式见 §4.3。

### 4.3 Override key 格式（演练前先核对）

consumer 级默认发布为 `<application.name>.configurators`（应用级 listener）；每个 interface
非空的 reference 发布为 `<interface>:<version>:<group>.configurators`——由
`colonSeparatedKey` 构造（dync.go:245-257）：version 为空或 `0.0.0` 哨兵时省略**但 `:`
分隔符永远写出**；group 同理。裸 interface 名永远匹配不上。推论：要 override 落地，
reference 的 `version`/`group` 必须与 provider 导出的一致。

### 4.4 治理合入通道（中心的动态超时）

可选；仅当引入 starter-govern 且 `govern.enabled=true` 时激活。poller 订阅两类治理资源
label（dync.go:263-264）：

- `dubbo:<application.name>` —— consumer 级默认
- `dubbo:<interface>:<version>:<group>` —— 按 reference（与 §4.3 同一冒号分隔 key）

`Policy.Timeout`（毫秒）与 `Policy.MaxRetries` 在 > 0 时覆盖 `timeout`/`retries` 参数
（dync.go:288-295）；注意 MaxRetries 映射到 dubbo 的 **cluster** 重试，不是 resilience 层
重试。顺序问题已处理：Rooter 先于 Runner 装配，引擎就绪后 `governance.OnReady` 补一次
poll（dync.go:90-99）。演练：在治理源里改 `govern.*` 超时，观察 reference override 免重启
重新下发。

### 4.5 故障演练（provider 侧，免重启）

1. 给 service 的 filter 链加 `fault`：`...services.greet.filter=loadtest,fault`。
2. 引入 starter-governance；经热源配置 `govern.fault.*`（rate/error/scope）。
3. `scope: loadtest` 时只有带压测标记的调用被烧——由带标记的上游注入出站 carrier
   （cloud/governance/traffic）来打标，或在专属环境用 `scope: real`。
4. 观察：consumer 收到注入错误；带标记调用在实现内 `traffic.IsLoadTest(ctx)` 为 true
   （loadtest filter 排最前）。

### 4.6 观测日志桥

引入 starter 即在 dubbo-go **两个** logger facade 下（含 gost/getty）安装同一个适配器——
internal/logger/logger.go init()。dubbo-go 框架的每行日志经 go-spring log 重放，tag 为
`_rpc_dubbo`（经 `log.RegisterRPCTag("dubbo", "")` 注册，internal/logger/logger.go:41）。像
任意 logger tag 一样在 `${logging.logger}` 下配置。已知取舍：该路径没有 ctx，无 trace-id
增强，caller file:line 指向桥本身。

---

## 5. 排障表

| 症状 | 可能原因 | 处置 |
|------|----------|------|
| 毫无动静；一个 dubbo bean 都没有 | `spring.dubbo.registries` 拼错/缺失——模块门（config.go:52） | 配上；该失败模式设计上即静默（wiring_test.go 有覆盖）。 |
| 启动失败：`${spring.dubbo.application.name} is required` | name 为空 | 设置 `spring.dubbo.application.name`（config.go:393-395）。 |
| 启动失败：registry id "x" is not defined | `registry-ids`/`protocol-ids` 引用了不存在的 map key | 对齐 `registries`/`protocols` 下的 id（config.go:592-605）。 |
| server 不启动、client 正常 | 无 `ServiceRegister` bean，或 `provider.enabled=false` | （在顶层）调用 `RegisterService`，或重新启用。 |
| 启动 panic 提及 OTel/stdout exporter | `tracing.enable=true`（默认）且 exporter 为 `stdout`、无 provider | 设 `spring.dubbo.tracing.enable=false` 或接 starter-otel / 真实 exporter。 |
| 9090 端口冲突 | dubbo-go metrics 默认开启（`:9090/metrics`） | `spring.dubbo.metrics.enable=false` 或显式换端口。 |
| timeout 热更新不落地 | version/group 不匹配 → 冒号分隔 key 错；或是 filter 类字段（冻结） | 让 reference 的 `version`/`group` 与 provider 一致；只有 URL 参数类字段可热更新（§4.2）。 |
| 时长 key 被静默忽略 | 值无法解析或非正 | 用 Go 时长字符串（`3s`、`10m`）——配置侧丢弃坏值不报错。 |
| bean 调试信息指向某个 helper 文件 | `RegisterService`/`RegisterReference` 被函数包了一层 | 在你自己的栈帧直接调用（§2.2）。 |
| dubbo 日志没进应用输出管道 | 未配置 go-spring logger | 桥只改路由；配置 `${logging.logger}`。 |

## 6. 设计体检表

| 指标 | 数值 |
|------|------|
| 配置 key（value tag） | 169 个 tag / 103 个去重写法（约 159 叶子） |
| 必填 | 1 个硬必填（`application.name`）+ `registries` ≥1 条作为模块门 |
| quickstart 前置外部依赖 | 1（注册中心） |
| "注意/坑"条数 | 10 |

设计嫌疑清单（交设计裁决）：

1. 配置面大到无法逐条文档化——本身就是信号。
2. 同一棵树重复绑定（静态 `Instance` + `Dync` consumer），冻结字段清单只存在于代码
   （dync.go:57-60）。
3. ~~连字符 vs 点号分隔符随嵌套层级不同~~——已按文档化规范口径解决（见 §3.6"分隔符
   混用"）：动态层级点号对齐 dubbo URL 参数、静态层级连字符为 starter 自身风格；兼容
   保留，仅精确匹配。
4. ~~`check` 默认值两级不同；`retries=0` 语义随层级不同~~——均已修复（2026-08）：
   `check` 两级默认 true（reference 级无法单独关闭的组合会 WARN）；`retries` 统一为
   -1=未设置 / 0=不重试 / >0=次数，低于 -1 启动报错。
5. ~~死配置：`metadata-report.*`、`provider.proxy`、`consumer.proxy`、`max_message_size`、
   `metrics.mode/namespace`、旧版 jaeger tracing 字段~~——全部删除（2026-08）；
   `shutdown.reject-handler` 保留：任意非空值打开拒绝开关（值本身被忽略——这正是文档
   化行为）。
6. tracing/metrics 默认开启且副作用意外（stdout exporter、9090 端口）。
7. 模块激活门控在 `spring.dubbo.registries` 的字面存在上——拼错 = 静默 no-op。
8. `RegisterService`/`RegisterReference` 的调用方栈帧敏感性——不允许包装。
9. reference 路径无端到端 example 覆盖（example 走直连 URL，不经注册发现）。
10. `check.sh` 声称"无外部服务"，但 example/conf/app.properties 依赖 etcd。
11. ~~wiring_test.go 的 "KNOWN BUG" 注释已过时（`map[string]string` 已修复）、protocols 块
    仍未测~~ ——`map[string]any` 绑定失败已修复（config.go:123-129 记录了该约束）；
    wiring_test.go 的 protocols 块仍缺测试。
12. ~~DESIGN.md 相对代码已过时~~——DESIGN.md 已于 2026-08 刷新对齐 config.go/dync.go；
    细节仍以本 USAGE 与源码为准。
