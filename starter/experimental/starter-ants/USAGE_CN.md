# starter-ants 使用说明 — 参考手册

详细使用参考。概览见 [README_CN.md](README_CN.md)。所有行为声明均已对照 starter 源码
（`starter.go`、`config.go`）与可运行的 [example/](example/)（`example/check.sh`，自断言、
无外部依赖）。**ants 自身语义（worker pool、purge、过载）见
[ants 文档](https://github.com/panjf2000/ants)** —— 下文只写 go-spring 的增量：装配、
observer、panic 策略、指标。

**激活方式**：任意 `spring.ants.<name>` 子树。starter 通过 `gs.Module(gs.OnProperty("spring.ants"))`
+ `conf.BindEach` 注册，每个 map key 生成一个具名 `Pool` bean。不配置就没有 bean；
没有 `enabled` 开关。

---

## 1. 完整工程示例

一个含两个隔离 pool（小容量非阻塞 IO pool、较大阻塞 CPU pool）的服务，panic 走共享上报链，
并暴露指标快照。文件树：

```
demo/
├── go.mod
├── main.go
├── worker.go
└── conf/
    └── app.properties
```

**go.mod**（关键依赖）：

```
require (
    go-spring.org/spring       v1.3.x
    go-spring.org/starter-ants latest
    go-spring.org/starter-actuator latest   // 可选：HTTP 暴露统计
)
```

**main.go**：

```go
package main

import (
    _ "demo/worker"

    "go-spring.org/spring/gs"
    _ "go-spring.org/starter-actuator"
    _ "go-spring.org/starter-ants"
)

func main() { gs.Run() }
```

**worker.go** —— 应用侧的完整 pool 面：

```go
package worker

import (
    "context"
    "sync/atomic"
    "time"

    "go-spring.org/log"
    "go-spring.org/spring/gs"
    StarterAnts "go-spring.org/starter-ants"
)

// panics 统计被全局 handler 捕获的任务 panic（见 §2.4）。
var panics int64

func init() {
    // 必须在容器启动前注册 —— handler 在每个 pool 创建时读取
    // （SetPanicHandler 文档注释）。不设置自定义 handler 时，
    // panic 落到 goutil.ReportPanic（共享链）。
    StarterAnts.SetPanicHandler(func(p any) {
        atomic.AddInt64(&panics, 1)
        log.Warnf(context.Background(), log.TagAppDef, "recovered task panic: %v", p)
    })
}

// Service 按名注入 pool（即 spring.ants 下的 map key）。
type Service struct {
    IO      StarterAnts.Pool             `autowire:"io"`
    CPU     StarterAnts.Pool             `autowire:"cpu"`
    Metrics *StarterAnts.MetricsObserver `autowire:""`
}

func init() {
    // gs 只装配根可达 bean：无注入且无 Export 的 Provide bean 在 prod
    // 不会实例化。Export 成 gs.Rooter（或被别处注入）让容器物化它。
    gs.Provide(func() *Service {
        s := &Service{}
        // 演示负载：CPU pool 上跑 100 个计数任务
        go func() {
            time.Sleep(500 * time.Millisecond)
            var n int64
            for i := 0; i < 100; i++ {
                _ = s.CPU.Submit(func() { atomic.AddInt64(&n, 1) })
            }
        }()
        return s
    }).Export(gs.As[gs.Rooter]())
}
```

**conf/app.properties** —— 完整注释配置（取自 example）：

```properties
# --- io pool：小容量，满载时快速失败而非排队 ------------------------------
spring.ants.io.size=2
spring.ants.io.nonblocking=true

# --- cpu pool：有界阻塞 pool，空闲 worker 回收 ----------------------------
spring.ants.cpu.size=8
spring.ants.cpu.expiry-duration=10s

# --- 可选 knob（默认值见 §3）----------------------------------------------
# spring.ants.cpu.max-blocking-tasks=0
# spring.ants.cpu.pre-alloc=false
# spring.ants.cpu.disable-purge=false
# spring.ants.cpu.driver=DefaultDriver
```

**验证**（与 `example/check.sh` 同构）：

```bash
cd demo && go run .                       # 观察日志 "ants pool \"cpu\" initialized"
grep -c "recovered task panic" <log>      # 提交 panic 任务之后
```

example 的程序化断言：100/100 任务执行、`IO.Cap()==2`、`CPU.Cap()==8`、满载非阻塞
pool 的第三次 Submit 返回 error、panic handler 触发。

---

## 2. 装配与时序

### 2.1 bean 生命周期

```
import starter-ants
  ├─ init(): RegisterDriver("DefaultDriver", DefaultDriver{})     (config.go)
  └─ init(): gs.Provide(newMetricsObserver).Export(gs.As[PoolObserver]())
        │   在 newMetricsObserver 内部经 RegisterObserver 自注册
gs.Run()
  ├─ gs.Module(gs.OnProperty("spring.ants")) 触发（前缀匹配：任意 spring.ants.* key）
  ├─ conf.BindEach("${spring.ants}") → 每个 map key 一份 Config
  ├─ 每个 name：gs.Provide(ctor).Name(name).Destroy(destroyPool)
  ├─ bean init：createPool 解析 Driver → DefaultDriver.CreatePool
  │     （ants.NewPool 挂 WithPanicHandler(poolPanicHandler)）
  │     再包 observedPool，使每次 Submit 都流经 observer 链
  ├─ 就绪：Init/Run 钩子里即可使用 pool
  └─ 停机：destroyPool → pool.Release()（停掉 purge goroutine）
```

设计说明（源码注释）：starter 用 `gs.Module` 而非 `gs.Group`，是为了把 pool 的 bean
name 传给 observer —— 这正是 `OnSubmit(name, task)` 能拿到 name 的原因。

### 2.2 一次 Submit 的逐层走读

`s.CPU.Submit(task)`：

1. `observedPool.Submit`（starter.go）先包任务：`wrapTask(name, task)` 把所有已注册
   `PoolObserver` 的 `OnSubmit` 依次折叠包裹，最内层是你的原任务。
2. 内置 `MetricsObserver.OnSubmit`（starter.go）再包一层 running 计数的
   加一/减一 —— `running` 按 pool name 维度统计。
3. `antsPool.Submit` 把包好的任务交给 ants 排队等 worker。
4. worker goroutine 上：panic → `poolPanicHandler`（见 §2.4）；正常返回 →
   defer 中递减计数。

### 2.3 observer —— 有哪些、怎么加

| 钩子 | 签名 | 注册方式 |
|------|------|----------|
| `PoolObserver.OnSubmit` | `OnSubmit(name string, task func()) func()` | 把 bean Export 成 `PoolObserver`，或启动前调用 `RegisterObserver(o)` |

链在 Submit 时刻应用而非建池时刻：`wrapTask` 每次 Submit 读取 `Observers()` 快照，
因此池已存在后再注册的 observer 对后续提交仍然生效。内置 `*MetricsObserver` bean
在 `newMetricsObserver` 里自注册。源码注释列出的典型用途：metrics、tracing 上下文
注入、耗时日志、按 pool 限流。

### 2.4 panic 链（统一 panic 策略）

`DefaultDriver.CreatePool` 挂上 `ants.WithPanicHandler(poolPanicHandler)`。链路
（config.go）：

- 设置了 `SetPanicHandler(fn)` → 执行你的 `fn`（传 nil 清除，恢复共享上报）。
- 否则 → `goutil.ReportPanic(ctx, p)` —— 与 goroutine/handler/job panic 同一条
  goutil 链（`go-spring.org/log` 安装的 structured-log 桥），pool panic 汇入同一
  上报流。ants 只交出 panic value，所以 ReportPanic 在 worker 的 deferred recover
  里被调用 —— panic 中的栈帧还在。
- hook 在建池时读取；启动后才 `SetPanicHandler` 只影响之后创建的池。按 pool 定制
  handler 需要自定义 `Driver`。

### 2.5 停机

`destroyPool` 调 `pool.Release()`：worker 跑完当前任务、后台 purge goroutine 停止、
内存回收。Release 时仍在队列里的任务会被 ants 丢弃 —— 业务侧先在自己的 Stop 钩子里
排空，再返回。

---

## 3. 逐 key 行为参考

前缀 `spring.ants.<name>.*` —— 多实例，key 按实例前缀绑定（`conf.BindEach` 带
实例前缀；不是顶层绝对引用）。

| key | 类型 | 默认值 | 行为/联动 | 配错后果 |
|-----|------|--------|-----------|----------|
| `size` | int | 256 | 最大并发 worker 数；`<=0` = 无界（此时 `Cap()` 为 -1）。 | 太小且 `nonblocking=false` → 提交方阻塞；太大 → 失去背压。 |
| `expiry-duration` | duration | 1s | 空闲 worker 回收周期（ants 周期 purger）。 ⚠ `disable-purge=true` 时被忽略。 | 过小 → worker 抖动（分配/GC 压力）；过大 → 空闲 goroutine 滞留。 |
| `pre-alloc` | bool | false | 预分配 worker-queue 内存。 | 只是启动成本/延迟权衡，无失败模式。 |
| `max-blocking-tasks` | int | 0 | 等待空闲 worker 的被阻塞提交方上限；0 = 不限。 ⚠ `nonblocking=true` 时被忽略。 | 0 + 小池 + 阻塞模式 → 提交方无限堆积。 |
| `nonblocking` | bool | false | 池满时 Submit 立即返回 `ErrPoolOverload` 而非阻塞。 ⚠ 覆盖 `max-blocking-tasks`。 | 开启且不检查 Submit 错误 → 任务被静默丢弃。 |
| `disable-purge` | bool | false | worker 永不回收；无 purge goroutine。 ⚠ 使 `expiry-duration` 变死 key。 | 持续繁忙的池无碍；突发池会保持峰值 goroutine 数。 |
| `driver` | string | DefaultDriver | 从注册表（`RegisterDriver`）选 `Driver`。 ⚠ 只随包提供 `DefaultDriver` —— 其他值导致建池失败，即 bean init 失败，即启动失败。 | 拼写错误 → 启动即失败，报 "ants driver not found"。 |

已与 `grep -rhoE 'value:"[^"]+"'` 核对 —— 恰好这 7 个 key，无多余。

---

## 4. 验证与故障演练

### 4.1 任务经由 pool 执行（来自 example）

```bash
cd starter/experimental/starter-ants/example && ./check.sh
# 期望输出："Nonblocking pool correctly rejected submit"、
#           "Panic handler fired: 1 times"、pool metrics 块、退出码 0
```

可复用的程序化断言：提交 N 个任务 + WaitGroup，比对计数器与 N；比对 `Cap()` 与
配置容量以证明实例隔离。

### 4.2 过载行为演练

`io.size=2` + `io.nonblocking=true`：提交两个阻塞在 channel 上的任务后提交第三个 ——
`Submit` 返回 ants 的 `ErrPoolOverload`。`nonblocking=false` 时第三次提交改为阻塞
（若设置了 `max-blocking-tasks` 则受其约束）。

### 4.3 panic 演练

在任意 DefaultDriver pool 上提交 `func(){ panic("boom") }` 且未设自定义 handler：
进程存活，panic 经 goutil 上报链出现（structured log）。注册了 `SetPanicHandler`
（example 的 init）时计数器加一并打 Warn 行 —— 证明注入点生效。

### 4.4 指标演练

```go
stats := s.Metrics.Snapshot()               // Name + Running（Cap/Free/Waiting 为 0）
s.Metrics.Enrich(&stats, map[string]StarterAnts.Pool{"io": s.IO, "cpu": s.CPU})
```

`Enrich` 需要 pool 句柄，因为 observer 只知道 name 不知道 Pool —— 见嫌疑 #1。

---

## 5. 排障表

| 症状 | 可能原因 | 处置 |
|------|----------|------|
| 没有 pool bean；autowire `Pool` 失败 | 未配置任何 `spring.ants.*` key | 至少加 `spring.ants.<name>.size` —— 子树即激活开关。 |
| 启动失败 "ants driver not found" | `driver` 拼写错误或自定义 driver 未注册 | 修正名称或启动前 `RegisterDriver`。 |
| 启动失败：driver already registered | 两次 `RegisterDriver` 同名 | 注册重名会 panic —— 改名。 |
| 任务被静默丢弃 | `nonblocking=true` 且未检查 Submit 错误 | 检查 Submit 错误（返回 `ErrPoolOverload`）。 |
| 提交方卡住 | 阻塞池满载且 `max-blocking-tasks=0` | 调大 `size`、设置 `max-blocking-tasks` 或改非阻塞。 |
| panic handler 不触发 | `SetPanicHandler` 在池创建之后才调用 | hook 建池时读取 —— 放到 `init()`。 |
| 自定义 `PoolObserver` 不生效 | bean 未 Export 成 `PoolObserver`（或完全没注册；每次 Submit 读快照，晚注册本身没问题） | `Export(gs.As[PoolObserver]())` 或启动前 `RegisterObserver`。 |
| 空闲时 goroutine 数偏高 | `disable-purge=true` 或 `expiry-duration` 过大 | 重新启用 purge / 调低 `expiry-duration`。 |
| panic 被上报两次 | 设了自定义 handler 又期望共享链行为 | 链路二选一 —— 自定义 handler 会替代 goutil 上报。 |

---

## 6. 设计体检表

| 指标 | 数值 |
|------|------|
| 配置 key 总数 | 7 |
| 其中必填 | 0 |
| quickstart 前置外部依赖 | 0 |
| "注意/坑" 条数 | 6 |

设计嫌疑清单（沿用上版，交设计裁决）：

1. `Snapshot()` 返回半空统计、要调用方手工收集 pool 句柄再 `Enrich` —— observer
   知道 pool name 却不知道 Pool 句柄；API 不对称。
2. `driver` key 只随包提供 DefaultDriver —— 推测性扩展点（与 starter-s3 同型）。
3. panic handler 是被 `SetPanicHandler` 修改的包级全局 —— 对时序敏感。
4. （新增）`wrapTask` 每次 Submit 在 RLock 下快照 `Observers()` —— 热路径上的一把锁，
   仅为支持晚注册 observer。
