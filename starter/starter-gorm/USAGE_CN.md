# starter-gorm (gormcore) 使用说明 — 参考手册

所有 gorm 方言 starter(mysql、postgres、sqlite、sqlserver、clickhouse)背后的共享脚手架的
详细使用参考。应用不直接 import 本模块——import `starter-gorm-<dialect>` 即可获得本文档的
全部能力。所有行为声明均经本模块源码核对(`gorm.go`、`open.go`、`module.go`、
`extension.go`、`health.go`、`observe/plugin.go`、`resilience/callbacks.go`)并锚定
`starter-gorm-mysql/example*` 下可运行的示例。**GORM 语义(模型、关联、事务、migrator)见
[gorm 官方文档](https://gorm.io/docs/)**——以下全部是 go-spring 增量:装配、配置绑定、
可观测、服务治理、健康检查、停机。**

本文档是**唯一共享参考**;各方言 USAGE 只写自己的 DSN/TLS/服务发现 key,其余指向这里。

---

## 1. 完整工程示例

一个使用 gorm 接入 MySQL、带健康探针、指标、trace 与运行期故障注入的真实服务。文件树:

```
demo/
├── go.mod
├── main.go
├── dao.go
├── conf/
│   ├── app.properties
│   └── govern.yaml
```

**go.mod**(关键依赖):

```
require (
    gorm.io/gorm                latest
    gorm.io/driver/mysql        latest
    go-spring.org/spring         v1.3.x
    go-spring.org/starter-gorm-mysql latest  // 传递引入 starter-gorm (gormcore)
    go-spring.org/starter-actuator latest   // 可选:探针 + /metrics
    go-spring.org/starter-otel     latest   // 可选:真实 trace/指标导出
    go-spring.org/starter-governance latest // 可选:运行期故障注入
)
```

**main.go**:

```go
package main

import (
    "demo/dao"

    "go-spring.org/spring/gs"
    _ "go-spring.org/starter-actuator"
    _ "go-spring.org/starter-governance"
    _ "go-spring.org/starter-gorm-mysql" // 注册 spring.gorm.mysql.instances.* 下的实例
    _ "go-spring.org/starter-otel"
)

func main() { gs.Run(dao.NewRepository) }
```

**dao.go**——应用的全部数据访问面:

```go
package dao

import (
    "context"

    gormcore "go-spring.org/starter-gorm"
    _ "go-spring.org/starter-gorm-mysql"
    "gorm.io/gorm"
)

// User 演示模型(gorm 语义:https://gorm.io/docs/models.html)。
type User struct {
    ID    uint   `gorm:"primaryKey"`
    Email string `gorm:"size:128;uniqueIndex"`
    Name  string `gorm:"size:64"`
}

// Repository 按名注入 "primary" 实例。bean 类型是共享的 gormcore.DB
// （内嵌 *gorm.DB，所有 gorm 方法原样提升），所有方言 starter 共用同一类型。
// 第二个实例("replica")用 autowire:"mysql.replica"；每个实例各自贡献 health indicator。
type Repository struct {
    DB *gormcore.DB `autowire:"mysql.primary"`
}

func NewRepository(r *Repository) { /* 注册为 Rooter 或提供 HTTP handler */ }

var _ = func() any {
    // 扩展点(方言无关,所有 gorm starter 共享):在 bean 返回前调整每个
    // 刚打开的 *gorm.DB。必须在 init 函数里调用,先于容器装配。
    gormcore.UseDBCustomizer(func(db *gorm.DB) error {
        // 例如:prepared-statement 缓存、额外 Plugin、配置未暴露的连接池旋钮。
        // 第一个 error 会让该实例创建失败。
        return nil
    })
    return nil
}()

// PoolStats 不依赖 OTel 暴露运行期连接池水位。
func (r *Repository) PoolStats() (open, inUse, idle int) {
    st, err := gormcore.Stats(r.DB)
    if err != nil {
        return 0, 0, 0
    }
    return st.OpenConnections, st.InUse, st.Idle
}

var _ = context.Background
```

**conf/app.properties**——完整共享面(方言自身的 `user/password/addr/db/tls/...`
key 见各方言 USAGE):

```properties
# --- gorm mysql 实例 "primary"(方言 key 略)-------------------------------
spring.gorm.mysql.instances.primary.user=root
spring.gorm.mysql.instances.primary.password=123456
spring.gorm.mysql.instances.primary.addr=127.0.0.1:3306
spring.gorm.mysql.instances.primary.db=test

# 本参考文档覆盖的共享 key(Common 块):
spring.gorm.mysql.instances.primary.max-open-conns=10
spring.gorm.mysql.instances.primary.max-idle-conns=5
spring.gorm.mysql.instances.primary.conn-max-lifetime=30m
spring.gorm.mysql.instances.primary.conn-max-idle-time=5m
spring.gorm.mysql.instances.primary.ping-timeout=5s
spring.gorm.mysql.instances.primary.slow-threshold=200ms

# --- actuator(聚合每个 gorm 实例的 health indicator)--------------------
spring.http.server.enabled=false
spring.actuator.addr=:9370

# --- 可观测(starter-otel)-------------------------------------------------
spring.observability.service-name=demo
spring.observability.trace.exporter=otlp-grpc
spring.observability.trace.endpoint=127.0.0.1:4317
spring.observability.metrics.exporter=prometheus
spring.observability.metrics.port=0        # /metrics 仅经 actuator

# --- 服务治理(运行期故障注入 / 熔断 / 重试)-------------------------------
# NOTE: governance RULES go in conf/govern.properties, referenced by govern.source.file.path in app.properties (see starter-governance USAGE).
govern.enabled=true
govern.driver=default
govern.default.enabled=true
govern.default.error-threshold=20
govern.default.open-duration=5s
govern.default.max-retries=1
govern.default.timeout=500ms
```

前置外部依赖(docker):

```bash
docker run -d --name mysql -e MYSQL_ROOT_PASSWORD=123456 -e MYSQL_DATABASE=test -p 3306:3306 mysql:9
docker run -d --name jaeger -p 16686:16686 -p 4317:4317 jaegertracing/all-in-one:latest
```

**验证**:

```bash
go run .                                        # 容器启动;client 创建期即 ping
curl -s :9370/readyz                            # UP,包含 gorm:mysql:primary
curl -s :9370/metrics | grep db_client_operation_duration
# Jaeger UI:http://127.0.0.1:16686 —— service "demo",每个操作一个 "query" span
```

---

## 2. 装配与时序

### 2.1 bean 生命周期时间线

```
import starter-gorm-mysql
  └─ init: gormcore.Module(Dialect[Config]{Prefix:"spring.gorm.mysql", ...})
        └─ gs.Module(gs.OnProperty("spring.gorm.mysql"))   [前缀判定:任一条目即触发]
gs.Run()
  ├─ 配置绑定:conf.BindEach 遍历 ${spring.gorm.mysql} → 每个 <name> 一份 Config
  ├─ 每个实例 <name>:
  │    ├─ Dialect.Build(ctx, c)   方言构造 DSN/dialector;处理 TLS、服务发现
  │    │                          (仅 mysql 等)、resource label
  │    ├─ gormcore.Open:  gorm.Open → ApplyPool(连接池旋钮 + fail-fast ping)
  │    │                  → ApplyDBCustomizers(用户 seam,按注册顺序)
  │    ├─ Provide *DB .Name(<dialect>.<entry>).Init((*DB).Init).Destroy((*DB).Destroy)
  │    └─ Provide health.Indicator "gorm:mysql:<name>"(按名注入上面的 *DB,
  │       导出为 health.Indicator)  ← .Name 必须加:多实例 bean 共享类型
  │          (Indicator、*DB);没有独立名字容器会报 (Name,Type) 重复键
  ├─ bean 装配:gs 按名组装各 *DB wrapper bean
  ├─ DB.Init:observe 插件(observe.enabled=false 除外)→ resilience
  │    executor 链 → ApplyCallbacks 替换六个 gorm processor
  ├─ Run / 就绪:actuator 聚合各 indicator → /readyz UP
  └─ SIGTERM:DB.Destroy → executor.Close → 方言 closers(停 discovery watch、
     注销 TLS)→ 关闭底层 *sql.DB 连接池
```

打开阶段任一步失败(方言构建、gorm.Open、ping、customizer)都会让该实例创建失败并
执行方言的 closers——地址/凭据配错在启动期暴露,而不是第一次查询时。

### 2.2 一次查询的回调链——精确顺序与设计理由

`db.Raw("SELECT ...")`(或 First/Create/Update/Delete/Row)执行:

```
gorm:query processor 链
  1. go-spring:observe:before_query   span 开始 + metric 开始 + in-flight +1
  2. gorm:query  ← 已被 resilience wrapper 替换:
        resilience.Run(ctx, exec, "gorm:mysql:<addr>", op)
          ├─ 准入(限流 / 舱壁,若已配置)
          ├─ fault 注入器(govern.fault.*——可能短路本次尝试)
          ├─ timeout / breaker / retry 包络
          └─ 原始 gorm:query 主体:构造 SQL、执行、应用 gorm 自身
             logger(慢查询 warn,见 slow-threshold)
  3. go-spring:observe:after_query    SetArg(SQL) → span 结束 + 时长 metric
                                      + in-flight -1 + 访问日志记录
```

设计理由(源码注释核对):

- **observe 钩子挂在 gorm processor 外侧而非 driver 层**(observe/plugin.go:44-50):
  SQL 语句要到 gorm 构建完才知道,所以 Before 回调只为计时打开观测,After 回调再经
  SetArg 附上 SQL 后 End——语句因此同时进入 detailed 日志和 span。
- **关联键是 *gorm.DB 指针**(observe/plugin.go:39-43):gorm 把同一个新建的
  `*gorm.DB` 交给两个回调,指针即一次飞行中操作的唯一键(`sync.Map`)。
- **resilience 替换 processor 本体**(resilience/callbacks.go:53-90):其他 starter 用
  redis Hook、grpc interceptor 驱动的同一个后端中立 Executor,这里经由 gorm 回调链驱动
  ——一份共享实现替代五份方言拷贝。
- **`gorm.ErrRecordNotFound` 视为成功**(resilience/callbacks.go:29-30):"查无此行"
  是正常结果不是故障,不得触发熔断(DB 侧的 redis.Nil 对应物)。
- **拒绝错误传播**(resilience/callbacks.go:76-83):resilience 拒绝(限流/熔断开/
  舱壁满)或 fault 注入错误会写到 `tx.Error`;真实操作错误保持 gorm 原样。
- **默认零开销**:未配置治理时 `resilience.ExecutorFor` 返回透明 executor——回调仍包裹
  但无任何行为,链路留在原地不产生成本。

### 2.3 事务

`db.Transaction(...)` 跑在 session `*gorm.DB` 上;被替换的 processor 被 session 继承,
事务内每条语句各自被观测与保护(事务整体不是独立 span)。

---

## 3. 逐 key 行为参考

所有 key 位于 `spring.gorm.<dialect>.<name>.*`(内嵌 `Common`,与方言自身字段同级绑定)。
已与 `grep -rhoE 'value:"[^"]+"' starter/starter-gorm` 核对——共享 key 恰为这 10 个。

| Key | 类型 | 默认值 | 行为 / 联动 | 配错后果 |
|-----|------|--------|------------|----------|
| `max-open-conns` | int | 0 | `>0` → `sql.DB.SetMaxOpenConns`;0 = database/sql 不限。 | 并发下太小 → `WaitCount` 增长、查询排队(经 `gormcore.Stats` 可见)。 |
| `max-idle-conns` | int | 0 | `>0` → `SetMaxIdleConns`;0 = database/sql 默认(2)。 | 高 QPS 下 0 → 反复重连抖动;> max-open 会被 database/sql 收敛。 |
| `conn-max-lifetime` | duration | 0 | `>0` → `SetConnMaxLifetime`;0 = 不限。 | 0 且 LB 静默丢弃空闲连接 → 运行中期报 stale connection。 |
| `conn-max-idle-time` | duration | 0 | `>0` → `SetConnMaxIdleTime`;0 = 不限。 | 主要与上面 lifetime 联动。 |
| `ping-timeout` | duration | 5s | 启动 fail-fast ping(`PingContext`)的时限。`<=0` 回落 5s。 | 太小 → 冷启动 DB 时启动失败;太大 → DB 宕机时启动缓慢。 |
| `slow-threshold` | duration | 0 | `>0` 安装 warn 级 gorm 慢查询 logger,输出经 **go-spring.org/log**(`log.Warnf`,TagAppDef)转发——进配置的 appender,而非裸 stdout。0 保持 gorm 默认 logger。⚠ 消息体是 GORM 的单行文本,不是结构化字段。 | 0 → 完全没有慢日志;要结构化字段 → 消息体是纯文本(改用访问日志)。 |
| `service-name` | string | — | 切换为服务发现寻址:方言绑定 discovery 拨号器,每条新连接到达存活实例。设置后 `addr` 被忽略(示例故意用 dummy `0.0.0.0:0` 证明)。mesh 模式(`GS_MESH=on`)下 sidecar 接管发现,`addr` 原样使用。 | 不设且无 `addr` → 方言构建报错("one of addr or service-name must be set")。 |
| `scheme` | string | — | 把发现收窄到单一传输 scheme(如 `tls`)。⚠ 未设 `service-name` 时为死 key(仅在此时被读取)。 | 设了但无 service-name → 静默忽略。 |
| `discovery` | string | — | 选择解析 `service-name` 的已注册 discovery 后端。⚠ 未设 `service-name` 时为死 key。 | service-name 已设但 discovery 未配置或名字无对应 bean → 启动报错；设了但无 service-name → 静默忽略。 |
| `observe.enabled` | bool | true | gorm observe 插件的硬开关:false 时插件完全不安装——无 span、无 metric、无访问日志、无逐查询回调。 | false → 逐查询可观测静默消失(为高吞吐实例有意为之)。 |

死 key 说明:对 **sqlite** 而言整个发现三件套(`service-name`/`scheme`/`discovery`)结构性
不可用(无服务可发现)但仍会绑定——已记入嫌疑清单。

---

## 4. 验证与故障演练

### 4.1 observe 信号(逐查询)

import starter-otel 后(配置见 §1):

- **span**:每操作一个,以操作种类命名(`query`/`create`/`update`/`delete`),属性
  `db.system=mysql`,附带 SQL 语句;查 Jaeger(`http://127.0.0.1:16686`,
  service = 你的 `spring.observability.service-name`)。
- **指标**:直方图 `db.client.operation.duration`,属性 `db.system`、`db.operation`、
  status:

```bash
curl -s :9370/metrics | grep -E 'db_client_operation_duration'
```

- **访问日志**:tag `_app_gorm_access`,每操作一条结构化记录——system、operation、
  status、duration、error,以及 gorm 生成后的 SQL 语句(按 512 字节截断)。
  错误 → Warn;无 SQL 的成功 → Info;带 SQL 的成功 → Debug。

```bash
go run . 2>&1 | grep _app_gorm_access
```

### 4.2 慢日志演练

1. 配置 `slow-threshold=200ms`(如 §1)。
2. 触发一条慢查询(需作为独立语句执行):

```bash
# 经你应用的 SQL 入口,或加一个调试 handler 执行:
#   db.Raw("SELECT SLEEP(1)").Scan(&x)
```

3. 预期约 1s 后经 **go-spring.org/log**(TagAppDef,消息体为纯文本)出现一条 gorm 慢查询 warn。
   同一条查询无论阈值如何也会出现在访问日志 / 时长直方图中。

### 4.3 健康翻转演练(停库 → readiness 变 DOWN)

```bash
go run . -manual &                      # 或你的常驻模式
curl -s :9370/readyz                    # 200 UP —— 包含 gorm:mysql:primary
docker stop mysql                       # 停掉数据库
curl -s :9370/readyz                    # 503 DOWN —— 实例级 indicator
                                        # (连接池 PingContext)失败
docker start mysql && sleep 3
curl -s :9370/readyz                    # 库恢复后回到 200
```

indicator 名为 `gorm:<dialect>:<name>`(如 `gorm:mysql:primary`);actuator 的
`/health` 展示组件明细。健康 ping **不经过**回调链(直接 `sqlDB.PingContext`,
health.go:31-38),因此探针失败不会触发 resilience 熔断。

### 4.4 故障 / resilience 演练(免重启)

用 §1 的治理配置加文件 source(见 starter-governance)或 example-load 布局
(`starter-gorm-mysql/example-load`):

1. 以 `govern.fault.enabled=false` 启动;基线查询全部成功。
2. 翻转 `govern.fault.enabled=true`(配 `rate`、`error`)——治理 source 热加载。
3. 被注入的尝试在 SQL 执行前短路:注入错误落到 `tx.Error`,熔断计数,observe 层仍
   记录失败操作——可以在访问日志和 `db.client.operation.duration` 的错误 status 桶
   里观察"火情"。
4. `error-threshold=20` 下持续放火熔断打开:后续查询以 circuit-open 拒绝快速失败而
   不触库;`open-duration=5s` 后半开试探。
5. 翻回 false 灭火(必要时经治理面 reset 熔断器)。

### 4.5 连接池验证

`gormcore.Stats(db)` 不依赖 OTel 返回 `sql.DBStats`——在调试端点暴露它,压测时观察
`OpenConnections`/`InUse`/`WaitCount`(`example-load` 压测环驱动的正是这个循环)。

---

## 5. 排障表

| 症状 | 可能原因 | 处置 |
|------|----------|------|
| 启动失败 "gorm ping: ..." | `ping-timeout` 内 DB 不可达/凭据错误 | 修 addr/凭据;冷启动调大 `ping-timeout`。这是有意的 fail-fast,不是 bug。 |
| 一个 bean 都没注册 | 无任何 `spring.gorm.<dialect>.*` 条目——`OnProperty(prefix)` 未触发 | 至少加一个实例块;默认不装配。 |
| 容器报 duplicate beans | 又 Provide 了未 `.Name` 的 `*DB`/`health.Indicator` | 不要自行 Provide DB bean；DB bean 名为 `<dialect>.<entry>`（如 `mysql.primary`），health indicator 名为 `gorm:<dialect>:<entry>`（module.go:85-97）。 |
| 注入报 "not a simple value"/类型不匹配 | 注入 `*gorm.DB` 而非 wrapper | autowire 共享的 `*gormcore.DB` bean；它内嵌 `*gorm.DB`。 |
| 无 span/指标/访问日志 | 未 import starter-otel,或 `observe.enabled=false` | import starter-otel;检查实例级硬开关——false 会整体移除插件。 |
| 慢查询行是纯文本 | `slow-threshold` 把 GORM 的 warn 输出经 go-spring.org/log 转发,但消息体是 GORM 单行文本 | 按消息过滤;要结构化慢日志改用访问日志。 |
| 查询被 rate-limited/circuit-open 拒绝 | 治理 resilience 生效(或 fault 放火中) | 属预期保护;查 `govern.*` 配置与演练步骤(§4.4)。 |
| 运行数小时后报 stale connection | LB/防火墙掐空闲 TCP;`conn-max-lifetime=0` | 把 `conn-max-lifetime` 设为低于基础设施空闲阈值。 |
| "正常 not found 会触发熔断"——不会 | `gorm.ErrRecordNotFound` 视为成功 | 设计如此(callbacks.go:29-30);只有真实错误喂熔断。 |

## 6. 设计体检表

| 指标 | 数值 |
|------|------|
| 共享配置 key | 10(Common) |
| 必填 | 此处 0(方言自有必填,如 mysql 的 `user`/`db`) |
| quickstart 外部依赖 | 1(数据库;全量可观测 +1 collector) |
| "注意/坑"条数 | 4 |

设计嫌疑(交审计台账):slow-threshold logger 的消息体是 GORM 纯文本(2026-08 起已改经
go-spring.org/log 转发,不再是 stdlib stdout);sqlite 方言已不再嵌入发现三件套——
只嵌 `PoolSettings`;只有逐操作 span 没有事务级 span(事务内语句仅靠
context 关联)。
