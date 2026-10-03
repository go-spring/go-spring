# starter-milvus 使用说明 — 参考手册

详细使用参考。概述见 [README_CN.md](README_CN.md)。所有行为声明均已对照 starter 源码（`starter.go`、`config.go`、`client.go`、`guard.go`、`observe.go`、`health.go`）与可运行的
[example/](example/) 核验——下文方括号为 file:line 抽查点。**Milvus 语义与 milvus-sdk-go v2 API 属于
[Milvus 官方文档](https://milvus.io/docs/install-go.md)
（[SDK](https://github.com/milvus-io/milvus-sdk-go)）**——本文只写 go-spring 增量。

**激活条件**：出现任意 `spring.milvus.instances.*` key（模块为 `OnProperty("spring.milvus")`，前缀匹配
[starter.go:36]）。每个 `spring.milvus.instances.<name>` 条目创建一个名为 `<name>` 的
`*StarterMilvus.Client` bean，外加名为 `milvus:<name>` 的健康指示器。

**诚实的边界声明**：自逐 RPC 治理守卫落地（guard.go——装在 SDK dial options 上的
gRPC 客户端拦截器）起，所有 Milvus RPC 都被透明保护（限流/熔断/隔舱/重试/超时 + 故障
注入），调用点零改动、无 opt-in。starter 只负责**声明**每个 RPC 是什么（`observe.go`：
span 名、`db.client` 指标前缀、`db.system`/`db.operation` 标签、完整方法路径作为
span/日志 detail），**发射**交给 resilience 层——executor 内唯一能看到整次调用（含重试）
的位置——因此每次调用都会产出调用级 `db.client.operation.duration` 直方图、尝试级
`db.client.attempt.duration` 直方图，以及打上 `_app_milvus_access` 标签的访问日志。本
starter 内没有任何自建逐 RPC span 或埋点；未受保护流量也没有独立的逐 RPC trace 层——
治理关闭时 executor 退化为**只观测**的执行器：限流/熔断/重试/超时不再生效，但观测照旧
发射（调用级/尝试级直方图 + `_app_milvus_access` 访问日志），消失的只有 `resilience.*`
outcome 指标。健康指示器仍是声明的存活信号（§2.2、§6）——除非实例置 `health=false`，否则注册。

---

## 1. 完整工程示例

单服务、单 Milvus 实例、经 actuator 暴露健康。文件树：

```
demo/
├── go.mod
├── main.go
├── service.go
└── conf/
    └── app.properties
```

**go.mod**（关键依赖）：

```
require (
    github.com/milvus-io/milvus-sdk-go/v2 v2.4.2
    go-spring.org/spring           v1.3.x
    go-spring.org/log              v0.1.x
    go-spring.org/starter-milvus   latest
    go-spring.org/starter-actuator latest   // 可选：readiness 端点
)
```

**main.go**：

```go
package main

import (
    "go-spring.org/spring/gs"
    _ "go-spring.org/starter-actuator"
    _ "go-spring.org/starter-milvus"
    _ "demo/service"
)

func main() { gs.Run() }
```

**service.go** — 注入 wrapper，跑一次真实向量往返（与冒烟验证过的
[example/main.go](example/main.go) 同构）：

```go
package service

import (
    "context"

    "github.com/milvus-io/milvus-sdk-go/v2/entity"
    "go-spring.org/spring/gs"
    StarterMilvus "go-spring.org/starter-milvus"
)

type Service struct {
    // 永远注入 wrapper 类型 *StarterMilvus.Client。它内嵌 SDK 的
    // client.Client 接口，NewCollection/Insert/Search/Flush/... 被原样提升。
    Client *StarterMilvus.Client `autowire:"a"`
}

func init() {
    gs.Provide(func(s *Service) gs.Runner {
        return func(ctx context.Context) error {
            _ = s.Client.NewCollection(ctx, "docs", 8) // 已存在则报错；见 §5
            _, err := s.Client.Insert(ctx, "docs", "",
                entity.NewColumnInt64("id", []int64{1}),
                entity.NewColumnFloatVector("vector", 8, [][]float32{
                    {0.1, 0.2, 0.3, 0.4, 0.5, 0.6, 0.7, 0.8},
                }))
            if err != nil {
                return err
            }
            _ = s.Client.Flush(ctx, "docs", false)
            _ = s.Client.LoadCollection(ctx, "docs", false)
            sp, _ := entity.NewIndexFlatSearchParam()
            res, err := s.Client.Search(ctx, "docs", nil, "", []string{"id"},
                []entity.Vector{entity.FloatVector{0.1, 0.2, 0.3, 0.4, 0.5, 0.6, 0.7, 0.8}},
                "vector", entity.L2, 1, sp)
            if err != nil {
                return err
            }
            _ = res // results[0].IDs 即 top-1（example 断言 id=1）
            return nil
        }
    })
}
```

**conf/app.properties** — 完整面（复制自
[example/conf/app.properties](example/conf/app.properties)，另加 actuator）：

```properties
# --- milvus 实例 "a" ---------------------------------------------------------
spring.milvus.instances.a.addr=127.0.0.1:19530
spring.milvus.instances.a.database=default
# 集群开启鉴权时才需要：
#spring.milvus.instances.a.username=root
#spring.milvus.instances.a.password=Milvus

# --- actuator（readiness 折叠 milvus:a）--------------------------------------
spring.actuator.addr=:9370
```

**验证**（先起 Milvus——[example 的 docker-compose.yml](example/docker-compose.yml)
拉起 etcd + minio + milvus standalone）：

```bash
docker compose -p gs-milvus-demo up -d   # milvus 启动很慢（约 60s）
go run .                                 # 19530 不可达则启动期 fail-fast（需 ping=true；默认关闭）
curl -s :9370/readyz | grep milvus       # 组件 "milvus:a" UP
grep -E 'round trip|Milvus' app.log      # 上面的 search 已返回
```

---

## 2. 装配与时序

### 2.1 bean 生命周期

```
import starter-milvus
  └─ gs.Module(OnProperty("spring.milvus"))：出现任意 spring.milvus.instances.* key 即触发
        └─ conf.BindEach(p, "${spring.milvus}") → 每个 <name> 条目一份 Config
              ├─ addr 由 expr tag 在绑定期校验非空 [config.go:26]
              ├─ Provide(newClient).Name(<name>).Destroy((*Client).Destroy) [starter.go:38-44]
              └─ Provide health.Indicator，名为 "milvus:<name>"，
                   TagArg(<name>) 选中对应 *Client，Export(gs.As[health.Indicator]())
                   [starter.go:47-49]

gs.Run()
  ├─ 构造 newClient [starter.go:69]：NewClient(ctx, c, cloud.ClientParams{...})
  ├─ NewClient [client.go:82] 一次组装完拨号 + 守卫 + 治理：
  │   client.NewClient(ctx, {Address, Username, Password, DBName,
  │   DialOptions: guardDialOptions(slot)}) gRPC 拨号（守卫拦截器随拨号装上，
  │   治理应用之前为透传）
  ├─ 随后定身份并应用治理：
  │   service = ServiceLabel("milvus", addr) →
  │   params.ExecutorFor("milvus", service)——构造期把注入的 `*resilience.Manager` /
  │   `*fault.Injector` 打包成 cloud.ClientParams →（fault 包裹 mgr.ClientExecutorFor）→
  │   slot.apply——此后每个 RPC 都过守卫；零值 params → 只观测的 Unmanaged executor
  ├─ 仅 ping=true：fail-fast 探针 HealthCheck（ListCollections 一次），出错 → Destroy()+启动失败
  │   [starter.go:89]——地址/凭据错，进程到不了 "serving"
  ├─ readiness：指示器周期性重复 HealthCheck（同一个 ListCollections 探针）
  └─ SIGTERM → Destroy [client.go:94]：exec.Close() 后 o.client.Close() 关闭 gRPC 连接
```

没有 `Init` 钩子：`newClient` 先把客户端装配完整（拨号 + 守卫 + 治理），随后才探测，
探针失败则 bean 根本不会创建——依赖它的 bean 不会装配到一个死 client 上。

### 2.2 一次操作的逐层走读（只写真实存在的层）

`Search(ctx, "docs", ...)` 恰好穿过**两**层：

1. wrapper 结构体——`Client` 内嵌裸 `client.Client` [client.go]，其完整方法集被提升，
   方法层无拦截；守卫在其下方的 gRPC 层生效。
2. milvus-sdk-go → gRPC client → **守卫拦截器**（executor：限流/熔断/隔舱/重试/超时
   外包 resilience observer，最外层 fault 注入）→ 服务端。

这就是全部，外加守卫。**守卫是 gRPC 拦截器链** [guard.go]：`NewClient` 传入自定义
dial options，SDK 自己的 `DefaultGrpcOpts`（keepalive、连接退避、2GB 收包上限）被先补
回、再追加守卫拦截器（unary + stream 建流）——是叠加不是替换。拦截器读每 client 一个
slot，该 slot 由 `NewClient` 内部创建、用 `params.ExecutorFor("milvus", "milvus:<addr>")`
应用并保持私有——它不能出现在导出的签名里（拨号时 dial options 已固定，所以构造器把
拨号和应用两步一并收进来）——执行器由构造期打包的
`*resilience.Manager` 与 `*fault.Injector` 构建
（零值 params → 只观测的 `Unmanaged` executor，透传；治理在客户端构造时即应用，
所以 `newClient` 的 fail-fast 探针在 slot 应用之后才跑，治理开启时探针本身也过守卫）。
拦截器同时是**声明 seam**：
它在 `exec.Execute` 之前把 RPC 身份放到调用方 ctx 上
（`observability.WithOperation(ctx, operation(method))`），executor 内的发射器在 Execute
入口读取它——那是唯一能看到整次调用（含重试）的位置——据此命名 span、`db.client.*` 指标
与访问日志。声明若放进 executor 的 fn 内（逐次尝试）则无人读取，故置于此处、executor 之外。
collection/index/search/insert
等全部 RPC 零改动过守卫，与其他 NoSQL starter 的透明逐请求口径一致。本 starter 自身发出的
另一信号是健康指示器（非逐调用）：`milvus:<name>` 除非实例置 `health=false`，否则注册；探针直接走裸 client，即 `HealthCheck` 的一次
`ListCollections` 往返，同时校验连通与鉴权 [health.go]。

---

## 3. 逐 key 行为参考

所有 key 位于 `spring.milvus.instances.<name>.` 下——经 `conf.BindEach` 按实例前缀绑定（不是
starter-Pool 的绝对属性规则）。已用 `grep -rhoE 'value:"[^"]+"'` 双向核对，表格覆盖
每个 tag。

| Key | 类型 | 默认值 | 行为/联动 | 配错后果 |
|-----|------|--------|-----------|----------|
| `addr` | string | — | **必填**，expr tag `$ != ''` 校验非空 [config.go:26]。Milvus gRPC 端点 `host:19530`。⚠ TLS 由 SDK 经地址 scheme 表达——本 starter 无 TLS 配置块。 | 缺失/空 → 拨号前绑定期报错。host/port 错 → 首次使用时（`ping=true` 时为构造期 fail-fast 探针）失败，启动中止。 |
| `database` | string | `default` | 作为 `DBName` 传给 `client.NewClient` [starter.go:65]。 | 库不存在 → 首次使用时（`ping=true` 时为 ListCollections 探针）启动期报错。 |
| `username` | string | `""` | 鉴权凭据；集群开鉴权时两半必须成对设置。⚠ 只设 `username` 不设 `password`（或反之）会被静默发送一半。 | 配错 → 首次使用时（`ping=true` 时为启动探针）失败，携带服务端鉴权错误。 |
| `password` | string | `""` | 见 `username`。 | 见 `username`。 |
| `ping` | bool | `false` | 启动连通探活：true 时构造期跑一次 `HealthCheck`（一次 `ListCollections`），出错则启动失败，恢复 fail-fast。默认关闭，使尚未就绪的 server 不阻塞启动。 | `ping=true` 且 server 已挂 → 启动报 `milvus: startup probe failed`。 |
| `health` | bool | `true` | 本实例是否贡献 `health.Indicator`（名 `milvus:<name>`）供 actuator 就绪/启动探测。置 false 可把该实例排除在聚合健康报告之外。 | `health=false` → `/readyz` 无 `milvus:<name>` 组件。 |

无 `driver` 注册表、无 `mode`（单机/集群是服务端拓扑）、无服务发现、无 otel key——
治理（resilience + fault）经共享 `spring.governance.*` 规则由逐 RPC 守卫消费，没有 milvus 专属
key——这就是全部面。

---

## 4. 验证与故障演练

### 4.1 经 actuator 看健康

```bash
curl -s :9370/readyz | grep milvus       # "milvus:a": UP
docker stop gs-milvus-demo-milvus-standalone-1
curl -s :9370/readyz                    # 下一轮探测翻 DOWN（503）
```

组件错误体原样携带 gRPC 错误——据此区分连通问题与鉴权问题（如 `Unavailable` vs
`Unauthenticated`）。

### 4.2 fail-fast 探针（启动时服务端已挂，`ping=true`）

```bash
docker compose -p gs-milvus-demo down && go run .   # 实例需 ping=true
# 构造期非零退出（向死端口 ListCollections）——进程永远到不了 "serving"
# 默认 ping=false 时启动成功，改为首个 RPC 才失败
```

### 4.3 确认"关治理只停保护、不停观测"

```bash
grep _app_milvus_access app.log              # 治理关闭时访问日志照旧 —— 观测不随治理关停
curl -s :9370/metrics | grep 'resilience\.'  # 治理关闭时为空：outcome 指标是治理动作的产物
```

治理（cloud/governance + `spring.governance.*` 规则）关闭时，限流/熔断/重试/超时都不生效，
但**观测仍在**：发射点在治理 executor 之外，`db.client.operation.duration`、
`db.client.attempt.duration` 两个直方图与 `_app_milvus_access` 访问日志照旧产出（直方图还需
`starter-otel` 安装 provider）。真正消失的只有 `resilience.*` outcome 指标。非经容器（零值
`ClientParams`）时 executor 退化为 `resilience.Unmanaged`：只观测 + 一次性"无保护生效"告警。
Milvus 服务端自身的指标在服务端的 `:9091`（example compose 已暴露），不经本 client。

### 4.4 往返冒烟（与 example/check.sh 同构）

```bash
grep "round trip" app.log    # check.sh grep 的 marker（"Milvus round trip OK: hit id= [1]"）
```

---

## 5. 排障表

| 症状 | 可能原因 | 处置 |
|------|----------|------|
| 启动期构造函数失败，gRPC `Unavailable`/超时 | Milvus 未就绪（启动慢）或 `addr` 错 | 等 19530 端口（`(exec 3<>/dev/tcp/127.0.0.1/19530)`），修 `addr`。 |
| 启动期报 `Unauthenticated` | 开了鉴权但 `username`/`password` 缺失或错误 | 两半都配；⚠ 只配一半会被静默忽略。 |
| 启动期 list collections 失败但服务端可达 | `database` 不存在 | `database` 指向已存在的库（Milvus ≥2.3）。 |
| 重启后 `NewCollection` 失败 | 上次运行已建同名集合 | 先 drop，或容忍该错误（example 的 check.sh 用固定名）。 |
| Search 结果为空 | 查询前漏了 `Flush` + `LoadCollection`（SDK 语义） | 先 flush 再 load，同 example/main.go:80-85。 |
| 查询正常但健康 DOWN | 指示器的 `ListCollections` 需要与 client 相同的库/鉴权 | 看 /readiness 里组件的错误体。 |
| Milvus 操作无 trace/指标/访问日志 | 流量绕开守卫（直接用原生 client）；治理关闭**不**会如此——关治理只停保护、观测仍在 | 走 client 的受 guard 覆盖的方法（Query/Search 等）；直方图另需 `starter-otel`。 |

## 6. 设计体检表

| 指标 | 数值 |
|------|------|
| 配置 key 总数 | 6 个 tag（全部生效） |
| 其中必填 | 1（`addr`） |
| quickstart 前置外部依赖 | compose 内 3 个（etcd + minio + milvus standalone） |
| "注意/坑" 条数 | 3（鉴权成对、TLS 在地址里、TLS/鉴权 dial option 无逃生口） |

设计嫌疑清单（审计台账——保留并扩充）：

- `schema.json` 放在 example/ 而非模块根（家族不对称：其他
  starter 放根目录）。
- 健康指示器每实例默认注册（`health=false` 可关）；启动探活为 opt-in（`ping=true`）——即
  redigo 拆成 `health.enabled`/`startup-ping` 的那两个旋钮。
- 无 TLS key——SDK 的 TLS 经地址表达；而且当前不给任何 dial option，用户想配 TLS
  只能 fork `newClient`，starter 内也无文档说明。
- ~~缺失埋点~~ 已解决：`observe.go` 声明每个 RPC 的身份，resilience 层经守卫拦截器发射
  `db.client.*` 信号与访问日志。仍开放：Config 上无 TLS/鉴权 `DialOptions` 逃生口——
  应用想加自己的拦截器（鉴权 token、自定义 trace）只能 fork `newClient`。
