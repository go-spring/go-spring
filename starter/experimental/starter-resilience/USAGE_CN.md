# starter-resilience 使用说明 — 参考手册

详细使用参考。概览见 [README_CN.md](README_CN.md)。所有行为声明均已对照 starter 源码
（`starter.go`、`executor.go`、`breaker_listener.go`、`executor_test.go`）、抽象层
[cloud/governance/resilience](../../../cloud/governance/resilience)（`driver.go`、
`provider.go`）与自断言的 [example/](example/)（`example/check.sh` —— 无容器、无外部
依赖）。**resilience 语义（熔断窗口、重试退避、policy 词汇）见
[cloud/governance/resilience](../../../cloud/governance/resilience)；sentinel-golang 行为见
[官方文档](https://github.com/alibaba/sentinel-golang)** —— 下文只写驱动接线。

**激活方式**：仅 blank import。`init`（starter.go）调用 `sentinel.InitDefault()`
（失败即 panic —— 源码注释："让配置错误的环境在这里大声失败，而不是等到首次使用"），
随后 `resilience.RegisterDriver("sentinel", sentinelDriver{})`。无 bean、无端口、
**无自有配置 key** —— policy 配在选择该驱动的消费方那里。

---

## 1. 完整工程示例

在 sentinel 驱动上演练客户端 seam 的自包含冒烟：dialer seam（连接建立期的熔断）与
rate limit + 熔断 + 重试的组合策略。文件树（即 example 本身）：

```
demo/
├── go.mod
├── main.go
└── conf/app.properties   // 空：纯 API 冒烟，不需要 spring.* key
```

**go.mod**（关键依赖）：

```
require (
    go-spring.org/cloud              v0.0.0   // governance/resilience
    go-spring.org/starter-resilience latest
)
```

**main.go**（取自 example）：

```go
package main

import (
    "context"
    "errors"
    "net"
    "net/http"
    "sync/atomic"
    "time"

    "go-spring.org/cloud/governance/resilience"

    _ "go-spring.org/starter-resilience" // 注册 "sentinel" 驱动
)

func main() {
    driver, err := resilience.GetDriver("sentinel")
    if err != nil {
        panic(err)
    }
    demoClientDialer(driver)
    demoComposedRetry(driver)
}

// demoClientDialer：阈值 3 的熔断器在三次连接被拒后跳闸；第四次在
// 触碰网络之前就被中立错误 ErrCircuitOpen 短路。
exec, _ := driver.NewExecutor(resilience.Policy{ErrorThreshold: 3, OpenDuration: time.Minute})
ln, _ := net.Listen("tcp", "127.0.0.1:0"); deadAddr := ln.Addr().String(); _ = ln.Close()
base := resilience.DialFunc((&net.Dialer{Timeout: time.Second}).DialContext)
dial := resilience.NewDialer(base, exec, "dead-service")
for i := 1; i <= 3; i++ { _, err := dial(ctx, "tcp", deadAddr) /* 真实拨号错误 */ }
_, err := dial(ctx, "tcp", deadAddr)
errors.Is(err, resilience.ErrCircuitOpen) // true —— 熔断已开，未碰网络

// demoComposedRetry：不稳定上游先 503 两次再成功 —— 在重试预算内恢复，
// 限流与熔断全程透明。
exec2, _ := driver.NewExecutor(resilience.Policy{
    RateLimit: 100, ErrorThreshold: 10, MaxRetries: 3, Timeout: time.Second,
})
client := &http.Client{Transport: resilience.NewRoundTripper(http.DefaultTransport, exec2, nil)}
resp, _ := client.Get("http://127.0.0.1:PORT/flaky") // 200；服务端恰好命中 3 次
```

**验证**：

```bash
cd starter/experimental/starter-resilience/example && ./check.sh
# 期望："client Dialer: circuit opened after 3 refused dials"、
#       "composed policy: recovered after 3 attempts ..."、退出码 0
```

生产上更常见的是声明式路径：任何驱动感知的消费方按名选择 sentinel，例如
`spring.http.client.<name>.resilience.driver=sentinel`（starter-http-client /
oauth2-client）。不设 `driver` key 时这些 starter 停留在零依赖的 `default` 驱动。
走 governance 中心时客户端根本不选驱动 —— 它们调用
`resilience.ExecutorFor(system, label)`（见 §2.2）。

---

## 2. 装配与时序

### 2.1 驱动接线

```
import starter-resilience
  └─ init(): sentinel.InitDefault()          // 失败即 panic
             resilience.RegisterDriver("sentinel", sentinelDriver{})
```

- `sentinelDriver.NewExecutor(p)`（starter.go）→ `newSentinelExecutor(p)`
  （executor.go）：前置拒绝负的 `RateLimit`；否则构造带空 loaded 集合的 executor。
- sentinel 一切按 resource name 索引，因此**规则惰性加载**：某资源首次被使用时
  `ensureRules(resource)`（executor.go）在互斥锁下最多安装三类规则 —— flow
  （RateLimit>0）、circuit-breaker（BreakerActive()）、isolation（MaxConcurrent>0，
  挂在带后缀的名字下，见 §2.3）—— 然后标记已加载。

### 2.2 ExecutorFor seam（governance 中心集成）

`resilience.ExecutorFor(system, resource)`（抽象层 provider.go，不在本 starter）是客户端
替代"注入 governance 中心"的唯一调用：

- 返回持有系统名与 label 的稳定 `resolvedExecutor`；真正的 executor 在**每次 Execute 时
  惰性解析**并按 label 记忆化（sync.Map 缓存）。observe 层就在这次解析中、在 executor
  尚未发布时应用，所以客户端拿到的已是组装好的 executor，不再自己包一层。
- provider 由 starter-govern 在建好 governance 中心后一次性安装；因为解析推迟到
  调用时，客户端与 govern 的装配先后无关紧要。
- 未注册 provider（没有 starter-govern 或 governance 关闭）时，executor 是透明的
  **no-op**：fn 原样跑一次 —— 无论是否配置 resilience，客户端代码形态一致。
- 热切换挂在 backing executor 上：provider 注册 governance 订阅并原地刷新它；
  本 starter 的 `Refresh`（下述）才是把新 policy 真正应用到 sentinel 的那一步。

### 2.3 一次 Execute 的逐层走读

`exec.Execute(ctx, "my-resource", fn)`（executor.go）：

1. `ensureRules("my-resource")` —— 加载/校验 flow + breaker + isolation 规则。
2. **先 bulkhead**：若 `MaxConcurrent>0`，在 `"my-resource$bulkhead"`（`isoSuffix`）
   名下 Entry 一次，经 defer 在**整个 Execute（含重试）**期间持有并发槽。加后缀的
   原因：sentinel 会对一个资源名下的每类已注册规则做判定 —— 并发槽与每次尝试的
   entry 必须挂在不同资源名下才能独立获取（源码注释）。
3. **MaxDuration 预算**：若设置，用 `context.WithTimeout` 包住后续全部。
4. 每次尝试（最多 `MaxRetries+1` 次）：
   - `sentinel.Entry(resource, Outbound)` —— 驱动 flow + circuit-breaking。block
     错误立即经 `mapBlockError`（§2.5）返回；block 不重试。
   - `runOnce` 对 `fn` 施加单次尝试的 `Timeout`（若 >0）。
   - 出错时：`sentinel.TraceError(entry, err)` 喂给熔断统计，随后循环检查预算
     （`budgetCtx.Err()`）、重试谓词 `ShouldRetry`、是否最后一次尝试，并睡眠
     `Backoff(i)`（预算中途耗尽则提前中止）。
5. 首次成功即返回 nil；否则返回最后一个错误。

### 2.4 Refresh（policy 热切换）

`sentinelExecutor.Refresh(p)`（executor.go）：校验 `RateLimit>=0`，在互斥锁下换
policy 并**清空 loaded 集合**。sentinel 的 `LoadRulesOfResource` 会替换资源既有规则，
因此下次 Execute 按新阈值整体重载 —— 且熔断统计窗口被重置。与 default 驱动
"丢弃状态、下次调用重建"的惰性语义对齐（源码注释）。route listener 在 `ensureRules`
重跑时重新注册。

### 2.5 熔断事件与结果映射（breaker_listener.go）

- **状态事件**：sentinel 的 `StateChangeListener` 是进程级单例，因此单个
  `routeListener` 按 `rule.Resource` 经 `breakerRoutes` sync.Map 分发到各 executor
  的 `resilience.BreakerEventListener`。挂接方式：`SetBreakerEventListener(l)`
  （实现 `resilience.BreakerEventListenerSetter`；observe-resilience 的 WrapExecutor
  即用它）；listener 向 sentinel 注册一次（`ensureRouteListener`，sync.Once），并
  在 `ensureRules` 里按资源注册路由。状态 1:1 映射：sentinel Open/HalfOpen/Closed →
  `resilience.BreakerOpen/BreakerHalfOpen/BreakerClosed`。无路由的资源（go-spring
  之外加载的熔断规则）被静默忽略。
- **block 结果**：`mapBlockError` 把 sentinel 的 block 原因翻译成中立哨兵错误 ——
  `BlockTypeCircuitBreaking` → `ErrCircuitOpen`、`BlockTypeIsolation` →
  `ErrBulkheadFull`、其余（即 flow 拒绝）→ `ErrRateLimited` —— 调用方只 import
  resilience 包。

### 2.6 Policy → sentinel 规则翻译表（核对自 `ensureRules`/`loadBreakerRule`）

| Policy 字段 | sentinel 规则 | 零值兜底 |
|---|---|---|
| `RateLimit` | flow.Rule Direct/Reject，`StatIntervalInMs=1000`，`Threshold=RateLimit` | `<=0` 不安装 |
| `BreakerStrategy=ErrorRate` | circuitbreaker.ErrorRatio；`Threshold=ErrorRateThreshold`；`MinRequestAmount=MinRequests` | MinRequests → 1 |
| `BreakerStrategy=Consecutive` | circuitbreaker.ErrorCount；`Threshold=float64(ErrorThreshold)`；`MinRequestAmount=1` | — |
| `OpenDuration` | `RetryTimeoutMs` | 5000ms |
| `BreakerWindow` | `StatIntervalMs` | 1000ms |
| （两种策略） | `ProbeNum=1` —— 恰好一次试探的半开，对齐 builtin 的单许可闸门 | — |
| `MaxConcurrent` | isolation.Rule Concurrency，挂 `resource$bulkhead` | `<=0` 不安装 |

重试与单次超时不是 sentinel 概念 —— 由本 executor 包在 entry 检查外层
（`Execute`/`runOnce`）。

---

## 3. 逐 key 行为参考

**本模块自有前缀下没有 key。** `grep -rhoE 'value:"[^"]+"' starter-resilience` 无命中。
`Policy` 各字段（`rate-limit`、`error-threshold`、`open-duration`、`max-concurrent`、
`max-retries`、`timeout` 等）由消费方 starter（如
`spring.http.client.<name>.resilience.*`）与
[cloud/governance/resilience](../../../cloud/governance/resilience) 文档化；走
governance 中心时它们位于集中式 source（`govern.resilience.*`），经 `ExecutorFor` +
`Refresh` 到达本驱动。

---

## 4. 验证与故障演练

### 4.1 驱动注册

```bash
cd starter/experimental/starter-resilience && go test ./...
# executor_test.go 固化了 §2.6 的翻译表与 block 错误映射
```

或：`resilience.GetDriver("sentinel")` 必须无错返回非 nil；import 时的 Info 日志
"registered sentinel resilience driver"（tag：app 默认）确认 init 已执行。

### 4.2 熔断演练（来自 example）

`Policy{ErrorThreshold: 3, OpenDuration: time.Minute}` 对一个死地址：第 1–3 次拨号
报真实错误；第 4 次不碰网络即返回 `errors.Is(err, resilience.ErrCircuitOpen)`。另一
面：等 `RetryTimeoutMs`（一分钟）过后，下一个 Entry 是单次半开试探（`ProbeNum=1`）。

### 4.3 组合策略演练（来自 example）

`Policy{RateLimit: 100, ErrorThreshold: 10, MaxRetries: 3, Timeout: time.Second}` 对
先 503 两次的上游：客户端看到 200；`hits == 3` 证明完成恢复的是重试预算而非熔断，
宽松限流全程透明。

### 4.4 限流演练

`Policy{RateLimit: 1}` 加快速循环 Execute：超过约 1 次/秒的调用以 `ErrRateLimited`
失败（`BlockTypeFlow` → `mapBlockError` 的 default 分支）。

### 4.5 Refresh 演练（热切换）

持有 `RateLimit: 1` 构建的 executor，按 §4.4 观察到限流后调用
`exec.Refresh(resilience.Policy{RateLimit: 1000})`（或在 starter-govern 下经 governance
source 推送变更）：下次 Execute 重载规则、限流消失。注意 refresh 会重置熔断统计窗口
（§2.4）。

### 4.6 熔断事件观测

实现 `resilience.BreakerEventListener`，在该资源首次 Execute 前经
`SetBreakerEventListener` 挂接：上述演练中会依次触发 Closed→Open→HalfOpen→Closed
状态迁移 —— 即 observe-resilience WrapExecutor 消费的 seam。

---

## 5. 排障表

| 症状 | 可能原因 | 处置 |
|------|----------|------|
| import 时进程 panic："sentinel init failed" | `sentinel.InitDefault()` 所需环境损坏（配置/日志目录） | 修复环境 —— panic 是刻意的 import 期大声失败（嫌疑 #1）。 |
| `GetDriver("sentinel")` 报错 | 未 blank-import starter | 加 `import _ "go-spring.org/starter-resilience"`。 |
| 消费方仍走内置 resilience | 消费方的 `...resilience.driver` 未设置 | 按消费方逐个设 `=sentinel` —— 没有全局开关（嫌疑 #3）。 |
| 熔断器从不打开 | 未达 `MinRequestAmount`，或 `ErrorThreshold`/窗口配比不当 | 对照 §2.6 默认值表（MinRequests → 1、窗口 → 1000ms）。 |
| 配置推送后熔断状态像是被重置 | `Refresh` 清空规则；sentinel 替换规则并重置统计窗口 | 刻意的惰性重载语义（§2.4）。 |
| sentinel 控制台/指标里资源数翻倍 | bulkhead 挂在 `resource$bulkhead` 名下 | 驱动内部命名（嫌疑 #2）—— 按后缀过滤。 |
| 设了 `MaxRetries` 却不重试 | `ShouldRetry(err)` 为 false，或 `MaxDuration` 预算在下次尝试前耗尽 | 检查 policy 的重试谓词与预算。 |
| 导入了 sentinel 但一切像 no-op | 走 `ExecutorFor` 且无 provider（无 starter-govern / governance 关闭） | 预期的零成本兜底 —— 配置 governance 或直接用驱动。 |

---

## 6. 设计体检表

| 指标 | 数值 |
|------|------|
| 配置 key 总数 | 0（归消费方） |
| 其中必填 | 0 |
| quickstart 前置外部依赖 | 0 |
| "注意/坑" 条数 | 3 |

设计嫌疑清单（沿用上版，交设计裁决）：

1. `sentinel.InitDefault()` + import 期 panic 使失败形态成为 import 顺序崩溃而非
   正常启动错误。
2. bulkhead 挂在 `resource$bulkhead` 后缀名下 —— sentinel 控制台/指标显示双倍资源；
   驱动内部细节泄漏到可观测层。
3. 驱动选择是逐消费方配置（`...resilience.driver=sentinel`）而无全局开关；要让
   sentinel 全面生效需在每个 client 上重复该 key。
