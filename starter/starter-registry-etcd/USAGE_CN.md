# starter-registry-etcd 使用说明 — 参考手册

详细使用文档,概览见 [README.md](README.md)。所有行为声明均对照 starter 源码(`starter.go`、
`config.go`、`registrar.go`、`discovery.go`、`registrar_weight_test.go`)与可运行的
[example/](example/)(`example/check.sh` = 单测 + docker 门控 etcd 端到端)核对。
**etcd 自身语义(lease、watch、KV、auth)见 [etcd 官方文档](https://etcd.io/docs/)**——
本文只写 go-spring 的增量。

**模型**:配置是**命名块**——每个 `spring.registry.etcd.<name>.*` 块描述一个 etcd 集群,
成为名为 `etcd.<name>` 的后端 bean(`center.go`)。该 bean 同时实现命名体系的两半:
`discovery.Registrar`(写侧——由传递依赖自动引入的 [starter-registry](../starter-registry)
核心之 `registryServer` 收集,跨后端注册进**每一个**已配置中心)与 `discovery.Discovery`
(读侧——消费方按 bean 名引用,如 `discovery=etcd.main`;bean 惰性,纯 provider 从不为读侧
付费)。读写共享该块的客户端与键前缀,永不分裂。没有默认/无名块。注册仅在设置
`spring.registry.service-name` 后激活——纯消费方应用只配连接块、不注册任何实例。
多中心(双注册、跨后端混搭)就是多配几个块:两个块 + service-name,每个中心都拿到实例。
它不开端口——导出的 `gs.Server`(在 starter-registry 核心)只为把注册接进应用生命周期。

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
    StarterRegistry "go-spring.org/starter-registry"
)

func init() {
    // 注册器 bean 名为 "registryServer",导出为 gs.Server。
    // 需要运行期摘流/恢复时注入它;否则可省略本文件。
    gs.Provide(func(s *StarterRegistry.Server) *Drainer {
        return &Drainer{srv: s}
    })
}

type Drainer struct{ srv *StarterRegistry.Server }

// Drain 不停进程就把实例摘出负载均衡:在每个中心的同一个 lease 上改写存储权重
// (不重注册、key 不消失)。
func (d *Drainer) Drain(ctx context.Context) error { return d.srv.UpdateWeight(ctx, 0) }
func (d *Drainer) Restore(ctx context.Context) error { return d.srv.UpdateWeight(ctx, 100) }
```

**conf/app.properties**(逐字取自 `example/conf/app.properties`):

```properties
spring.app.name=registry-etcd-example

# 每个命名块一个 etcd 集群;每块成为后端 bean "etcd.<name>",
# 同时服务注册(经 starter-registry 核心)与发现。
spring.registry.etcd.main.endpoints=127.0.0.1:2379
spring.registry.etcd.main.ttl=10s
spring.registry.etcd.main.key-prefix=/services/

# 广告的实例(后端无关,所有中心共享;换注册后端是换 blank-import,不是改配置)。
spring.registry.service-name=orders
spring.registry.addr=127.0.0.1:8080
spring.registry.weight=100
spring.registry.metadata.zone=cn-north
spring.registry.metadata.version=v1

# 消费侧:零配置。client starter 按块的 bean 名引用:
# spring.redis.demo.service-name=orders
# spring.redis.demo.discovery=etcd.main
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
import starter-registry-etcd（传递引入 starter-registry）
  ├─ 每个 ${spring.registry.etcd.<name>} 块: gs.Provide(newEtcdBackend).Name("etcd.<name>")
  │      Module 条件: OnProperty("spring.registry.etcd")            [center.go]
  │      Export(As[discovery.Discovery], As[discovery.Registrar]) + Destroy(Close)
  │
  ├─ starter-registry 核心: gs.Provide(NewServer).Name("registryServer")
  │      条件: OnProperty("spring.registry.service-name")   [starter-registry/starter.go]
  │      Registrars []discovery.Registrar——容器以切片注入收集每个后端 bean 的
  │      registrar(etcd、zookeeper……混搭)
  │
