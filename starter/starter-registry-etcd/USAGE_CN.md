# starter-registry-etcd 使用说明 — 参考手册

详细使用文档,概览见 [README.md](README.md)。所有行为声明均对照 starter 源码(`starter.go`、
`config.go`、`registrar.go`、`discovery_etcd.go`、`registrar_weight_test.go`)与可运行的
[example/](example/)(`example/check.sh` = 单测 + docker 门控 etcd 端到端)核对。
**etcd 自身语义(lease、watch、KV、auth)见 [etcd 官方文档](https://etcd.io/docs/)**——
本文只写 go-spring 的增量。

**激活**:`spring.registry.etcd.endpoints` 即注册侧开关(`starter.go:66-71`)。
**消费侧**按具名块激活:每个 `${spring.discovery.etcd.<name>.endpoints}` 块注册一个
discovery 后端(`discovery_etcd.go:81-96`)。与 starter-registry-consul 不同,本 starter
**两侧都自带**:同一套 key 布局上的注册与发现。它不开端口——导出 `gs.Server` 只为把
注册接进应用生命周期。

---

## 1. 完整工程示例

一个 **provider**(本 starter + 一个被服务端口)和一个 **consumer**(任何按名解析的
discovery 感知 client starter)。文件树:

```
demo/
├── go.mod
├── main.go
├── provider.go
└── conf/
    └── app.properties
```

**前置依赖**(唯一外部系统):一个 etcd 节点——

```bash
docker run -d --name etcd -p 127.0.0.1:2379:2379 \
  gcr.io/etcd-development/etcd:v3.5.15 etcd --listen-client-urls http://0.0.0.0:2379 \
  --advertise-client-urls http://0.0.0.0:2379
curl -fsS http://127.0.0.1:2379/health        # {"health":"true",...}
```

**go.mod**:

```
require (
    go-spring.org/spring                   v1.3.x
    go-spring.org/starter-registry-etcd    latest
    go-spring.org/starter-redigo           latest   // 任一 discovery 感知的 client starter
)
```

**main.go**:

```go
package main

import (
    "go-spring.org/spring/gs"
    _ "go-spring.org/starter-registry-etcd"
)

func main() { gs.Run() }
```

**provider.go** —— 持有 registry server 以驱动运行期权重变更(可选):

```go
package main

import (
    "context"

    "go-spring.org/spring/gs"
    StarterRegistryEtcd "go-spring.org/starter-registry-etcd"
)

func init() {
    // 注册器 bean 名为 "registryServer",导出为 gs.Server。
    // 需要运行期摘流/恢复时注入它;否则可省略本文件。
    gs.Provide(func(s *StarterRegistryEtcd.Server) *Drainer {
        return &Drainer{srv: s}
    })
}

type Drainer struct{ srv *StarterRegistryEtcd.Server }

// Drain 不停进程就把实例摘出负载均衡:在同一个 lease 上改写存储权重
// (不重注册、key 不消失)。
func (d *Drainer) Drain(ctx context.Context) error { return d.srv.UpdateWeight(ctx, 0) }
func (d *Drainer) Restore(ctx context.Context) error { return d.srv.UpdateWeight(ctx, 100) }
```

**conf/app.properties**(逐字取自 `example/conf/app.properties`):

```properties
spring.app.name=registry-etcd-example

# 注册进的 etcd 集群。设置 endpoints 即激活 starter。
spring.registry.etcd.endpoints=127.0.0.1:2379
spring.registry.etcd.ttl=10s
spring.registry.etcd.key-prefix=/services/

# 广告的实例(后端无关;换注册后端是换 blank-import,不是改配置)。
spring.registry.service-name=orders
spring.registry.addr=127.0.0.1:8080
spring.registry.weight=100
spring.registry.metadata.zone=cn-north
spring.registry.metadata.version=v1

# 消费侧:名为 "local" 的 etcd discovery 后端,解析同一 key 前缀。
# 任何 client starter 的 `discovery:` 字段按名引用。
spring.discovery.etcd.local.endpoints=127.0.0.1:2379
spring.discovery.etcd.local.key-prefix=/services/

# 消费侧接线示例(任一 discovery 感知 client starter):
# spring.redis.demo.service-name=orders
# spring.redis.demo.discovery=local
```

**验证**(与 `example/check.sh` 同构):

```bash
go run ./example          # 日志: registered "orders" at 127.0.0.1:8080
                          #       discovered endpoint=127.0.0.1:8080 weight=100 ...
etcdctl get /services/orders/ --prefix
# /services/orders/orders-127.0.0.1:8080
# {"service_name":"orders","addr":"127.0.0.1:8080","weight":100,"metadata":{...}}
```

---

## 2. 装配与时序

### 2.1 bean 生命周期时间线

```
import starter-registry-etcd
  ├─ gs.Provide(NewServer).Name("registryServer").Export(gs.As[gs.Server]())
  │      Condition: OnProperty("spring.registry.etcd.endpoints")   [starter.go:66-71]
  ├─ gs.Module(OnProperty("spring.discovery.etcd"))                 [discovery_etcd.go:82]
  │      conf.BindEach → 每个 <name> 一个后端 → discovery.RegisterDiscovery(name, ...)
  │
gs.Run()
  ├─ 绑定: ${spring.registry.etcd} → EtcdConfig;${spring.registry} → Server.Config 字段
  ├─ NewServer: clientv3.New + 对 endpoints[0] 做 Status 探活——不可达/认证错
  │      的集群在此直接启动失败,而非等到首次 Register               [registrar.go:95-100]
  ├─ discovery 后端在绑定期做同样探活                                 [discovery_etcd.go:116-121]
  ├─ Runner 启动 → 就绪信号触发
  ├─ Server.Run: 校验 service-name/addr                              [starter.go:103-105]
  │      等待 <-sig.TriggerAndWait()(就绪门:所有 server 起来后才注册)
  │      Register → 日志 `registered "orders" at ...`                 [starter.go:114-121]
  ├─ 稳态: keep-alive goroutine 持续续约(clientv3 默认 ≈ TTL/3)
  └─ SIGTERM: PreStop 最先反注册(先于 pre-stop 延迟、先于任何 server 停止)
         → 在途请求继续排空时 discovery 已不再分发本实例
         Stop/StopContext 是幂等兜底                                    [starter.go:130-146]
```

设计理由(源码注释):注册绑定应用就绪——"实例在应用就绪后才发布……这一顺序让滚动重启
零损"(starter.go:30-36);崩溃安全从不依赖 Deregister——lease TTL 自动清 key,
"self-healing without a reaper"(starter.go:27-29)。

### 2.2 注册写入语义

- key = `<key-prefix><service-name>/<instance-id>`,id 取配置 `id`,否则派生
  `<service-name>-<addr>` —— 重启覆盖同一 key(registrar.go:111-122)。
- value = JSON `instanceValue{service_name, addr, weight, metadata}`
  (registrar.go:43-48,137-142)。
- Register 时权重 ≤0 归一为 1:"默认值"永不存成 0——**0 保留给运行期摘流信号**,
  只能经 `UpdateWeight` 达成(registrar.go:131-136)。
- `Register` = Grant(整秒 TTL) → `Put(key, val, WithLease)` → `KeepAlive` goroutine
  必须排干续约通道,否则 lease 死亡(registrar.go:147-170)。同实例重注册会先注销旧
  lease(registrar.go:172-179)。
- `ttlSeconds()` 向上取整到整秒、最小 1s;`TTL<=0` 静默变 15s(config.go:85-98)。

### 2.3 WATCH 链路(消费侧)

`etcdDiscovery.Watch`(discovery_etcd.go:158-197):

1. `clientv3.Watch(ctx, prefix, WithPrefix())` 打开 etcd watch 通道
   (discovery_etcd.go:176);单个 goroutine 是输出通道的唯一写者与关闭者
   (registry-nacos 惯例——无 send-after-close 竞态)。
2. 该 goroutine 先做**全量快照**(`Get` + `WithPrefix`,5s 超时)并把当前集合作为首推
   播下去,调用方无需先 Resolve(discovery_etcd.go:162-184)。
3. watch 通道上每个 etcd 事件都触发一次**全新全量快照**(不是增量),按需做 scheme
   过滤(discovery_etcd.go:186-187)。
4. 变更检测:`endpointsKey` 把快照渲染成 `addr,scheme,weight;...` 可比较字符串
   (discovery_etcd.go:264-275);字符串不变即 no-op——"no-op re-delivery must not churn
   consumers"(discovery_etcd.go:188-191)。所以 `UpdateWeight`(weight 在 key 里)会
   推送;快照相同的前缀事件不推。
5. 每个 KV 从注册器写的 JSON 解码;`Healthy` 恒为 true——**key 的存在就是健康信号**
   (lease 过期即删 key)(discovery_etcd.go:237-256 及文件头注释 29-32 行)。畸形
   payload 单条 Warn 跳过,不破坏整个快照(discovery_etcd.go:241-245)。
6. `Resolve` 是同一次 Get 的一次性版本;`Services` 从 key 布局枚举服务名
   (discovery_etcd.go:140-148,201-211)。

下游 loadbalance `Pool` 消费这些快照,并在每次 Pick 应用 **weight-0 过滤**:
`excludeDrained` 丢弃 `Weight == 0` 的端点;全部端点都被摘时回退到全集,避免未归一化
快照把池打黑洞(cloud/loadbalance/pool.go:97-99,122-135)。

### 2.4 DRAIN 链路 —— UpdateWeight(0)

`Server.UpdateWeight(ctx, 0)`(starter.go:152-157)→ 守卫:registrar 为 nil 或从未注册 →
报错 `registry-etcd: instance not registered yet` → `etcdRegistrar.UpdateWeight`
(registrar.go:187-213):

- 查 key 对应的 `hold`(leaseID + 上次 payload);无 hold → 解释性报错
  (registrar.go:189-194;单测 registrar_weight_test.go:34-39)。
- 用新权重序列化**完整 instanceValue** 并 `Put` **到既有 lease 上**
  (`clientv3.WithLease(h.leaseID)`)——"不 grant、不 revoke、不重建 keep-alive,实例
  在更新中途从不离开 discovery,watcher 只是看到同一 key 的新值"
  (registrar.go:183-186,206)。
- 与 Register 不同,`UpdateWeight` 对 **0 原样放行**(只有 Register 钳位)——0 进入
  存储的 JSON,进而进入消费端 pool 的 `excludeDrained`。

消费侧纯靠 WATCH 感知:etcd 事件 → 快照 → `endpointsKey` 变化(weight 在 key 里)→
推送 → pool 在所有策略下过滤 0 权端点。`UpdateWeight(ctx, 100)` 恢复。同 lease 热更新
在 `registrar_weight_test.go` 的 `TestUpdateWeightHotReloadLive` 对真实 etcd 端到端验证
(watch 分布 9:1 → 1:9 翻转,全程无 Register)。

### 2.5 TTL / keep-alive 机制 —— 及已知脆弱点

- 每实例一个 lease,TTL 整秒(默认 15s;example 10s)。keep-alive goroutine
  (registrar.go:166-170)唯一职责是排干 clientv3 的续约通道——"the returned channel
  must be drained or the lease will not be renewed"(registrar.go:157-158)。
  续约节奏是 clientv3 的(≈ TTL/3),本 starter 不提供配置。
- **已知行为(下文嫌疑 #2)**:若 lease 在服务端死亡(etcd 重启/压缩、lease 被带外
  revoke),key 在 TTL 后消失,keep-alive 通道直接结束——**不重注册、无日志;进程继续
  运行而实例从 discovery 静默蒸发**。恢复手段是重启进程。请监控实例数,或对 key 做
  `etcdctl watch`。
- Deregister = 取消 keep-alive + revoke lease(key 立即删除,不等 TTL)
  (registrar.go:217-233)。停机反注册失败仅 Warn,key 等 TTL 过期(starter.go:159-166)。

---

## 3. 逐 key 行为参考

### 3.1 `${spring.registry.etcd.*}` —— 集群连接(config.go:26-55)

| key | 类型 | 默认值 | 行为与联动 | 配错后果 |
|-----|------|--------|-----------|----------|
| `endpoints` | []string | — | **激活 key**(OnProperty);启动时对 `endpoints[0]` 做 `Status` 探活。 | 未设:starter 静默不装配;不可达:启动失败 `registry-etcd: startup probe failed`(registrar.go:95-100) |
| `username` / `password` | string | "" | etcd 认证凭据。 | 开 auth 的集群未配 → 启动探活失败 |
| `dial-timeout` | duration | 5s | 约束 client dial 与启动探活超时。 | 过小 → 慢网络下启动偶发失败 |
| `ttl` | duration | 15s | lease TTL;向上取整到整秒、最小 1s。⚠ `<=0` 静默变 15s,不是绑定期报错。 | 过长 → 崩溃驱逐延迟到 ~TTL;0 不会禁用任何东西 |
| `key-prefix` | string | `/services/` | 所有 key 的前缀。⚠ 必须与每个消费块的 `key-prefix` 相同——耦合只有文档约束、从不校验。 | 不一致 → provider 正常注册、consumer 什么都发现不了,双方都"成功" |
| `tls.*` | tlsconf | 关 | 共享 `cloud/tlsconf` 块:`enabled`、`cert-file`、`key-file`、`ca-file`、`server-name`、`insecure-skip-verify`。 | CA 错 → 启动探活失败 |

### 3.2 `${spring.registry.*}` —— 广告的实例(config.go:57-81)

| key | 类型 | 默认值 | 行为与联动 | 配错后果 |
|-----|------|--------|-----------|----------|
| `service-name` | string | "" | 逻辑名;构成 key 段,也是消费方解析/watch 的名字。必填。 | 空 → Run 报 `registry: ${spring.registry.service-name} and ${spring.registry.addr} are required`(starter.go:103-105) |
| `addr` | string | "" | 广告的 `host:port`。从不猜测。 | 空 → 同上;Register 也有 RequireField 守卫(registrar.go:128-130) |
| `id` | string | "" | 实例 id 覆盖;空则派生 `<service-name>-<addr>`,重启覆盖同一 key。⚠ 跨进程重复 id 会互相覆盖 lease 持有。 | 两进程同名+同地址 → 只剩一条 key,lease 互踩 |
| `weight` | int | 0 | 存储权重;**Register 时** `<=0` 归一为 1(registrar.go:134-136)。⚠ 配置 0 不摘流——摘流只有 `UpdateWeight(0)`。 | 以为配置 0 能摘流 → 实例仍是权重 1 |
| `metadata` | map[string]string | 空 | 任意属性(zone、version);`scheme` 是保留 key,驱动消费侧 scheme 过滤(discovery_etcd.go:248)。 | `scheme` 值错 → 实例被 scheme 限定查询过滤掉 |

### 3.3 `${spring.discovery.etcd.<name>.*}` —— 消费侧后端(discovery_etcd.go:58-79)

| key | 类型 | 默认值 | 行为与联动 | 配错后果 |
|-----|------|--------|-----------|----------|
| `endpoints` | []string | — | 必填且绑定期 `expr:"len($) > 0"`;每块注册一个具名后端进 `cloud/discovery`。 | `<name>` 重复 → 启动报 `discovery backend %q already registered` |
| `username` / `password` | string | "" | 同注册侧。 | 探活期认证失败 |
| `dial-timeout` | duration | 5s | 约束 dial 与探活。 | 同上 |
| `key-prefix` | string | `/services/` | 必须与注册侧 prefix 一致(§3.1 ⚠)。 | 什么都发现不了,且无报错 |
| `tls.*` | tlsconf | 关 | 同一共享块。 | 探活失败 |

这里没有 TTL:消费侧从不写(discovery_etcd.go:56-57)。

---

## 4. 验证与故障演练

所有演练用 `etcdctl`(或 v3 curl API)对着 §1 环境。key:
`/services/orders/orders-127.0.0.1:8080`。

1. **注册 → 解析**:`go run .`(example 模式)打出 `registered "orders" at 127.0.0.1:8080`
   与 `discovered endpoint=127.0.0.1:8080 weight=100 metadata=map[...]`——单进程闭环
   (example/example.go:86-111)。原始视角:

   ```bash
   etcdctl get /services/orders/ --prefix
   # value JSON: {"service_name":"orders","addr":"...","weight":100,...}
   ```

2. **Weight=0 摘流与恢复**:example 以 `-manual` 运行,调用 `UpdateWeight(ctx, 0)`
   (经 §1 的 Drainer)。同 key、新 JSON、同 lease:

   ```bash
   etcdctl get /services/orders/orders-127.0.0.1:8080    # "weight":0
   # 消费 pool 的下一个 watch 快照丢弃该端点(excludeDrained)
   # 恢复: UpdateWeight(ctx, 100) → 权重回升,端点重新进入轮转
   ```

   由 `TestUpdateWeightHotReloadLive` 活体断言(registrar_weight_test.go:53-133)。

3. **手工改权重 vs 进程内状态**:`etcdctl put /services/orders/orders-127.0.0.1:8080
   '{"service_name":"orders","addr":"127.0.0.1:8080","weight":7}'`(未带 lease——key 变
   永久!)。watcher 看到 7,但进程内后续 `UpdateWeight` 会覆盖它(嫌疑 #6),且手工 key
   崩溃后永不过期。此演练用于理解,不用于运维。

4. **优雅停机反注册**:`kill <pid>` → PreStop revoke lease → key 立即消失:

   ```bash
   etcdctl get /services/orders/ --prefix     # 空
   ```

5. **崩溃 / TTL 过期(实例丢失)**:`kill -9 <pid>` → 无 revoke;最后一次续约后 ~TTL
   key 消失(默认 15s,example 10s)。现场观看:

   ```bash
   etcdctl watch /services/orders/ --prefix   # 过期时对该 key 触发一个 DELETE 事件
   ```

6. **lease 死亡静默蒸发(已知问题)**:应用运行期间重启 etcd 容器(或先
   `etcdctl get <key> -w json | jq .kvs[0].lease` 拿到 lease id 再
   `etcdctl lease revoke <id>`):key 在 TTL 后消失、进程继续运行、**无任何日志、不重
   注册**(registrar.go:166-170——排干循环直接结束)。消费方经 watch 驱逐端点;
   provider 对自己已掉线毫无感知。恢复:重启进程。即 §6 嫌疑 #2。

7. **WATCH 快照降级**:Watch 期间短暂断掉 etcd(如 iptables drop 2379):快照 Get 失败
   → Warn `registry-etcd: snapshot %q failed (waiting for watch)` 且返回 nil——"陈旧地址
   好过没有地址";事件恢复后 watch 通道照常投递(discovery_etcd.go:166-171)。

8. **坏集群快速失败**:`endpoints=127.0.0.1:9999` 启动 → 直接失败
   `registry-etcd: startup probe failed for 127.0.0.1:9999`(registrar.go:95-100)——
   不会留下静默运行期缺口。

运行期日志带 tag `_app_registry_etcd`:`creating etcd registrar`、`registering
service=...`、`registered %q at %s`、`deregister %q`(失败 Warn)、`registered etcd
discovery backend name=...`。经 `logger.<name>.tag=_app_registry_etcd` 单独调级。
本 starter 不产出 metrics/trace。

---

## 5. 排障表

| 症状 | 原因 | 处置 |
|------|------|------|
| 启动报 `registry: ${spring.registry.service-name} and ${spring.registry.addr} are required` | 任一 key 未设 | 两个都设上(starter.go:103-105) |
| 启动报 `startup probe failed` | etcd 不可达 / TLS 错 / 缺认证 | 修 `endpoints`/`tls.*`/凭据;探活就是启动期证明 |
| provider 正常,consumer 解析不到 | 注册块与 discovery 块 `key-prefix` 不一致 | 两边对齐(默认 `/services/`);该不一致从不校验 |
| 进程健康但实例从 discovery 消失 | lease 死亡(etcd 重启、带外 revoke)——静默、不重注册 | 重启进程;监控实例数(演练 6) |
| `discovery backend "x" already registered` | 出现两个 `${spring.discovery.etcd.x.*}` 块 | 改名其一 |
| consumer 仍路由到已摘流实例 | pool 只看到全 0 权端点而回退全集(pool.go:97-99),或快照陈旧 | 检查是否所有端点都被摘;确认 watch 已推 0 权快照 |
| 重启后残留重复 key | 此前手工 `etcdctl put` 写了无 lease 的永久 key | 删除手工 key;永远不要手写实例 key |
| UpdateWeight 报 `registry-etcd: instance not registered yet` | 在 Run 注册前调用(或 Run 失败) | 在 `registered` 日志出现后再调用 |
| 两进程只剩一条 etcd key | 同名+同地址 → 同派生 id | 每实例设独立 `spring.registry.id` |
| 消费日志出现 `skip malformed instance` Warn | 服务前缀下有非 JSON value | 清掉坏 key(手工写入、别的工具写入) |

---

## 6. 设计体检表

| 指标 | 数值 |
|------|------|
| 配置 key | 18(连接 7 + 实例 5 + discovery 6)+ 每个 tls 块共享 6 个 `tls.*` |
| 必填 | 绑定期 2(两侧 `endpoints`)+ Run 期 2(`service-name`、`addr`) |
| quickstart 前置外部依赖 | 1(etcd;example docker 门控) |
| "注意/坑"条数 | 6 |

设计嫌疑清单(承接上一轮审计;均未结):

1. 注册器与 discovery 块之间的 `key-prefix` 耦合只有文档约束、从不校验——即使两块在
   同一份 app.properties 里。
2. lease keep-alive 死亡 = TTL 后静默消失(不重注册、无日志)——最大的运维黑洞。
3. watch 通道关闭是静默的(流终止无日志)。
4. `ttl<=0` 静默变 15s,而不是绑定期报错。
5. README 的 weight 行暗示配置 `weight:=0` 可摘流——实际永远存 1(被钳位);摘流只有
   `UpdateWeight` 一条路。
6. 在注册中心手改权重对 watcher 可见,但进程内后续 `UpdateWeight` 会覆盖手改(无调和)。
