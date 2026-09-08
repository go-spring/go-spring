# starter-xxljob 使用说明 — 参考手册

详细使用参考。总览见 [README_CN.md](README_CN.md)。所有行为声明均已对照 starter 源码
（`starter.go`、`config.go`、`executor.go`、`protocol.go`、`registry.go`）与自包含可运行的
[example/](example/)（冒烟：`example/check.sh`，无需 docker）核实。**任务语义——调度、
路由/阻塞策略、admin 控制台——见 [xxl-job 官方文档](https://www.xxl-job.com)**。executor
协议为本仓手写（无第三方 Go SDK），覆盖 admin 回调 `/run`、`/beat`、`/idleBeat`、
`/kill`、`/log`；本文写 go-spring 增量与我们实现的确切协议面。

**激活条件**：任一 `spring.xxljob.<name>` 子树。多实例：每个条目一个 executor。三个 key
绑定期必填（`app-name`、`admin-addresses`、`port`——config.go:30-41 的 `expr` 校验），
配一半的实例会在启动期失败，而不是首次触发时。

---

## 1. 完整工程示例

example 内嵌 mock admin（仅够注册与触发的 REST），让 触发→执行→回调 全链路本地跑通——
即 [example/example.go](example/example.go)。生产环境把 `admin-addresses` 指向真实
xxl-job-admin。文件树：

```
demo/
├── go.mod
├── main.go            # executor + handler 注册 + 自测
└── conf/
    └── app.properties
```

**go.mod**（见 `example/go.mod`）：

```
module demo

require (
    go-spring.org/spring        v1.3.x
    go-spring.org/starter-xxljob latest
)
```

**main.go**（节选自 example——mock admin 脚手架省略，见原文件）：

```go
package main

import (
    "context"
    "net/http"

    "go-spring.org/spring/gs"

    starter "go-spring.org/starter-xxljob"
)

type Service struct {
    Executor *starter.Executor `autowire:"a"`
}

// Init 在 executor bean 注入之后、回调 server 开始服务之前注册 handler
// （executor.go:61-63：server 起来后名字集合即固定）。
func (s *Service) Init() error {
    s.Executor.RegisterHandler("demoJob", func(ctx context.Context, param string) error {
        // nil => 向 admin 报成功；非 nil => 失败 + 消息。
        // 需要 admin 经 /log 看到任务日志的话，自行写 <log-dir>/<logId>.log
        // （starter 只负责"服务"该目录）。
        return nil
    })
    return nil
}

func main() {
    gs.Provide(&Service{}).Export(gs.As[gs.Rooter]()).Init((*Service).Init)
    gs.Run()
}
```

**conf/app.properties**——完整注释配置面（复制自 `example/conf/app.properties`）：

```properties
# --- xxl-job executor 实例 "a" -------------------------------------------------
spring.xxljob.a.app-name=go-spring-demo          # 注册到 admin 的名字
# 生产：http://<admin-host>:8080/xxl-job-admin（列表 = 负载均衡）。
spring.xxljob.a.admin-addresses=http://127.0.0.1:18081
# 回调 server 端口——admin 必须能回拨；显式运维决策（无默认端口）。
spring.xxljob.a.port=9999
# 重注册/心跳周期（默认 10s；example 为冒烟调小）。
spring.xxljob.a.registry-interval=5s
# 任务日志目录，经 /log 回传；目录由 starter 建/读，文件由你的 TaskFunc 写。
spring.xxljob.a.log-dir=./logs
# 仅当 admin 设置了 token 时配置（发送 XXL-JOB-ACCESS-TOKEN 头）。
# spring.xxljob.a.access-token=...
```

**验证**（与 `example/check.sh` 同构）：

```bash
go run .                          # 期望输出："xxl-job round trip OK: demoJob ran"
./example/check.sh                # 脚本化 marker 断言
# 手动模式（go run . -manual）直接驱动回调 server：
curl -s -XPOST :9999/beat                          # {"code":200,"msg":""}
curl -s -XPOST -d '{"jobId":1,"executorHandler":"demoJob","executorParams":"a=1","logId":1}' :9999/run
curl -s -XPOST -H 'Content-Type: application/json' -d '{"jobId":1}' :9999/idleBeat  # 空闲 200 / 运行中 500 "job running"
curl -s ":9999/log?logId=1&fromLineNum=0"          # LogResult JSON
```

---

## 2. 装配与时序

### 2.1 bean 生命周期时间线

```
import starter-xxljob
  └─ init: gs.Module(gs.OnProperty("spring.xxljob"))              [starter.go:30]
        └─ conf.BindEach("${spring.xxljob}", 逐 <name>):
             ├─ Config 的 expr 校验（app-name != ''、admin-addresses
             │  非空、port > 0——绑定期即失败）
             ├─ Provide(newExecutor).Name(<name>).
             │   Export(gs.As[gs.Server]()).Destroy(...)           [starter.go:32-35]
             └─ Provide(health.Indicator).Name("xxljob:"+<name>).
                Export(gs.As[health.Indicator]())                  [starter.go:37-39]
gs.Run()
  ├─ bean 装配：应用按实例名 autowire *Executor
  ├─ Rooter Init 阶段：应用 Service.Init 调 RegisterHandler
  ├─ newExecutor（构造期，executor.go:72-90）：mux 挂
  │   /run /beat /idleBeat /kill /log；http.Server 监听 :port
  │   （ReadHeaderTimeout 5s）
  ├─ Runner 阶段：Executor.Run（executor.go:94-112）
  │   ├─ prepare()：outboundIP() 经 UDP 拨 8.8.8.8（registry.go:100-108）、
  │   │   ensureLogDir() 建目录
  │   ├─ register(ctx)：ticker 循环——registryOnce() 向每个 admin 的
  │   │   /api/registry POST {"registryGroup":"EXECUTOR",
  │   │   "registryKey":app-name,"registryValue":"http://<ip>:<port>/"}；
  │   │   stop 函数向 /api/registry/remove POST
  │   ├─ sig.TriggerAndWait() → 就绪
  │   └─ ListenAndServe 直到 ctx.Done → srv.Shutdown
  └─ SIGTERM 时：Stop → http.Server.Shutdown(ctx)（排空回调）；
       defer 的 stopRegistry 顺路摘除注册
```

### 2.2 源码注释中的设计理由

- **端口必填、无默认**："an executor server must be reachable by the admin to receive
  trigger callbacks, so the port is an explicit operator decision"（config.go:37-41）。
- **协议手写、无 SDK**："It speaks the xxl-job executor protocol to an admin ... over
  plain HTTP, hand-rolled (no third-party SDK) — see DESIGN"（config.go:25-27）。
- **任务跑独立 goroutine、ctx 可取消**："`/kill` can interrupt a long task; a panic in a
  task is recovered through the shared goutil panic chain"（executor.go:17-21；执行路径用
  `goutil.SafeRun` 包裹，executor.go:167-169）。
- **/run 立即返回**："runs a task in a new goroutine and returns immediately; the task
  posts its completion back to the admin via /api/callback"（executor.go:142-143）——
  xxl-job 的异步触发模型。

### 2.3 一次触发逐层走读

1. admin 的调度器触发任务 `1` → 从注册表选中本 executor，POST `/run`，body 为
   `TriggerParam`（`jobId`、`executorHandler`、`executorParams`、`logId` 等）
   （protocol.go:27-40）。
2. `handleRun`（executor.go:144-178）解码、查 `registry[executorHandler]`；未注册的
   handler → `{"code":500,"msg":"no handler registered for ..."}`，任务不跑。
3. 可取消 ctx 存为 `running[jobId]`，任务在新建 goroutine 上启动。
4. `/run` 立刻回 `{"code":200}`（异步模型）；超时由 admin 侧
   （`executorTimeout`）掌控。
5. 任务体在 `goutil.SafeRun` 下运行：error 变 `{code:500, msg:err}`；panic 由共享
   panic 链 recover 并按失败上报。
6. 完成后 executor 向**每个** `admin-addresses` 的 `/api/callback` POST 官方
   `HandleCallbackParam` JSON 数组（`[{logId, logDateTime, handleCode, handleMsg}]`），
   配置了 token 时带 `XXL-JOB-ACCESS-TOKEN`；回调上下文不受 kill 取消影响，
   被杀任务的结果仍会上报。
7. 期间 `/beat` 答存活、`/idleBeat`（JSON body `{"jobId":N}`）答该 job 是否在跑
   （阻塞策略输入）、`/kill`（JSON body `{"jobId":N}`）取消该 job 运行中任务的 ctx、
   `/log?logId=&fromLineNum=` 从 `log-dir/<logId>.log` 服务日志切片。

---

## 3. 逐 key 行为参考

前缀：`spring.xxljob.<name>.*`（多实例）。

| Key | 类型 | 默认值 | 行为 / 联动 | 配错后果 |
|-----|------|--------|------------|----------|
| `app-name` | string | — | admin 分组 executor 的注册 key。必填（`expr:"$ != ''"`）。 | 绑定期启动报错。 |
| `admin-addresses` | []string | — | admin 基地址列表；注册/心跳与任务回调**向所有条目**扇出。必填（`len > 0`）。 | 绑定期启动报错；URL 错但存在 → 注册不上去，只能从注册日志静默发现。 |
| `port` | int | — | 回调 server 端口；admin 必须可回拨。必填（`> 0`）。 | 绑定期启动报错；端口冲突在 `ListenAndServe` 处作为 Run 错误浮出。 |
| `access-token` | string | 空 | 非空时在 executor→admin 调用上带 `XXL-JOB-ACCESS-TOKEN`。⚠ 必须与 admin 侧 token 完全一致；空只匹配无 token 的 admin。 | 不一致 → admin 静默丢弃注册/回调。 |
| `registry-interval` | duration | 10s | 重注册/心跳周期。⚠ 须明显小于 admin 的注册过期（原版 xxl-job 默认 90s），否则 executor 掉线抖动。 | 过大 → admin 视角 executor 周期性离线；触发被路由到别处。 |
| `log-dir` | string | `./logs` | `/log` 以 `<logId>.log` 服务该目录；目录缺失时 starter 会创建。⚠ 职责分工：starter 只负责**读**这一侧——写 `<log-dir>/<logId>.log` 由你的 TaskFunc 负责。 | admin 控制台日志为空。 |

---

## 4. 验证与故障演练

### 4.1 触发往返（任务执行）

```bash
go run .                                        # "xxl-job round trip OK: demoJob ran"
./example/check.sh                              # 脚本化 marker 断言
```

### 4.2 探测回调端点（手动模式）

```bash
go run . -manual &
curl -s -XPOST :9999/beat ; echo
curl -s -XPOST -H 'Content-Type: application/json' \
     -d '{"jobId":1,"executorHandler":"demoJob","executorParams":"a=1","logId":1001}' :9999/run ; echo
curl -s -XPOST -H 'Content-Type: application/json' -d '{"jobId":1}' :9999/idleBeat ; echo  # 任务运行中 500 "job running"
curl -s -XPOST -H 'Content-Type: application/json' -d '{"jobId":1}' :9999/kill ; echo      # 200 取消；否则 500 "job not running"
curl -s ":9999/log?logId=1001&fromLineNum=0" ; echo
```

### 4.3 kill 演练（中断长任务）

注册一个 `select { case <-ctx.Done(): ... case <-time.After(30*time.Second): }` 的
handler，触发后向 `/kill` POST `{"jobId":1}`——任务观察到 ctx 取消，完成回调上报中断。
运行中任务按 `jobId` 记账（admin 对 /kill 与 /idleBeat 用的键）；触发参数里的 `logId`
只透传到 `/api/callback` 载荷与 `/log`。example 用 `jobId=2, logId=2002` 触发并按
jobId kill，证明二者分离（`example/example.go` 的 "kill round trip OK"）。

### 4.4 失败上报

让 handler 返回 error：admin 侧回调带 `handleMsg:<err>` 与 `handleCode:500`。panic 被
recover（goutil 链）并按失败上报——进程存活。

### 4.5 退出排空

任务运行中 `kill -TERM`：`Stop` 调 `http.Server.Shutdown(ctx)`——回调 server 排空
在途 HTTP；starter 不强制停掉运行中的任务 goroutine（停机不做 cancel 清扫）。摘除注册
的请求在退出路径上补发。

---

## 5. 排障表

| 症状 | 可能原因 | 处置 |
|------|----------|------|
| 启动期配置报错 | `app-name`/`admin-addresses`/`port` 缺失或为空 | 三者绑定期 expr 必填（config.go:30-41）。 |
| executor 收不到触发 | `port` 对 admin 不可达（防火墙/NetworkPolicy）；出网 IP 误判 | 查 admin 的 executor 列表是否显示 `http://<ip>:<port>/`；`outboundIP` 用 UDP 拨 8.8.8.8（registry.go:100-108）——多网卡主机可能选错网卡。 |
| `/run` 回 500 "no handler registered" | handler 未在服务前注册，或与 admin 的 JobHandler 名不一致 | 在 Rooter `Init` 里注册（先于 Runner 阶段）；名字须完全一致。 |
| executor 注册后又掉线 | `registry-interval` 相对 admin 过期时间太大 | 保持明显小于 admin 的注册过期（原版默认 90s）。 |
| admin 任务日志为空 | starter 只服务 `log-dir`、从不写 | TaskFunc 里自己写 `<log-dir>/<logId>.log`（可用触发参数里的 logId 作文件名）。 |
| 终止任务无效 | POST 的 `jobId` 与触发的 job 不符（running 表按 `jobId` 记账） | 确认向 `/kill` POST JSON body `{"jobId":N}`，且与触发参数中的 jobId 一致。 |
| 健康显示 UP 但 admin 不可达 | 指示器只检查"server 已构建"（executor.go:253-260） | 关注注册日志；必要时自建 admin 探测。 |
| token 不一致、注册全无 | `access-token` 与 admin 侧不同 | 两侧对齐；空只匹配无 token 的 admin。 |

## 6. 设计体检表

| 指标 | 数值 |
|------|------|
| 配置 key 总数 | 6 |
| 其中必填 | 3（app-name、admin-addresses、port） |
| quickstart 前置外部依赖数 | 1（xxl-job admin；example 用 mock） |
| 注意/坑条数 | 4 |

设计嫌疑清单（保留旧版条目，另加新发现）：

- **2026-08-28 已修——/run 与 /kill+idleBeat 记账键错位**（原状：`running` 按 `LogID`
  记账，`/kill`+`/idleBeat` 按 `jobId` 查询参数查，对原版 admin kill 永远落空）：现
  running 表按 `jobId` 记账，`logId` 只透传给 `/api/callback` 与 `/log`；/kill 与
  /idleBeat 解码官方 JSON body（`KillParam`/`IdleBeatParam`）。由
  `executor_test.go`（TestKillByJobIDNotLogID）与 example kill 演练覆盖。
- **2026-08-28 已修——/api/callback 载荷形状**（原状：`LogResult` 形状的 body，原版
  admin 会静默丢弃）：现 POST 官方 `HandleCallbackParam` JSON 数组
  （`logId`/`logDateTime`/`handleCode`/`handleMsg`）；回调上下文也不受 kill 取消
  影响。由 TestCallbackPayloadShape 与 example 覆盖。
- 健康指示器只检查"server 已构建"，不校验向 admin 注册是否成功——admin 不可达只能在
  注册日志里发现。
- **2026-08-28 已修——健康指示器显示名与 bean 名不一致**（原状：显示名用 `app-name`
  而 bean 名用实例 `<name>`）：指示器现按实例名命名为 `xxljob:<name>`，与 bean 名及
  兄弟 starter 约定一致。由 TestHealthIndicatorNamedByInstance 覆盖。
- **2026-08-28 已修——删除死键 `observability.*` 绑定**：Config 的 Observability
  字段绑了但无人读；按死配置删除原则删掉了字段、schema 项与文档（未加观察层）。
- `/api/callback` 的 body 用 `LogResult` 形状而非原版 admin 的 `HandleCallbackParam`
  （`{logId, handleCode, handleMsg}`）；注释声称与原版 admin 不变往返
  （protocol.go:17-20），但只有 mock 验证过。
- **2026-08-28 已修——log-dir 职责分工显式化**（原状：starter 拥有 `log-dir` 与
  `/log` 读取却不写任务日志，分工无文档）：现已在 Config 字段与本文档写明——starter
  建目录并经 `/log` **服务**读取；应用侧 TaskFunc 负责写 `<log-dir>/<logId>.log`。
  starter 未新增文件写入。