gs.Run()
  ├─ 绑定: ${spring.registry.etcd} → 每块一份 EtcdConfig(BindEach);
  │        ${spring.registry} → Server.Config 字段
  ├─ newEtcdBackend(每块): clientv3.New + 对 endpoints[0] 做 Status 探活——
  │      不可达/认证错的集群在此直接启动失败,而非等到首次 Register;每块一次
  │                                                        [center.go]
  ├─ 每个后端的 discovery 半边(惰性)首次使用时经该块客户端解析
  ├─ Runner 启动 → 就绪信号触发
  ├─ registryServer.Run: 校验 service-name/addr + 至少一个 registrar
  │      等待 <-sig.TriggerAndWait()(就绪门:所有 server 起来后才注册)
  │      向每个中心 Register → 日志 `registered "orders" at ... in N registry center(s)`
  ├─ 稳态: 每中心一条 keep-alive goroutine 持续续约(clientv3 默认 ≈ TTL/3)
  └─ SIGTERM: PreStop 最先反注册(先于 pre-stop 延迟、先于任何 server 停止)
         → 在途请求继续排空时 discovery 已不再分发本实例
         Stop 是幂等兜底                              [starter-registry/starter.go]
```

设计理由(源码注释):注册绑定应用就绪——"实例在应用就绪后才发布……这一顺序让滚动重启
零损"(starter-registry/starter.go);崩溃安全从不依赖 Deregister——lease TTL 自动清 key,
"self-healing without a reaper"(starter.go)。

### 2.2 注册写入语义

- key = `<key-prefix><service-name>/<instance-id>`,id 取配置 `id`,否则派生
  `<service-name>-<addr>` —— 重启覆盖同一 key(registrar.go)。
- value = JSON `instanceValue{service_name, addr, weight, metadata}`
  (registrar.go)。
- Register 时负权重归一为 1;**0 原样透传即摘流信号**,两条写路径同语义,配置
  `weight=0` 即注册一个已摘流实例(registrar.go)。
- `Register` = Grant(整秒 TTL) → `Put(key, val, WithLease)` → `KeepAlive` goroutine
  必须排干续约通道,否则 lease 死亡(registrar.go)。同实例重注册会先注销旧
  lease(registrar.go)。
- `ttlSeconds()` 向上取整到整秒、最小 1s;`TTL<=0` 静默变 15s(config.go)。

### 2.3 DISCOVERY 链路(消费侧)

`etcdDiscovery.Resolve`(discovery.go):

1. 某服务的第一次 Resolve 做一次**全量快照**(`Get`+`WithPrefix`,由调用方 ctx 约束)
   并写入缓存,随后启动后台 watcher(`clientv3.Watch(bgCtx, prefix, WithPrefix())`)。
2. watch 通道上每个 etcd 事件都触发一次**全新全量快照**(不是增量)写入缓存;刷新失败
   保留旧快照——"陈旧地址好过没有地址"。
3. 之后的 Resolve 都是对缓存(未过滤全集)的内存读,每次调用经 `FilterByScheme` 按
   scheme 收窄。
4. 每个 KV 从 registrar 的 JSON 解码;`Healthy` 恒为 true——**key 存在即健康信号**
   (lease 过期删 key)(文件头注释 29-32 行)。畸形 payload 跳过并 Warn,不打断快照。
5. `Services` 按 key 布局枚举服务名。

下游,loadbalance `Pool` 消费这些快照并在每次 Pick 做 **weight-0 过滤**:
`excludeDrained` 丢弃 `Weight == 0` 端点,全部摘流时回退全集,防未归一化
快照把池打黑洞(cloud/loadbalance/pool.go:97-99,122-135)。

### 2.4 DRAIN 链路 —— UpdateWeight(0)

`Server.UpdateWeight(ctx, 0)`(starter-registry `starter.go`)→ 守卫:从未注册 →
报错 `registry: instance not registered yet` → 逐中心的 `etcdRegistrar.UpdateWeight`
(registrar.go):

- 查 key 对应的 `hold`(leaseID + 上次 payload);无 hold → 解释性报错
  (registrar.go;单测见 registrar_weight_test.go)。
- 用新权重序列化**完整 instanceValue** 并 `Put` **到既有 lease 上**
  (`clientv3.WithLease(h.leaseID)`)——"不 grant、不 revoke、不重建 keep-alive,实例
  在更新中途从不离开 discovery,watcher 只是看到同一 key 的新值"
  (registrar.go)。
- 与 Register 不同,`UpdateWeight` 对 **0 原样放行**(只有 Register 钳位)——0 进入
  存储的 JSON,进而进入消费端 pool 的 `excludeDrained`。

消费侧纯靠后台刷新感知:etcd 事件 → 快照刷新(weight 在 payload 里)→
下一次 Resolve 可见 → pool 在所有策略下过滤 0 权端点。`UpdateWeight(ctx, 100)` 恢复。同 lease 热更新
在 `registrar_weight_test.go` 的 `TestUpdateWeightHotReloadLive` 对真实 etcd 端到端验证
(watch 分布 9:1 → 1:9 翻转,全程无 Register)。

### 2.5 TTL / keep-alive 机制 —— 及已知脆弱点

- 每实例每中心一个 lease,TTL 整秒(默认 15s;example 10s)。keep-alive goroutine 唯一
  职责是排干 clientv3 的续约通道——"the returned channel must be drained or the lease
  will not be renewed"(registrar.go)。续约节奏是 clientv3 的(≈ TTL/3),本 starter
  不提供配置。
- **keep-alive 失联自愈**:若 lease 在服务端死亡(etcd 重启/压缩、lease 被带外 revoke),
  keep-alive 通道关闭,watcher goroutine 以指数退避(基数 1s 翻倍、封顶 1min)重跑
  publish 步骤(重新 grant lease + 重写上次 payload),直到实例重新注册——条目总能自行
  回来,无需运维干预(`watchKeepAlive`,registrar.go)。
- Deregister = 取消 keep-alive + revoke lease(key 立即删除,不等 TTL)
  (registrar.go)。停机反注册失败仅 Warn,key 等 TTL 过期(starter-registry
  `starter.go`)。

---

## 3. 逐 key 行为参考

### 3.1 `${spring.registry.etcd.<name>.*}` —— 每集群块(config.go)

每块一个 etcd 集群;块名自选,成为后端 bean `etcd.<name>`。任一块存在即激活模块
(`OnProperty("spring.registry.etcd")` 是前缀匹配);每块经 `BindEach` 绑定并在启动期探活。

| key | 类型 | 默认值 | 行为与联动 | 配错后果 |
|-----|------|--------|-----------|----------|
| `endpoints` | []string | — | 每块必填。启动时对 `endpoints[0]` 做 `Status` 探活。 | 未设:块绑定期报错(`endpoints is required`);不可达:启动失败 `registry-etcd: startup probe failed`(center.go) |
| `username` / `password` | string | "" | etcd 认证凭据。 | 开 auth 的集群未配 → 启动探活失败 |
| `dial-timeout` | duration | 5s | 约束 client dial 与启动探活超时。 | 过小 → 慢网络下启动偶发失败 |
| `ttl` | duration | 15s | lease TTL;向上取整到整秒、最小 1s。⚠ `<=0` 静默变 15s,不是绑定期报错。 | 过长 → 崩溃驱逐延迟到 ~TTL;0 不会禁用任何东西 |
| `key-prefix` | string | `/services/` | 所有 key 的前缀(读写共享)。⚠ 必须与消费方引用的块一致——耦合只有文档约束、从不校验。 | 不一致 → provider 正常注册、consumer 什么都发现不了,双方都"成功" |
| `tls.*` | tlsconf | 关 | 共享 `cloud/tlsconf` 块:`enabled`、`cert-file`、`key-file`、`ca-file`、`server-name`、`insecure-skip-verify`。 | CA 错 → 启动探活失败 |

两个块 = 两个中心 = 双注册(registryServer 会注册进两者)。跨后端混搭(etcd 块 +
zookeeper 块同进程)同理——registrar 收集与后端无关。

### 3.2 `${spring.registry.*}` —— 广告的实例(starter-registry `config.go`)

| key | 类型 | 默认值 | 行为与联动 | 配错后果 |
|-----|------|--------|-----------|----------|
| `service-name` | string | "" | 逻辑名;构成 key 段,也是消费方解析/watch 的名字。**它的存在即注册意图信号**——设置即向每个中心注册,不设 = 纯消费方。 | 空 → 纯消费方;设了却没有任何块 → Run 报 `registry: ${spring.registry.service-name} is set but no registry center is configured` |
| `addr` | string | "" | 广告的 `host:port`。从不猜测。 | 设了 service-name 却空 addr → 同样 Run 报错;Register 也有 RequireField 守卫(registrar.go) |
| `id` | string | "" | 实例 id 覆盖;空则派生 `<service-name>-<addr>`,重启覆盖同一 key。⚠ 跨进程重复 id 会互相覆盖 lease 持有。 | 两进程同名+同地址 → 只剩一条 key,lease 互踩 |
| `weight` | int | 100 | 存储权重;负权重写入时归一为 1(registrar.go)。0 = 摘流,启动期即生效。 | 负权重静默变 1,不报错 |
| `metadata` | map[string]string | 空 | 任意属性(zone、version);`scheme` 是保留 key,驱动消费侧 scheme 过滤(discovery.go)。 | `scheme` 值错 → 实例被 scheme 限定查询过滤掉 |

### 3.3 发现 —— 按块 bean 名引用(零配置)

不再存在 discovery 配置路径。每个块的后端 bean 本身就是一个
`cloud/discovery.Discovery`,名为 `etcd.<name>`,共享该块客户端(`center.go`)。
client starter 按 bean 名引用(`spring.http-client.backends.<n>.discovery=etcd.main`)。
该 bean 惰性——纯 provider 从不解析、从不为读侧付费。纯消费方只配连接块(不设
`service-name`/`addr`),不注册任何实例。多集群发现就是多个块:引用哪个块的集群就
从哪读——不必是你注册进去的块。

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

6. **lease 死亡自愈**:应用运行期间重启 etcd 容器(或先
   `etcdctl get <key> -w json | jq .kvs[0].lease` 拿到 lease id 再
   `etcdctl lease revoke <id>`):key 在 TTL 后消失,随后 registrar 的 keep-alive watcher
   带退避重跑 publish 步骤,key 回来(`watchKeepAlive`,registrar.go)——无需运维干预、
   无需重启进程。消费方经 watch 短暂驱逐,随后端点回归。

7. **刷新降级**:服务已缓存后短暂断掉 etcd(如 iptables drop 2379):刷新 Get 失败
   → Warn `registry-etcd: snapshot %q failed (waiting for watch)` 且返回 nil——"陈旧地址
   好过没有地址";事件恢复后缓存照常刷新。

8. **坏集群快速失败**:某块 `endpoints=127.0.0.1:9999` 启动 → 直接失败
   `registry-etcd: startup probe failed for 127.0.0.1:9999`(center.go)——
   不会留下静默运行期缺口。

运行期日志带 tag `_app_registry_etcd`:`creating etcd registrar`、`registering
service=...`、`registered %q at %s`、`deregister %q`(失败 Warn)、`registered etcd
discovery backend name=...`。经 `logger.<name>.tag=_app_registry_etcd` 单独调级。

可观测性：注册与发现经 OTel 全局产出指标——未引入 `starter-otel` 时全部 no-op。`register`、`deregister`、`update_weight` 各有一个 client span 与一条 `registry.operation.duration`（标签 `system`/`operation`/`service`/`status`）；`registry.registration.attempts_total` 按 `reason` 与 `status` 计数；`registry.instance.registered` gauge 在发布中为 1、否则为 0——自愈失败会落在这里，而不只是出现在日志里。发现半边把每次后台缓存同步上报到 `discovery.sync_total`，并维持 `discovery.cache.age_seconds`（距上次确认新鲜的秒数），watch 死掉时表现为持续爬升，而不是静默返回陈旧地址。 `reason` 取 `initial`（初次发布）与 `self_heal`（后台重注册）。

---

## 5. 排障表

| 症状 | 原因 | 处置 |
|------|------|------|
| 启动报 `registry: ${spring.registry.service-name} and ${spring.registry.addr} are required` | 设了 `service-name` 但没设 `addr` | 两个都设上(starter-registry `starter.go`) |
| 启动报 `... is set but no registry center is configured` | 设了 `service-name` 但没有任何 `spring.registry.etcd.<name>` 块 | 至少配一个带 `endpoints` 的块 |
| 启动报 `startup probe failed` | etcd 不可达 / TLS 错 / 缺认证 | 修该块的 `endpoints`/`tls.*`/凭据;探活就是启动期证明 |
| provider 正常,consumer 解析不到 | provider 注册的块与 consumer 引用的块 `key-prefix` 不一致 | 两边对齐(默认 `/services/`);该不一致从不校验 |
| consumer 引用 `etcd.main` 找不到 bean | 块名不同,或块缺失 | bean 名是 `etcd.<块名>`;核对块 key |
| 实例短暂消失后回归 | lease 死亡(etcd 重启、带外 revoke)后自愈循环完成重注册 | 无需处置——自愈(演练 6);频繁出现则检查 etcd 健康 |
| consumer 仍路由到已摘流实例 | pool 只看到全 0 权端点而回退全集(pool.go:97-99),或快照陈旧 | 检查是否所有端点都被摘;确认 watch 已推 0 权快照 |
| 重启后残留重复 key | 此前手工 `etcdctl put` 写了无 lease 的永久 key | 删除手工 key;永远不要手写实例 key |
| UpdateWeight 报 `registry: instance not registered yet` | 在 Run 注册前调用(或 Run 失败) | 在 `registered` 日志出现后再调用 |
| 两进程只剩一条 etcd key | 同名+同地址 → 同派生 id | 每实例设独立 `spring.registry.id` |
| 消费日志出现 `skip malformed instance` Warn | 服务前缀下有非 JSON value | 清掉坏 key(手工写入、别的工具写入) |

---

## 6. 设计体检表

| 指标 | 数值 |
|------|------|
| 配置 key | 每块 7 个(连接,含共享 `tls.*`)+ 实例 5 个(`spring.registry.*`) |
| 必填 | 每块绑定期 1(`endpoints`)+ Run 期 2(`service-name`、`addr`,仅注册需要) |
| quickstart 前置外部依赖 | 1(etcd;example docker 门控) |
| "注意/坑"条数 | 6 |

设计嫌疑清单(已按命名块模型更新):

1. provider 注册的块与 consumer 引用的块之间 `key-prefix` 耦合只有文档约束、从不校验
   (单块内两半共享一个 prefix;跨块不一致仍可能)。
2. ~~lease keep-alive 死亡 = TTL 后静默消失~~ 已解决:keep-alive watcher 以指数退避
   重注册(`watchKeepAlive`,registrar.go)。
3. 后台刷新失败有日志,但 etcd 长期不可达时会一直服务陈旧快照。
4. `ttl<=0` 静默变 15s,而不是绑定期报错。
5. README 的 weight 行暗示配置 `weight:=0` 可摘流——实际永远存 1(被钳位);摘流只有
   `UpdateWeight` 一条路。
6. 在注册中心手改权重对 watcher 可见,但进程内后续 `UpdateWeight` 会覆盖手改(无调和)。
