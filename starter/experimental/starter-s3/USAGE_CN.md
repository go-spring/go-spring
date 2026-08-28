# starter-s3 使用说明 — 参考手册

深度使用文档。概览见 [README.md](README.md)。所有行为声明均对照源码
（`starter.go`、`config.go`、`driver.go`、`client.go`、`command.go`、`health/health.go`、
`s3_test.go`）与可运行的 [example/](example/)（经 docker-compose 的 MinIO 冒烟验证）核实。
**对象存储操作语义（bucket/object API、保留策略、版本化）见
[minio-go 官方文档](https://github.com/minio/minio-go)** —— 本文只写 go-spring 的增量：
配置、装配、启动 fail-fast、逐请求可观测、resilience、健康检查。

**激活条件**：`gs.OnProperty("spring.s3")` 门控一个 gs.Module；每个 `spring.s3.<name>`
子树创建一个名为 `<name>` 的 `*Client` bean，外加一个名为 `s3:<name>` 的健康指示器。
不配置则不装配。

starter 通过 **minio-go** 说 S3 协议，因此一套配置面原生覆盖 MinIO 与 AWS S3，
也覆盖其他云的 S3 兼容端点（Aliyun OSS、Tencent COS 等）——兼容性说明见 README。

---

## 1. 完整工程示例

一个向 MinIO 存对象、带健康探测、逐请求可观测与治理防护的服务。文件树
（对应冒烟验证过的 [example/](example/)）：

```
demo/
├── go.mod
├── main.go
├── storage.go
└── conf/
    └── app.properties
```

**go.mod**（关键依赖）：

```
require (
    github.com/minio/minio-go/v7   latest   # 传递依赖，由 starter 引入
    go-spring.org/spring           v1.3.x
    go-spring.org/starter-s3       latest
    go-spring.org/starter-actuator latest   # 可选：readiness 端点
    go-spring.org/starter-otel     latest   # 可选：真实 span/metric 导出
    go-spring.org/starter-governance latest # 可选：retry/limiter/breaker/fault
)
```

**main.go**：

```go
package main

import (
    "go-spring.org/spring/gs"
    _ "go-spring.org/starter-actuator"
    _ "go-spring.org/starter-governance"
    _ "go-spring.org/starter-otel"
    _ "go-spring.org/starter-s3"
)

func main() { gs.Run() }
```

**storage.go** —— 应用的全部存储面：

```go
package storage

import (
    "context"

    "github.com/minio/minio-go/v7"
    "go-spring.org/spring/gs"

    starter "go-spring.org/starter-s3"
)

// Service 注入一个命名 client。Client 内嵌 *minio.Client，所有生成方法
// （PutObject、GetObject、StatObject……）原样提升。第二个端点是纯配置
// 变更加一个字段。
type Service struct {
    Client *starter.Client `autowire:"a"`
}

func init() {
    gs.Provide(&Service{}).Export(gs.As[gs.Rooter]())
}

// Put 演示一次上传；可观测/resilience transport 装在 client 内部，
// 因此这次调用已被 span、metric、访问日志与治理覆盖（见 §2.3）。
func (s *Service) Put(ctx context.Context, bucket, key string, b []byte) error {
    _, err := s.Client.PutObject(ctx, bucket, key,
        bytes.NewReader(b), int64(len(b)), minio.PutObjectOptions{ContentType: "text/plain"})
    return err
}
```

**conf/app.properties** —— 完整注释配置（在 example 配置上扩展）：

```properties
# --- s3 client "a"（每个 spring.s3.<name> 子树一个实例）-----------------------
spring.s3.a.endpoint=127.0.0.1:9000        # host:port，不带 scheme
spring.s3.a.access-key-id=minioadmin
spring.s3.a.secret-access-key=minioadmin
spring.s3.a.region=us-east-1
spring.s3.a.use-ssl=false
# path 风格对拒绝 virtual-host 寻址的 S3 兼容云更友好。
spring.s3.a.bucket-lookup=path

# 同集群的第二个 client，展示多实例装配
spring.s3.b.endpoint=127.0.0.1:9000
spring.s3.b.access-key-id=minioadmin
spring.s3.b.secret-access-key=minioadmin

# 逐请求可观测：优先配在实例前缀下；顶层 observability.* 仍作为回退面
# 生效（实例 key 逐字段覆盖它——见 §3）。
spring.s3.a.observability.level=brief
# spring.s3.a.observability.maxArgBytes=512
# spring.s3.a.observability.skipOps=GET /bucket/x

# --- actuator（把 s3:<name> 指示器并入 readiness）----------------------------
spring.actuator.addr=:9370

# --- governance（可选：s3:<endpoint> 资源下的 retry/limiter/breaker/fault）----
govern.source.file.path=conf/govern.yaml
```

本地依赖：`cd example && docker compose up -d`（MinIO :9000/:9001，另有 `initbucket`
任务创建 `go-spring-example` 桶）。

**验证**（与 example/check.sh 的断言同构）：

```bash
go run .                                                  # example 打印 "Object round trip OK:"
curl -i :9370/readyz ; curl -s :9370/readyz | grep -o 's3:a[^,}]*'   # 指示器健康
curl -s :9370/metrics | grep -E 's3.*client|duration' | head        # 逐请求指标（需 otel）
mc ls local/go-spring-example                              # 若有 mc，经 compose 网络查看
```

---

## 2. 装配与时序

### 2.1 Bean 生命周期

```
import starter-s3
  └─ init(): gs.Module(gs.OnProperty("spring.s3"), ...) —— 用 Module 而非 gs.Group，
        是为了给每个实例的 *Client 配一个同名 health.Indicator
        （源码注释，starter.go:32-36）

gs.Run()
  ├─ 对 spring.s3.* 做 conf.BindEach → 每个实例名一份 Config
  ├─ 每实例：
  │    ├─ r.Provide(newClient, IndexArg(1, ValueArg(c))).Name(name)
  │    │      .Init((*Client).Init).Destroy((*Client).Destroy).Caller(1)
  │    ├─ r.Provide(健康指示器).Name("s3:"+name).Export(health.Indicator)
  │    │      —— .Name 保证多实例 (Name,Type) 键唯一
  │    ├─ newClient：查 driverRegistry（默认 DefaultDriver）
  │    │      → DefaultDriver.CreateClient：静态凭据 + region + bucket-lookup
  │    │        + minio.Options 里的 dynamicTransport 占位
  │    │      → dynamicTransports.LoadAndDelete 把占位交给 wrapper
  │    ├─ fail-fast 探测：HealthCheck → ListBuckets —— 端点不可达或凭据被拒
  │    │      都会中止启动（starter.go:76-78）
  │    └─ gs 字段注入 Client.Observability 后调用 Init()：
  │          obs := observe.NewDB("s3", ...) → obsTransport（span+metric+访问日志）
  │          exec := fault.WrapExecutor(resilience.ExecutorFor("s3:<endpoint>"))
  │          exec := resilobserve.WrapExecutor(exec, "s3", ...)  // outcome span/计数
  │          dyn.Swap(resilience.NewRoundTripper(obsTransport, exec, → resource))
  ├─ Run / 服务：readyz 并入每个 s3:<name> 指示器（需 starter-actuator）
  └─ SIGTERM：Destroy() 关闭 resilience executor；minio 侧无会话可关
```

### 2.2 dynamicTransport 为何存在（源码理由）

minio-go 在构造时把 `http.Transport` 固定进 `minio.Options` 且不提供 setter，而
可观测策略要在 client 存在**之后**才被字段注入。因此 `DefaultDriver.CreateClient`
装一个薄的 `dynamicTransport`——原子 RoundTripper 间接层（RWMutex 守护而非
atomic.Value，因为活动的 tripper 是多种具体类型之一；见 client.go:104-114）——并按
返回的 client 为键登记进包级 `dynamicTransports sync.Map`。`newClient` 取出它
（`LoadAndDelete`），`Init` 再把真正的 observe+resilience transport 换进去。Init
运行前，请求直通 `http.DefaultTransport`。

### 2.3 一次上传的逐层走读

`PutObject(ctx, bucket, key, ...)`：

1. minio-go 用 SigV4 静态凭据签名，经配置的 transport 发 HTTP 请求——即被换入的
   resilience round-tripper。
2. Resilience round-tripper：请求进入按资源 `s3:<endpoint>` 解析的 executor——引入
   starter-governance 后 retry / rate-limit / circuit-breaker / bulkhead 生效（经治理
   中心可热切换），否则透明直通；进程级 fault 注入器（`fault.InjectorFor`，nil 安全）
   可为演练注入失败。`resilobserve.WrapExecutor` 为熔断跳闸、限流拒绝、隔舱拒绝发出
   outcome span + 调用计数 + 时长直方图 + 访问日志。
3. obsTransport（command.go:38）：以操作名 `"PUT /bucket/key"`（方法 + URL path）
   开 per-request observer span，跑底层 `http.DefaultTransport`，带错误结束 span ——
   span + 时长 metric + 访问日志都带该操作名（minio-go 自身无 OTel 钩子，starter 的
   transport 承载全部三个信号）。
4. 响应回卷：记录 span 属性/metric、按 observability 级别出日志行；minio-go 把
   object info 返回给调用方。

### 2.4 健康检查

`NewClientHealth`（health/health.go:33）用 `ListBuckets` 探测——同时验证端点可达
**与**凭据被接受。按实例以 `s3:<name>` 注册并导出为 `health.Indicator`，因此引入
starter-actuator 的应用无需额外接线即可把 S3 readiness 并入 `/readiness`。同一探测
也以 `StarterS3.HealthCheck(ctx, *Client) error` 导出，供临时探测使用。

---

## 3. 逐 key 行为参考

ctor 绑定的 `Config` key（config.go）带前缀 `spring.s3.<name>.*`。`observability`
另有顶层 `observability.*` 回退面（Client wrapper 字段）：实例 key 逐字段覆盖它；
绑定在未配置时也会填默认值（brief/512/无 skip），因此只有非默认的实例取值才算"已配置"（见 §6）。

| Key | 类型 | 默认值 | 行为 / 联动 | 配错后果 |
|-----|------|--------|-------------|----------|
| `endpoint` | string | — | **必填**（`expr:"$ != ''"`）。`host:port`，不带 scheme。 | 缺失/为空 → 启动期绑定校验报错。 |
| `access-key-id` | string | — | **必填**；静态 SigV4 凭据。 | 缺失 → 启动报错；错误 → ListBuckets fail-fast 探测拒绝启动。 |
| `secret-access-key` | string | — | **必填**；静态 SigV4 凭据。 | 同上。 |
| `session-token` | string | — | 可选第三要素（临时凭据）。 | 永久凭据配过期 token → 启动探测处签名被拒。 |
| `region` | string | us-east-1 | 传给 minio.Options 的 bucket region。 | region 错误 → region 敏感端点上出现签名/重定向错误（对不敏感的 MinIO 可能过了探测、之后按桶失败）。 |
| `use-ssl` | bool | false | 对端点启用 HTTPS。 | 对只收 TLS 的端点配 false（或对明文端点配 true）→ 启动探测失败。 |
| `bucket-lookup` | string | auto | `auto` \| `virtual-host`/`dns`（别名，`BucketLookupDNS`）\| `path`。 | 部分 S3 兼容云只支持 path 风格 → 风格错导致逐请求寻址失败；未知值 → 启动报错列出合法值。 |
| `observability` | struct | brief | 逐请求 span+metric+访问日志级别（`spring.s3.<name>.observability.level` / `.maxArgBytes`（默认 512）/ `.skipOps`）；顶层 `observability.*` 为回退面，被实例 key 逐字段覆盖。 | 实例取值设回默认（如 `level=brief`）无法覆盖非默认的顶层值——两个面只配一个。 |
| `driver` | string | DefaultDriver | 从注册表（`RegisterDriver`）选择 driver。未知名字 → 启动报 "s3 driver not found"。 | 自定义 driver 跳过 dynamicTransport 握手 → 该 client 无 resilience（observe 仍经 wrapper 字段生效）。 |

---

## 4. 验证与故障演练

### 4.1 对象往返演练（与 example/check.sh 同路径）

```bash
cd example && docker compose up -d && go run .   # 自断言 put→read→stat→remove，打印 marker
curl -s :9370/readyz | grep -o '"s3:a[^"]*":[^,}]*'   # 指示器 UP（需 actuator）
```

example 在 GetObject 后自断言 `bytes.Equal(got, content)`——任何传输层损坏都会失败。

### 4.2 fail-fast 演练

把 `spring.s3.a.secret-access-key=wrong`：启动中止，报
`failed to reach s3 endpoint ...`（底层是签名不匹配）。探测（ListBuckets）的存在
就是让凭据/端点错误到不了首次使用。

### 4.3 健康演练

应用运行中停掉 MinIO（`docker compose stop minio`）：`curl :9370/readyz` 里 `s3:a`
组件翻 DOWN（探测 = ListBuckets）。重启后恢复——指示器按次探测，不复位锁定。

### 4.4 可观测演练

`spring.s3.a.observability.level=detailed` 且引入 starter-otel 后，做一次上传并读三个信号：
名为 `PUT /go-spring-example/hello.txt` 的 span、时长直方图、逐请求访问日志行。
加上 `spring.s3.a.observability.skipOps=PUT /go-spring-example/hello.txt` 再试：该操作
从日志消失而其他操作保留——证明实例前缀解析路径（顶层 `observability.*` 作为回退面）。

### 4.5 fault/resilience 演练（需 starter-governance）

按端点资源标签 `s3:127.0.0.1:9000` 配置治理规则：对该资源的 `fault.rate` 让一部分
上传经 executor 失败——可通过 `resilobserve.WrapExecutor` 的 outcome span/计数观测。
改回规则文件即撤火（经治理 source 热切换）。⚠ 注意 retry 按 round-trip 重试而非按流：
大 body 上传可能重发 body。

---

## 5. 排障表

| 症状 | 可能原因 | 处置 |
|------|----------|------|
| 启动中止 "failed to reach s3 endpoint" | 端点宕机、端口错、`use-ssl` 不匹配、凭据错误 | 探测错误带底层原因（签名不匹配 ⇒ 凭据；connection refused ⇒ 端点/ssl）。 |
| 启动中止 "s3 driver not found: X" | `driver` 指向未注册名字 | 在 init() 里 `RegisterDriver` 后再用，或删掉该 key。 |
| 启动中止 "unknown bucket-lookup" | 风格字符串非法 | auto / virtual-host / dns / path 之一。 |
| 顶层 `observability.*` 不生效 | 实例前缀 `spring.s3.<name>.observability.*` 配了非默认值，逐字段覆盖了它 | 只保留两个面之一，或把实例 key 配成想要的值。 |
| 自定义 driver 的 client 无 resilience | dynamicTransport 握手仅 DefaultDriver 有 | 接受 observe-only，或在 driver 里自装间接层。 |
| 对 MinIO 正常、某云上 404/重定向 | 该云不支持 virtual-host 寻址 | `bucket-lookup=path`。 |
| 应用正常但 readyz DOWN | 启动后凭据轮换失效 | 指示器是活探测；刷新凭据 / 重启。 |
| 两实例容器报健康 bean 重复键 | （历史）指示器未加 `.Name` 注册 | 现行代码注册 `s3:<name>`——自建 starter 时沿用该模式。 |

## 6. 设计体检表

| 指标 | 数值 |
|------|------|
| 配置 key 总数 | 9（另共享 3 个顶层 observability.*） |
| 其中必填 | 3（endpoint、access-key-id、secret-access-key） |
| quickstart 前置外部依赖 | 1（MinIO / 任意 S3 端点） |
| "注意/坑" 条数 | 5 |

设计嫌疑清单（前两条沿用上一版）：
- `driver` key 存在但仓内只随 DefaultDriver 一个实现——预判性扩展点；driver 与
  wrapper 之间的 dynamicTransport 握手是隐式的（挂在 sync.Map 上）。
- `bucket-lookup` 对同一模式接受 "virtual-host" 与 "dns" 两个别名——配置面轻度冗余。
- 已修（原"实例级 observability key 失效"）：`Init` 现经 `resolveObservability`
  解析策略——实例前缀 `spring.s3.<name>.observability.*`（绑入
  `Config.Observability`）逐字段覆盖顶层 wrapper 字段。残留限制：绑定在未配置时
  也填默认值，与默认值相同的实例取值无法覆盖非默认顶层值。
- 新增：资源标签只有 `s3:<endpoint>`——同端点两实例（如 example 的 `a`/`b`）共享
  一个 resilience 作用域，无按实例区分。
- 新增：健康探测与 fail-fast 探测同为 ListBuckets 但代码重复
  （starter.go HealthCheck vs health/health.go）——无害，可小幅合并。
