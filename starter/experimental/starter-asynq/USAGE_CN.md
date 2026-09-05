# starter-asynq 使用说明 — 参考手册

详细使用参考。总览见 [README_CN.md](README_CN.md)。所有行为声明均已对照 starter 源码
（`starter.go`、`client.go`、`driver.go`、`config.go`、`health/health.go`）与可运行的
[example/](example/)（docker 门控冒烟：`example/check.sh`）核实。**asynq 自身语义——
任务类型、重试、调度、队列、Inspector——见 [asynq 官方文档](https://github.com/hibiken/asynq)**；
本文只写 go-spring 的增量（装配、治理、观测、健康检查）。

**激活条件**：任一 `spring.asynq.<name>` 子树（`gs.OnProperty("spring.asynq")` 为前缀匹配，
starter.go:36）。一个实例恒产出生产者 `*Client`；worker `*Server` 仅当
`<name>.server.enabled=true` 时存在（默认关——常驻 worker 是选配）。多实例：每个
`spring.asynq.<name>` 条目是独立的 Redis 任务队列。

---

## 1. 完整工程示例

单实例双角色：生产者投递 `example:greet` 任务，worker（配置中开启）执行，断言往返成功后
进程自退出——与 [example/example.go](example/example.go) 同构。文件树：

```
demo/
├── go.mod
├── main.go
├── conf/
│   └── app.properties
└── docker-compose.yml          # redis:7
```

**go.mod**（关键依赖——见 `example/go.mod`）：

```
module demo

require (
    github.com/hibiken/asynq    latest
    go-spring.org/spring        v1.3.x
    go-spring.org/starter-asynq latest
    go-spring.org/starter-actuator latest   // 可选：§4 的健康端点
    go-spring.org/starter-governance latest // 可选：投递上的 resilience/fault
)
```

**main.go**：

```go
package main

import (
    "context"
    "encoding/json"
    "fmt"
    "time"

    "github.com/hibiken/asynq"
    "go-spring.org/log"
    "go-spring.org/spring/gs"

    starter "go-spring.org/starter-asynq"
    _ "go-spring.org/starter-actuator"
    _ "go-spring.org/starter-governance"
)

const taskType = "example:greet"

// Service 持有生产者与 worker bean。autowire tag 是实例名与派生的 server bean
// 名（"<name>:server"）。
type Service struct {
    Client *starter.Client `autowire:"a"`
    Server *starter.Server `autowire:"a:server"`
}

// completed 把 handler 的 payload 带回冒烟断言。
var completed = make(chan string, 1)

// Init 在 gs 字段注入两个 bean 之后运行——这是 worker 开始消费前注册 handler 的
// 正确时机（gs 的 Rooter Init 阶段先于 Runner/server，见 example/example.go:49-55）。
func (s *Service) Init() error {
    s.Server.RegisterHandler(taskType, handleGreet)
    return nil
}

func handleGreet(ctx context.Context, task *asynq.Task) error {
    var payload map[string]string
    if err := json.Unmarshal(task.Payload(), &payload); err != nil {
        return err // 返回 error 即进入 asynq 的重试域
    }
    completed <- payload["msg"]
    return nil
}

func main() {
    svrBean := gs.Provide(&Service{}).Export(gs.As[gs.Rooter]()).Init((*Service).Init)

    go func() {
        time.Sleep(700 * time.Millisecond)
        ctx := context.Background()
        payload, _ := json.Marshal(map[string]string{"msg": "hello asynq"})
        // 用 wrapper 的 Enqueue——带守护与观测的路径（§2.3）。
        if _, err := svrBean.Interface().(*Service).Client.Enqueue(ctx, asynq.NewTask(taskType, payload), asynq.Queue("default")); err != nil {
            log.Errorf(ctx, log.TagAppDef, "ENQUEUE failed: %v", err)
        }
        fmt.Println("Asynq round trip OK:", <-completed)
    }()
    gs.Run()
}
```

**conf/app.properties**——上面用到的完整注释配置面：

```properties
# --- asynq 实例 "a"（生产者与 worker 共用这些 Redis 配置）---------------------
spring.asynq.a.addr=127.0.0.1:6379
spring.asynq.a.db=0
spring.asynq.a.concurrency=4            # worker：并发处理任务上限
# spring.asynq.a.queues=critical:5,default:1   # 队列名 -> 优先级权重
# 开启该实例的 worker 角色（默认关）。
spring.asynq.a.server.enabled=true
# spring.asynq.a.shutdown-timeout=8s    # worker 退出排空上限

# --- 到 Redis 的 TLS（共享 tlsconf 块；此处关闭）------------------------------
# spring.asynq.a.tls.enabled=true
# spring.asynq.a.tls.cert-file=...      # 另有 key-file / ca-file / server-name /
#                                       #   insecure-skip-verify

# --- actuator（§4 的健康端点）---------------------------------------------------
spring.actuator.addr=:9370
```

**docker-compose.yml**（复制自 `example/docker-compose.yml`）：

```yaml
services:
  redis:
    image: redis:7
    ports:
      - "127.0.0.1:6379:6379"
```

**验证**：

```bash
docker compose up -d
go run .                          # 期望输出："Asynq round trip OK: hello asynq"
./example/check.sh                # 完整冒烟：起 compose、运行、断言、拆除
curl -s :9370/healthz | grep asynq   # Redis 应答后 "asynq:a": UP
```

---

## 2. 装配与时序

### 2.1 bean 生命周期时间线

```
import starter-asynq
  └─ init: gs.Module(gs.OnProperty("spring.asynq"))               [starter.go:36]
        └─ conf.BindEach("${spring.asynq}", 逐 <name>):
             ├─ Config 的 expr 校验（addr != ''——绑定期即失败）
             ├─ Provide(newClient).Name(<name>).Init/Destroy       [恒有]
             ├─ if c.Server.Enabled:
             │    Provide(newServer).Name(<name> ":server").
             │      Export(gs.As[gs.Server]()).Init/Destroy        [worker 选配]
             └─ Provide(health.Indicator).Name("asynq:"+<name>).
                Export(gs.As[health.Indicator]())
gs.Run()
  ├─ bean 装配：应用 bean 按实例名 autowire *Client / *Server
  ├─ Rooter Init 阶段：应用 Service.Init 注册 handler（mux 惰性创建，
  │  client.go:129-147——之后注册也可以，但 worker 开始消费后 handler 集合固定）
  ├─ Client.Init (client.go)：newObserver()（observe.go）、
  │    resilience.ResourceLabel("asynq", addr)、fault executor 就绪
  ├─ Server.Init (client.go:111-127)：asynq.NewServer(connOpt, Config{...})
  ├─ Runner 阶段：Server.Run——srv.Start(mux)、sig.TriggerAndWait() → 就绪，
  │    随后阻塞于 <-ctx.Done()
  └─ SIGTERM 时：Server.Stop → srv.Shutdown() 在 shutdown-timeout 内排空在途
       任务；Destroy 也会调 Stop；Client.Destroy 关闭生产者与 resilience executor
```

### 2.2 源码注释中的设计理由

- **worker 选配**（`server.enabled` 默认 false）："a long-running worker is an opt-in...
  most processes only enqueue"（starter.go:33-35；starter/DESIGN.md 约定）。
- **`Run` 用 `Start` + 等 ctx，绝不用 `asynq.Server.Run`**：asynq 的 helper 自装信号
  处理器，"would race gs's graceful-shutdown signal handling"（client.go:154-158）。
  信号处理权留在 gs。
- **handler 错误/panic 归 asynq**：server 刻意不装 ErrorHandler/recover 包装——
  "errors and panics inside a handler are asynq's to recover and retry... we keep our
  own reporting out of the hot path"（client.go:121-125）。
- **健康检查每次用全新 Inspector 探测**，"verifies reachability without coupling to
  the producer/worker lifecycle"（health/health.go:26-32）。

### 2.3 一次任务逐层走读（投递 → 执行）

1. 应用调用 **wrapper 的** `Client.Enqueue(ctx, task, opts...)`（client.go:71-94）。不要
   调用内嵌提升的 `*asynq.Client.Enqueue`——只有 wrapper 走守护链。
2. 观测层开启生产者观测（`o.obs.start(ctx, "enqueue", task.Type())`，
   observe.go）：span、指标与访问日志。
3. executor 执行：`fault.WrapExecutor(resilience.ExecutorFor("asynq:<addr>"))`——带
   starter-governance 时，限流拒绝/熔断开启会在**接触 Redis 之前**中止；未引入则为直通。
4. `Client.EnqueueContext` 把任务写入 Redis（asynq 语义：队列/优先级由 opts 决定）。
5. worker 的 `ServeMux` 按任务类型匹配注册的 pattern（`:` 作中间件分组分隔符），在
   `concurrency` 个槽位之一上调你的 `HandlerFunc`。
6. handler 返回 nil → 完成，span 无错结束。返回 error → **asynq 的重试域**
   （默认 25 次指数退避——见 [asynq retries](https://github.com/hibiken/asynq#retries)）；
   starter 不加东西。
7. handler 内 panic 由 asynq 自带 guard recover，同样进入重试。

---

## 3. 逐 key 行为参考

实例前缀：`spring.asynq.<name>.*`（Config 经带前缀的 `conf.BindEach` 绑定——这些
**是**实例前缀 key）。

| Key | 类型 | 默认值 | 行为 / 联动 | 配错后果 |
|-----|------|--------|------------|----------|
| `spring.asynq.<n>.addr` | string | — | Redis `host:port`；同时构成治理资源标签 `asynq:<addr>`。必填（`expr:"$ != ''"`）。 | 缺失 → 绑定期启动报错。 |
| `..username` / `..password` | string | 空 | Redis ACL 认证。 | 配错 → 运行期投递/消费失败而非启动期（健康检查能探出）。 |
| `..db` | int | 0 | Redis 数据库编号。 | 生产者与 worker 的 db 不一致 → 任务投了却没人消费。 |
| `..tls.*` | 块 | 关 | 共享 tlsconf（`enabled`、`cert-file`、`key-file`、`ca-file`、`server-name`、`insecure-skip-verify`）；开启时 DefaultDriver 构建 TLS RedisClientOpt（driver.go:58-77）。 | TLS 配一半 → bean 构建期报错。 |
| `..concurrency` | int | 10 | worker：并发处理任务上限。对纯生产者实例无效。 | 过低 → 队列积压；`server.enabled=false` 时静默无效。 |
| `..queues` | map[string]int | 空 → asynq "default":1 | 队列 → 优先级权重（越高越常被处理）。⚠ 投递侧 `asynq.Queue(...)` 选项必须指向已配置的队列（或回退 default），否则 worker 取不到。 | 投到未列出的队列 → 任务永久 pending。 |
| `..shutdown-timeout` | duration | 8s | worker 排空上限（`srv.Shutdown()`）；传给 `StopContext` 的 ctx 不被使用——排空由该 timeout 兜底（client.go:176-183）。 | 过短 → 发布时在途任务被弃。 |
| `..server.enabled` | bool | false | **worker 选配开关**。同时决定 `*Server` bean 是否存在——不开却 `autowire:"a:server"` 会在装配期失败。 | 注入 worker 但没开 → 容器报 bean 不存在。 |
| `..driver` | string | `DefaultDriver` | 选择已注册的 Driver（名字须与 `RegisterDriver` 一致）。空 → `DefaultDriver`。 | 名字未知 → 启动报错 `asynq driver not found: <name>`（starter.go:66）。 |

---

## 4. 验证与故障演练

### 4.1 往返（任务执行）

```bash
docker compose up -d && go run .        # "Asynq round trip OK: hello asynq"
./example/check.sh                      # 脚本化：compose -p gs-asynq-example、marker 断言
```

### 4.2 健康检查

实例级 indicator `asynq:a` 对 `default` 队列做 Inspector 往返：

```bash
docker stop asynq-redis
curl -s :9370/healthz | grep asynq      # 翻 DOWN；重启后恢复
docker start asynq-redis
```

### 4.3 失败重试（asynq 域）

让 handler 首次返回错误（如首次投递失败一次）：

```go
if atomic.AddInt32(&tries, 1) == 1 { return fmt.Errorf("boom") }
```

任务按 asynq 重试策略（默认 25 次、指数退避）重跑——观察：

```bash
redis-cli -n 1 keys '*'                  # asynq 的重试状态存于 Redis
grep -c "boom" <log>                     # handler 错误经 asynq 日志浮出
```

### 4.4 退出排空

启动慢 handler（sleep 3s）、投一个任务、运行期间 `kill -TERM`：worker 在
`shutdown-timeout`（默认 8s）内排空后退出；timeout 短于任务时长则弃置（asynq 会在
下次投递时重试——asynq 语义）。

### 4.5 治理守护（可选）

带 starter-governance + `govern` source 时，对资源 `asynq:<addr>` 开熔断/限流：
`Client.Enqueue` 直接返回拒绝、**不触 Redis**；提升来的 `asynq.Client` 路径则完全绕过
守护（见 §5 第 2 行）。

---

## 5. 排障表

| 症状 | 可能原因 | 处置 |
|------|----------|------|
| 容器报 bean `a:server` 不存在 | 未开 `server.enabled`（worker 选配） | 设 `spring.asynq.<n>.server.enabled=true` 或去掉注入。 |
| 任务投了没人跑 | worker 未开启；投到 `queues` 之外的队列；生产者与 worker 不同 `db` | 对齐配置；用 `asynq.Queue("<已列出>")` 投递。 |
| "asynq driver not found: DefaultDriver" | 注册表被改动 / init 顺序异常 | 不要反注册；上报（默认查找，starter.go:63）。 |
| 守护/resilience 从不生效 | 调了提升的 `*asynq.Client.Enqueue/EnqueueContext` 而非 wrapper | 调 wrapper 的 `Enqueue`（client.go:71）。 |
| 能投递但健康 DOWN | `default` 队列从未创建 / ACL 限制 Inspector | 健康检查固定探 `default` 队列；确认 Redis 可达与权限。 |
| 发布丢任务 | `shutdown-timeout` 短于在途任务 | 调到高于最长任务时长。 |
| 注册了自定义 Driver 却不生效 | `..driver` key 没指向注册名 | 配 `spring.asynq.<n>.driver=<name>` 为 `RegisterDriver` 的名字；名字未知会启动报错。 |

## 6. 设计体检表

| 指标 | 数值 |
|------|------|
| 配置 key 总数 | 实例前缀 11 个（含 6 个 tls 子 key） |
| 其中必填 | 1（addr） |
| quickstart 前置外部依赖数 | 1（Redis） |
| 注意/坑条数 | 5 |

设计嫌疑清单（原"死掉的 driver 选择"一项已修复）：

- 生产者与 worker 共用一份 Config，真正共用的只有 addr/认证/tls/driver；
  concurrency/queues/shutdown-timeout 是 worker 专属 key 却放顶层。
- 提升的 `*asynq.Client` 方法（`EnqueueContext` 等）绕过守护/观测 seam——容易误调。
- 实例级观测 key 只有偏离绑定默认值（brief/512/无 skip）才算"已设置"——这是按差异
  合并的代价，与 starter-s3 相同。
