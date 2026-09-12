# starter-config-bus 使用说明（参考手册级）

详细使用文档，概览见 [README_CN.md](README_CN.md)。所有行为声明均对照 starter 源码
（`starter.go`、`bus.go`、`config.go`）与经 docker 冒烟验证的 [example/](example/)
（`example/check.sh`）核对。本 starter 是配置**事件总线**：只经 NATS 传播刷新*信号*，
从不携带配置内容——事实源始终是配置中心或本地文件。NATS 自身语义见
[nats.go 文档](https://docs.nats.io/)；以下全部是 go-spring 的增量。

**激活方式**：空白导入注册的 `ConfigBus` bean 带 `OnProperty("spring.config.bus")` 条件
（starter.go:49-58）。**未配置任何** `spring.config.bus.*` key 时 starter 什么都不装配，
导入是闲置的——共享模块里带上导入也不会强迫每个应用配置 NATS。一旦出现任意
`spring.config.bus.*` key（仅设 `spring.config.bus.subject` 也算），bean 装配，autowire tag
`${spring.config.bus.nats-instance:=config-bus}`（bus.go:55）要求 `spring.nats.instances.*` 下存在
名为 `config-bus`（或你指定的名字）的实例；不存在则容器装配失败、启动中止。没有单独的
`enabled` 开关。

> **迁移说明（行为变更）**：加条件门之前，零 NATS 实例下空白导入本包会在启动期以
> missing-bean 装配错误失败；现在同样的导入可以干净启动，bus 闲置直到出现
> `spring.config.bus.*`。若你曾把那个启动失败当作误配置告警，请改在自己的启动检查里
> 断言 `spring.config.bus.*` 存在。

---

## 1. 完整工程示例

一个"单实例发布一次、全舰队动态配置刷新"的服务。目录树（与冒烟验证的
[example/](example/) 同构）：

```
demo/
├── go.mod
├── main.go
├── conf/
│   └── app.properties
└── docker-compose.yml      # 本地 NATS broker
```

**go.mod**（关键依赖）：

```
require (
    go-spring.org/spring            v1.3.x
    go-spring.org/starter-config-bus latest
    go-spring.org/starter-nats       latest   // 传输层提供者——必需
)
```

**main.go**：

```go
package main

import (
    "context"
    "fmt"
    "os"
    "time"

    "go-spring.org/log"
    "go-spring.org/spring/gs"

    StarterConfigBus "go-spring.org/starter-config-bus"
    _ "go-spring.org/starter-nats"
)

// Demo 绑定一个动态字段并持有 bus，用于广播刷新。Export 成 gs.Rooter
// 让容器急切创建它——即使没有其他 bean 注入它（见 §2.1）。
type Demo struct {
    Bus     *StarterConfigBus.ConfigBus `autowire:"configBus"` // starter.go 里的 bean 名
    Message gs.Dync[string]             `value:"${demo.message:=none}"`
}

func main() {
    gs.Provide(&Demo{}).Export(gs.As[gs.Rooter]())

    // 模拟"发生了一次配置中心自身 watch 察觉不到的变更"：
    // 在 env source 抬一个新值，然后广播一次。
    go func() {
        time.Sleep(500 * time.Millisecond)
        want := "v-" + time.Now().Format("150405")
        _ = os.Setenv("GS_DEMO_MESSAGE", want) // env source → demo.message
        if err := getDemo().Bus.Publish(""); err != nil { // "" = 全舰队刷新
            log.Errorf(context.Background(), log.TagAppDef, "publish failed: %v", err)
            os.Exit(1)
        }
        deadline := time.Now().Add(15 * time.Second)
        for time.Now().Before(deadline) {
            if getDemo().Message.Value() == want {
                fmt.Println("config-bus refresh observed:", want)
                return // 字段在不重启的情况下翻转
            }
            time.Sleep(200 * time.Millisecond)
        }
        os.Exit(1)
    }()
    gs.Run()
}
```

（仓库内的 example 直接持有 `gs.Provide` 返回的 bean 句柄，流程与上面一致，见
[example/example.go](example/example.go)。）

**conf/app.properties**——完整带注释的配置面：

```properties
# --- NATS 传输层（starter-nats 中名为 "config-bus" 的实例）-------------------
# 名字必须与 spring.config.bus.nats-instance（默认 "config-bus"）一致。
spring.nats.instances.config-bus.url=nats://127.0.0.1:4222

# --- config bus ---------------------------------------------------------------
# Subject：共享同一 subject 的所有实例构成一个 bus。以下为默认值。
# spring.config.bus.subject=spring.config.refresh
# spring.config.bus.watch-prefixes=db,cache

# --- 动态值 -------------------------------------------------------------------
demo.message=v0
```

**docker-compose.yml**：

```yaml
services:
  nats:
    image: nats:2.10
    ports:
      - "127.0.0.1:4222:4222"
```

**验证**（与 `example/check.sh` 同构）：

```bash
docker compose up -d
go run .              # 预期：先 "subscribed to bus subject=..."，
                      # 再 "config-bus refresh observed: v-..."，干净退出
docker compose down -v
```

example 的手动模式（`go run . -manual`）保持进程存活，便于从第二个终端发布、或观察
第二个实例的刷新。

---

## 2. 装配与时序

### 2.1 bean 生命周期——bus 何时装配、为什么

```
import starter-config-bus
  └─ init(): gs.Provide(&ConfigBus{}).Name("configBus")
              .Init(subscribe).Destroy(Destroy).Export(gs.As[gs.Rooter]())   [starter.go:44-54]

gs.Run() → App.Start()                                                       [gs_app/app.go]
  ├─ 挂载 gs.RefreshProperties / gs.AppStarted 门面目标
  ├─ 属性加载（全部 source）+ 日志初始化
  ├─ 容器 refresh（bean 装配）：
  │    ├─ ConfigBus 字段注入：
  │    │    Conn      ← 按名注入 NATS 实例（${spring.config.bus.nats-instance:=config-bus}）
  │    │    Config    ← ${spring.config.bus} value tag（config.go）
  │    └─ bean Init 钩子：subscribe() —— 对 Config.Subject 做 NATS Subscribe  [bus.go:66-99]
  │         打日志 "subscribed to bus subject=... prefixes=[...]"
  ├─ app.started = true            ← 从此刻起 RefreshProperties 才被允许
  ├─ Runners → Servers → 就绪
  └─ SIGTERM：bean Destroy → sub.Unsubscribe()；NATS 连接由 starter-nats 负责关闭
```

对一个*总线*而言，两个时序事实很关键：

- **Rooter export 是"总是装配"的机制。** 该 bean 默认没有依赖方；
  `Export(gs.As[gs.Rooter]())` 使其 root 可达，容器因此在每个导入该包的进程里都创建
  它并执行 `subscribe()`——这正是舰队级总线需要的（不会出现"某实例忘了注入、静默错过
  刷新"）。starter.go:46-48 的注释明确写了这个意图。
- **Init 先于 `app.started = true` 执行。** 订阅在刷新被合法化*之前*就已生效：
  `RefreshProperties` 在该标志置位前会拒绝并返回 "app not started yet"
  （app.go:247-253，置位在 app.go:305）。落在该启动窗口内的信号会以
  `config bus: property refresh failed: app not started yet` 记日志后丢弃——不重试、
  不排队。窗口通常是毫秒级，但做竞态测试时要知道它存在。

### 2.2 启动契约：配置 key 门，以及非自 hosted key

- **有 OnProperty 门。** bean 在 `Condition(gs.OnProperty("spring.config.bus"))` 下注册，
  与仓内家族约定（配置 key 条件注册）一致。没有 `spring.config.bus.*` key → 没有 bean →
  导入闲置。key 存在时，剩余的隐式条件是 Conn 注入：没有
  `spring.nats.instances.config-bus.*` 定义 → 启动期容器装配失败。
- **bus 刷新出来的 key 门不了 bus 自己。** bus 自身的 key
  （`spring.config.bus.*`、`spring.nats.instances.<name>.*`）只在装配期读一次。刷新事件即使改变了
  `spring.config.bus.subject`，订阅**不会**迁移——bean 只在重启后重读。`watch-prefixes`
  同理：`subscribe()` 只解析一次（bus.go:67-71）。因此把所有 `spring.config.bus.*` key
  当作启动期常量；不要放进你期望热加载的命名空间。

### 2.3 Publish → 订阅 → gs.Dync 刷新，逐步走读

1. 某个配置 source 发生变化（本地 watch 漏掉的配置中心推送、挂载文件被编辑、env source
   抬值）。bus 不关心内容——它只被"告知"。
2. 应用代码（或管理端点）调用 `bus.Publish(prefix)`——签名
   `func (b *ConfigBus) Publish(prefix string) error`（bus.go:123）。它把
   `RefreshEvent{Prefix: prefix}` 序列化为 JSON 并 NATS 发布到 `Config.Subject`。
3. 订阅该 subject 的每个实例收到消息，执行 `subscribe()` 注册的回调（bus.go:73-92）：
   - 空 payload → 视为零值 `RefreshEvent`（全舰队刷新）；
   - JSON 畸形 → warn `ignoring malformed refresh event` 并丢弃（bus.go:76-80）；
   - `shouldRefresh(ev.Prefix)` 过滤（bus.go:106-116）：事件 prefix 为空、实例未配置
     watch、或事件 prefix 与某个 watched prefix **双向重叠**时放行——`db` watcher 对
     `db.pool` 事件有反应，反之亦然；
   - `gs.RefreshProperties()`——app 挂载的进程级刷新门面（无注入的
     refresher 字段）。
4. `App.RefreshProperties()`（app.go:247-255）：重新加载**全部**已配置 source
   （文件、env、命令行参数），按优先级重新合并，把新 storage 推进容器
   `c.RefreshProperties(p)`——后者原子地重解析所有 `gs.Dync[T]` 绑定
   （gs_core/injecting）。普通 `value:` 字段**不会**重新绑定。
5. 每个实例打一条成功日志：
   `config bus: refreshed properties on event (prefix="..." origin="...")`。
   刷新本身失败则打 `config bus: property refresh failed: ...` 且信号被消费——不重试，
   发布方也看不到订阅方的结果（NATS core 即发即忘）。

注意刷新是全应用粒度，不做 key 裁剪：`Prefix` 过滤的是*哪些实例反应*，不是*哪些 key
重解析*。`Publish("db")` 在放行它的实例上依然刷新该实例的全部 `gs.Dync` 字段。

---

## 3. 逐 key 行为参考

全部 key 位于顶层绝对前缀 `spring.config.bus`（经 config.go 的 value tag 绑定；
`nats-instance` tag 位于 bus.go 的 bean 结构体上）。无一必填——但看配错后果列。

| Key | 类型 | 默认值 | 行为 / 联动 | 配错后果 |
|-----|------|--------|-------------|----------|
| `spring.config.bus.subject` | string | `spring.config.refresh` | NATS subject，发布+订阅共用；共享它的所有实例构成一个 bus。装配期读一次——刷新事件改它不会触发重新订阅（§2.2）。 | 某实例写错 subject → 舰队静默裂成两半：刷新只在各自一半内传播，任何地方都不报错。 |
| `spring.config.bus.watch-prefixes` | string | ``（空） | 逗号分隔 prefix 过滤，`subscribe()` 解析一次。空 = 对每个事件都反应。非空 = 只对空 prefix 事件或双向 prefix 重叠放行（`db` ↔ `db.pool`）。⚠ 热加载死 key：仅重启后重解析。 | 列表过宽会刷得比预期多（无害但噪音）；某 prefix 与发布侧永不匹配时，该实例静默跳过 scoped 刷新，但 `Publish("")` 仍放行。 |
| `spring.config.bus.nats-instance` | string | `config-bus` | 作为传输层注入的 `spring.nats.instances.<name>.*` 连接名（按实例名 autowire，bus.go:55）。⚠ 指名的实例必须存在——这是事实上的激活门。 | 无对应 `spring.nats.instances.<name>.*` 块 → 启动期容器装配失败（bean 无法组装）。与业务 NATS 连接共用名字会把 bus 故障耦合到该连接。 |

除此之外，被引用的 NATS 实例还有自己的 `spring.nats.instances.<name>.*` key（url、认证等）——见
starter-nats 文档。NATS 连接是硬前置；没有嵌入式/内存兜底。

---

## 4. 验证与故障演练

所有演练用 §1 工程；先起 NATS（`docker compose up -d`）。

### 4.1 正常路径：发布 prefix 变更，观察 gs.Dync 更新

```bash
docker compose up -d
GS_DEMO_MESSAGE=manual-1 go run . -manual &   # example 的手动模式
# 在另一个终端触发 scoped 发布（在应用里加一个小 /refresh 处理器调
# d.Bus.Publish("demo")——或直接用自测模式）：
go run .                                       # 打印 "config-bus refresh observed: v-..."
```

可观测面：日志 tag `_app_config_bus`（经 `log.RegisterAppTag("config_bus", "")` 注册，
starter.go:41；用 `logger.config_bus.*` 调级别）。预期顺序：
`subscribed to bus subject=... prefixes=[...]`，随后每个事件一条
`config bus: refreshed properties on event (prefix="..." origin="...")`。

### 4.2 演练：无订阅者的发布

向无人订阅的 subject `Publish`（例如让一个实例用不同 `spring.config.bus.subject` 跑）：
调用返回 `nil`——NATS core 投递即发即忘。教训：发布方**拿不到确认**，无法得知是否有人
刷新；只能靠订阅方日志或应用行为佐证。你自己的发布不会出现在你自己的日志里，除非你
自己的订阅放行了它。

### 4.3 演练：错误 / 不匹配的 prefix

1. 以 `spring.config.bus.watch-prefixes=db,cache` 启动。
2. 发布 `db` → 刷新发生（`db` 在 watch 列表）。
3. 发布 `demo` → **不刷新，也没有任何日志**——`shouldRefresh` 返回 false 后消息被静默
   丢弃（bus.go:82-84）。这是预期的安静退出；别去 grep 错误。
4. 发布 `""` → 永远刷新，与 watch 列表无关。

### 4.4 演练：畸形 payload

直接向 subject 发布原始字节：

```bash
docker run --rm --network host nats:2.10 nats -s nats://127.0.0.1:4222 pub spring.config.refresh 'not-json{'
```

订阅方打 `ignoring malformed refresh event: ...`（Warn）并保持健康。**空** payload 不算
畸形——它是合法的全舰队刷新（bus.go:74-81）。带未知字段的 JSON 会被容忍（标准 unmarshal）。

### 4.5 演练：启动陷阱复现

- **缺 NATS 实例**：保留任意 `spring.config.bus.*` key、注释掉 `spring.nats.instances.config-bus.url`
  再运行——启动在容器装配阶段失败于无法解析的 `configBus` bean。没有 `enabled=false`
  逃生门；要么补 NATS 实例，要么连 `spring.config.bus.*` key 一起移除（bus 随之闲置）。
- **自 hosted key**：把 `spring.config.bus.subject=other.subject` 放进一个只随刷新到达的
  source（例如只出现在后续发布的 env 里）——bean 仍停留在启动期的 subject。启动期绑定
  按构造即胜出（§2.2）。

### 4.6 刷新失败可观测性

若某个 source 重载失败（文件坏、内容非法），`RefreshProperties` 返回错误，订阅方打
`config bus: property refresh failed: ...`；先前的属性值继续生效（按 app.go 文档注释，
刷新是全有或全无）。信号不会重试——修好 source 后重新发布。

---

## 5. 排障表

| 症状 | 可能原因 | 处置 |
|------|----------|------|
| 启动时装配 `configBus` 失败 | `spring.nats.instances.*` 下没有 `nats-instance` 指名（默认 `config-bus`）的实例 | 定义 `spring.nats.instances.config-bus.url=...`，或把 `nats-instance` 指到已有实例名 |
| 发布成功但没人刷新 | subject 不一致（舰队裂分），或所有订阅方的 `watch-prefixes` 都不含发布的 prefix | 各实例对齐 `subject`；检查重叠方向（`db` 匹配 `db.pool`，不匹配 `demo`） |
| 别的实例刷了、这台没刷 | 本实例启动晚——NATS core 无回放，`subscribe()` 之前的信号已丢 | 全部实例就绪后再发布；bus 不是持久日志 |
| `property refresh failed: app not started yet` | 事件落在 `app.started` 置位前的装配窗口（§2.1） | 启动期属良性；变更重要则就绪后重发 |
| `property refresh failed: <source 错误>` | 某配置 source 重载失败；旧值保留 | 修复 source 后重新发布——不会自动重试 |
| 刷新后字段没变 | 绑定是普通 `value:` 字段而非 `gs.Dync[T]`——只有 Dync 重解析 | 把字段改成 `gs.Dync[T]` |
| 改了 `spring.config.bus.subject`/`watch-prefixes` 行为不变 | 这些 key 仅启动期生效，`subscribe()` 只解析一次 | 重启实例 |
| 改了 Dync 读取的 key 但值过期 | 新值在应用不加载的 source 里，或被优先级压住（如文件压过 env） | 从刷新日志确认哪些 source 被重载；确认变更出现在已加载 source 中 |
| bus 没有任何 metrics / 健康检查 | 本就没有——只有 `_app_config_bus` 日志 | 盯日志；订阅悄悄掉线时无其他信号（见 §6） |

---

## 6. 设计体检表

| 指标 | 数值 |
|------|------|
| 配置 key 总数 | 3 |
| 其中必填 | 显式 0，隐式 1（被引用的 NATS 实例定义） |
| quickstart 前置外部依赖 | 1（NATS broker） |
| "注意/坑"条数 | 5 |

设计嫌疑清单（审计台账）：

- **无激活条件**（沿用上轮审计）：已修复——bean 现在带 `OnProperty("spring.config.bus")`
  门（同 governance sources），未配置时导入闲置。
- **无投递确认**：NATS core 即发即忘，发布方无法得知是否有实例刷新——协调式 rollout
  前考虑 reply/ack 模式或可观测计数器。
- **自身 key 仅启动期生效**：`subject`/`watch-prefixes`/`nats-instance` 静默忽略热加载；
  刷新时检测到自身 key 变化应打 warn 暴露误用。
- **可观测性只有日志**：无健康指示器、无 metrics（收到事件数 / 刷新成功数 / 失败数）——
  订阅死掉是静默的。
- **scoped 退出是静默的**（已修复）：被过滤的事件现在打一条 debug 级日志
  （bus.go:82-87），"这台为什么没刷"免重启即可诊断。
- **刷新粒度**：`Prefix` 过滤实例而非 key——每个放行实例都刷新全量属性。若证明开销大，
  可能需要 key 级刷新路径。
