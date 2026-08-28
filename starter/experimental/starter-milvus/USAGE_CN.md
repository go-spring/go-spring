# starter-milvus 使用说明 — 参考手册

详细使用参考。概述见 [example/README_CN.md](example/README_CN.md)（starter 级 README 目前放在
example/ 下）。所有行为声明均已对照 starter 源码（`starter.go`、`config.go`、`client.go`、`health/health.go`）与可运行的
[example/](example/) 核验——下文方括号为 file:line 抽查点。**Milvus 语义与 milvus-sdk-go v2 API 属于
[Milvus 官方文档](https://milvus.io/docs/install-go.md)
（[SDK](https://github.com/milvus-io/milvus-sdk-go)）**——本文只写 go-spring 增量。

**激活条件**：出现任意 `spring.milvus.*` key（模块为 `OnProperty("spring.milvus")`，前缀匹配
[starter.go:29]）。每个 `spring.milvus.<name>` 条目创建一个名为 `<name>` 的
`*StarterMilvus.Client` bean，外加名为 `milvus:<name>` 的健康指示器。

**诚实的边界声明**：自逐 RPC 治理守卫落地（guard.go——装在 SDK dial options 上的
gRPC 客户端拦截器）起，所有 Milvus RPC 都被透明保护（限流/熔断/隔舱/重试/超时 + 故障
注入），调用点零改动、无 opt-in；实例的 `observability.*` 块驱动守卫执行的观测（span +
outcome 指标 + 访问日志）。治理关闭时 executor 是透明 no-op——未受保护流量没有独立的
逐 RPC trace 层。健康指示器仍是常开的存活信号（§2.2、§6）。

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
[example/example.go](example/example.go) 同构）：

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
    // client.Client 接口，NewCollection/Insert/Search/Flush/... 原样提升。
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
spring.milvus.a.addr=127.0.0.1:19530
spring.milvus.a.database=default
# 集群开启鉴权时才需要：
#spring.milvus.a.username=root
#spring.milvus.a.password=Milvus

# --- actuator（readiness 折叠 milvus:a）--------------------------------------
spring.actuator.addr=:9370
```

**验证**（先起 Milvus——[example 的 docker-compose.yml](example/docker-compose.yml)
拉起 etcd + minio + milvus standalone）：

```bash
docker compose -p gs-milvus-demo up -d   # milvus 启动很慢（约 60s）
go run .                                 # 19530 不可达则启动期 fail-fast
curl -s :9370/readyz | grep milvus       # 组件 "milvus:a" UP
grep -E 'round trip|Milvus' app.log      # 上面的 search 已返回
```

---

## 2. 装配与时序

### 2.1 bean 生命周期

```
import starter-milvus
  └─ gs.Module(OnProperty("spring.milvus"))：出现任意 spring.milvus.* key 即触发
        └─ conf.BindEach(p, "${spring.milvus}") → 每个 <name> 条目一份 Config
              ├─ addr 由 expr tag 在绑定期校验非空 [config.go:26]
              ├─ Provide(newClient).Name(<name>).Init((*Client).Init)
              │       .Destroy((*Client).Destroy) [starter.go:31-33]
              └─ Provide health.Indicator，名为 "milvus:<name>"，
                   TagArg(<name>) 选中对应 *Client，Export(gs.As[health.Indicator]())
                   [starter.go:35-37]

gs.Run()
  ├─ 构造 newClient [client.go]：client.NewClient(ctx, {Address, Username, Password,
  │   DBName, DialOptions: guardDialOptions(slot)}) gRPC 拨号（守卫拦截器随拨号装上，
  │   Init 之前为透传）；随后 fail-fast 探针：ListCollections 一次，出错 →
  │   cl.Close() + 启动失败——地址/凭据错，进程到不了 "serving"
  ├─ Init [client.go]：resource = ResourceLabel("milvus", addr) →
  │   fault.WrapExecutor(resilience.ExecutorFor(resource)) →
  │   resilobserve.WrapExecutor(exec, "milvus", Observability) → slot.arm——此后每个
  │   RPC 都过守卫；治理关闭 → no-op executor
  ├─ readiness：指示器周期性重复同一个 ListCollections 探针
  └─ SIGTERM → Destroy [client.go]：exec.Close() 后 o.Client.Close() 关闭 gRPC 连接
```

`Init` 只为武装守卫（executor 需要字段注入的 `observability` 块）；拨号 + 探针仍在
构造函数里，探针失败则 bean 根本不会创建——依赖它的 bean 不会装配到一个死 client 上。

### 2.2 一次操作的逐层走读（只写真实存在的层）

`Search(ctx, "docs", ...)` 恰好穿过**两**层：

1. wrapper 结构体——`Client` 内嵌 `client.Client` [client.go]，方法层无拦截；守卫在
   其下方的 gRPC 层生效。
2. milvus-sdk-go → gRPC client → **守卫拦截器**（executor：限流/熔断/隔舱/重试/超时 +
   故障注入，外面包着 resilience observer）→ 服务端。

这就是全部，外加守卫。**守卫是 gRPC 拦截器链** [guard.go]：`newClient` 传入自定义
dial options，SDK 自己的 `DefaultGrpcOpts`（keepalive、连接退避、2GB 收包上限）被先补
回、再追加守卫拦截器（unary + stream 建流）——是叠加不是替换。拦截器读每 client 一个
slot，`Init` 用 `fault.WrapExecutor(resilience.ExecutorFor("milvus:<addr>"))` 外包
`resilobserve.WrapExecutor` 武装它（治理关闭 → no-op 透传；构造期的 fail-fast 探针在
Init 之前跑，正依赖该透传）。collection/index/search/insert 等全部 RPC 零改动过守卫，
与其他 NoSQL starter 的透明逐请求口径一致。实际存在的可观测性另有健康指示器：
`milvus:<name>` 恒注册，探针即 wrapper 自带的 `Health(ctx)`，一次 `ListCollections`
往返同时校验连通与鉴权 [health/health.go]。

---

## 3. 逐 key 行为参考

所有 key 位于 `spring.milvus.<name>.` 下——经 `conf.BindEach` 按实例前缀绑定（不是
starter-Pool 的绝对属性规则）。已用 `grep -rhoE 'value:"[^"]+"'` 双向核对，表格覆盖
每个 tag。

| Key | 类型 | 默认值 | 行为/联动 | 配错后果 |
|-----|------|--------|-----------|----------|
| `addr` | string | — | **必填**，expr tag `$ != ''` 校验非空 [config.go:26]。Milvus gRPC 端点 `host:19530`。⚠ TLS 由 SDK 经地址 scheme 表达——本 starter 无 TLS 配置块。 | 缺失/空 → 拨号前绑定期报错。host/port 错 → 构造期 fail-fast 探针失败，启动中止。 |
| `database` | string | `default` | 作为 `DBName` 传给 `client.NewClient` [client.go:45]。 | 库不存在 → fail-fast 探针（ListCollections）启动期报错。 |
| `username` | string | `""` | 鉴权凭据；集群开鉴权时两半必须成对设置。⚠ 只设 `username` 不设 `password`（或反之）会被静默发送一半。 | 配错 → 启动期探针失败，携带服务端鉴权错误。 |
| `password` | string | `""` | 见 `username`。 | 见 `username`。 |
| `observability` | group | 空 | 字段注入到 wrapper，由 `Init` 读取以观测守卫执行（`resilobserve.WrapExecutor`）：每个受守卫 RPC 的 span、outcome 指标（`resilience.*`）、访问日志。`off` 只静默日志信号。 | 期待治理关闭时（未受保护流量）的逐 RPC span → 什么都不发；executor 是 no-op。 |

无 `driver` 注册表、无 `mode`（单机/集群是服务端拓扑）、无服务发现、无 otel key——
治理（resilience + fault）经共享 `govern.*` 规则由逐 RPC 守卫消费，没有 milvus 专属
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

### 4.2 fail-fast 探针（启动时服务端已挂）

```bash
docker compose -p gs-milvus-demo down && go run .
# 构造期非零退出（向死端口 ListCollections）——进程永远到不了 "serving"
```

### 4.3 确认守卫静默（治理关闭时的诚实演练）

```bash
curl -s :9370/metrics | grep -i milvus   # 治理关闭时本 starter 无任何输出
grep _app_ app.log | grep -i milvus      # 治理关闭时不发访问日志
```

治理（starter-governance + `govern.*` 规则）关闭时两条都应为空：守卫 executor 是
no-op，本 starter 发出的唯一信号是健康组件。治理开启后 `resilience.*` outcome 指标
与守卫访问日志随之出现。Milvus 服务端自身的指标在服务端的 `:9091`（example compose
已暴露），不经本 client。

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
| Search 结果为空 | 查询前漏了 `Flush` + `LoadCollection`（SDK 语义） | 先 flush 再 load，同 example/example.go:80-85。 |
| 查询正常但健康 DOWN | 指示器的 `ListCollections` 需要与 client 相同的库/鉴权 | 看 /readiness 里组件的错误体。 |
| Milvus 操作无 trace/指标/访问日志 | 治理关闭（executor 是 no-op），或 `observability.level=off` | 开启治理（starter-governance + `govern.*` 规则）；服务端 :9091 指标补充。 |

## 6. 设计体检表

| 指标 | 数值 |
|------|------|
| 配置 key 总数 | 5 个 tag（全部生效） |
| 其中必填 | 1（`addr`） |
| quickstart 前置外部依赖 | compose 内 3 个（etcd + minio + milvus standalone） |
| "注意/坑" 条数 | 3（鉴权成对、TLS 在地址里、无埋点） |

设计嫌疑清单（审计台账——保留并扩充）：

- ~~`observability.*` 只绑定从不读取~~ 已接上（Init 读它驱动守卫执行的观测）。
- starter 级 README/DESIGN/schema.json 放在 example/ 而非模块根（家族不对称：其他
  starter 放根目录）。
- 健康指示器无关闭开关（缺 `health.enabled` 类 key；与 redigo 家族不对称）。
- 无 TLS key——SDK 的 TLS 经地址表达；而且当前不给任何 dial option，用户想配 TLS
  只能 fork `newClient`，starter 内也无文档说明。
- ~~缺失埋点~~ 已解决：守卫拦截器装在 dial options 上（guard.go）；TLS/auth 类附加
  dial option 仍无逃生口，要加只能 fork `newClient`。
- errutil import 靠占位 `var _ = errutil.Explain` [starter.go:43-45] 保活，注释称
  "留给未来 driver dispatch"——预判性 API 残留。
